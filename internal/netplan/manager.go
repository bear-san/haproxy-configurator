package netplan

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bear-san/haproxy-configurator/internal/config"
	"github.com/bear-san/haproxy-configurator/internal/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// TransactionChange represents a change to be applied in a transaction
type TransactionChange struct {
	Operation  string `json:"operation"`   // "add" or "remove"
	IPAddress  string `json:"ip_address"`
	Interface  string `json:"interface"`
	Port       int    `json:"port,omitempty"`
	SubnetMask string `json:"subnet_mask,omitempty"`
}

// Transaction represents a Netplan transaction
type Transaction struct {
	TransactionID string              `json:"transaction_id"`
	CreatedAt     time.Time           `json:"created_at"`
	Status        string              `json:"status"` // "pending", "committed", "failed"
	Changes       []TransactionChange `json:"changes"`
}

// Manager handles Netplan configuration operations
type Manager struct {
	config        *config.NetplanConfig
	addresses     map[string]string // IP -> Interface mapping for tracking
	transactionDir string           // Directory for transaction files
	mutex         sync.RWMutex      // Protects addresses map
}

// NetplanConfiguration represents the structure of a Netplan YAML file
type NetplanConfiguration struct {
	Network NetplanNetwork `yaml:"network"`
}

// NetplanNetwork represents the network section of Netplan
type NetplanNetwork struct {
	Version   int                         `yaml:"version"`
	Ethernets map[string]NetplanInterface `yaml:"ethernets,omitempty"`
}

// NetplanInterface represents a network interface configuration
type NetplanInterface struct {
	Addresses []string `yaml:"addresses,omitempty"`
}

// NewManager creates a new Netplan manager
func NewManager(cfg *config.NetplanConfig) *Manager {
	// Use configured transaction directory or default
	transactionDir := cfg.Netplan.TransactionDir
	if transactionDir == "" {
		transactionDir = "/tmp/haproxy-netplan-transactions"
	}
	
	logger.GetLogger().Info("Initializing Netplan manager",
		zap.String("transaction_dir", transactionDir),
		zap.String("netplan_config_path", cfg.Netplan.ConfigPath),
		zap.Bool("backup_enabled", cfg.Netplan.BackupEnabled))
	
	// Ensure transaction directory exists
	_ = os.MkdirAll(transactionDir, 0755)
	_ = os.MkdirAll(filepath.Join(transactionDir, "committed"), 0755)

	return &Manager{
		config:         cfg,
		addresses:      make(map[string]string),
		transactionDir: transactionDir,
	}
}

// AddIPAddress adds an IP address to the appropriate interface
func (m *Manager) AddIPAddress(ipAddr string, _ int) error {
	if ipAddr == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	logger.GetLogger().Debug("Adding IP address to interface",
		zap.String("ip_address", ipAddr))

	// Find the appropriate interface for this IP
	interfaceName, err := m.config.FindInterfaceForIP(ipAddr)
	if err != nil {
		return fmt.Errorf("failed to find interface for IP %s: %w", ipAddr, err)
	}

	// Load current Netplan configuration
	netplanConfig, err := m.loadNetplanConfig()
	if err != nil {
		return fmt.Errorf("failed to load Netplan config: %w", err)
	}

	// Add the IP address to the interface
	if netplanConfig.Network.Ethernets == nil {
		netplanConfig.Network.Ethernets = make(map[string]NetplanInterface)
	}

	iface := netplanConfig.Network.Ethernets[interfaceName]

	// Get the appropriate subnet mask for this IP
	subnetMask, err := m.getSubnetMaskForIP(ipAddr)
	if err != nil {
		// Log warning but continue with /32 default
		logger.GetLogger().Warn("Failed to determine subnet mask, defaulting to /32",
			zap.String("ip_address", ipAddr),
			zap.Error(err))
		subnetMask = "/32"
	}

	fullAddr := fmt.Sprintf("%s%s", ipAddr, subnetMask)

	// Check if IP already exists
	for _, addr := range iface.Addresses {
		if strings.HasPrefix(addr, ipAddr) {
			// IP already exists, no need to add
			m.addresses[ipAddr] = interfaceName
			return nil
		}
	}

	// Add the new IP address with proper subnet mask
	iface.Addresses = append(iface.Addresses, fullAddr)
	netplanConfig.Network.Ethernets[interfaceName] = iface

	// Save the configuration
	if err := m.saveNetplanConfig(netplanConfig); err != nil {
		return fmt.Errorf("failed to save Netplan config: %w", err)
	}

	// Track the IP address
	m.addresses[ipAddr] = interfaceName

	return nil
}

// RemoveIPAddress removes an IP address from the interface
func (m *Manager) RemoveIPAddress(ipAddr string) error {
	// Find which interface this IP was assigned to
	interfaceName, exists := m.addresses[ipAddr]
	if !exists {
		// Try to find it in the current config
		var err error
		interfaceName, err = m.config.FindInterfaceForIP(ipAddr)
		if err != nil {
			return fmt.Errorf("IP address %s not found in tracking or config: %w", ipAddr, err)
		}
	}

	// Load current Netplan configuration
	netplanConfig, err := m.loadNetplanConfig()
	if err != nil {
		return fmt.Errorf("failed to load Netplan config: %w", err)
	}

	// Remove the IP address from the interface
	if netplanConfig.Network.Ethernets == nil {
		return fmt.Errorf("no ethernet interfaces configured")
	}

	iface, exists := netplanConfig.Network.Ethernets[interfaceName]
	if !exists {
		return fmt.Errorf("interface %s not found in Netplan config", interfaceName)
	}

	// Filter out the IP address
	var newAddresses []string
	for _, addr := range iface.Addresses {
		if !strings.HasPrefix(addr, ipAddr) {
			newAddresses = append(newAddresses, addr)
		}
	}

	iface.Addresses = newAddresses
	netplanConfig.Network.Ethernets[interfaceName] = iface

	// If no addresses left, remove the interface from config
	if len(iface.Addresses) == 0 {
		delete(netplanConfig.Network.Ethernets, interfaceName)
	}

	// Save the configuration
	if err := m.saveNetplanConfig(netplanConfig); err != nil {
		return fmt.Errorf("failed to save Netplan config: %w", err)
	}

	// Remove from tracking
	delete(m.addresses, ipAddr)

	return nil
}

// ApplyNetplan generates and applies the Netplan configuration
func (m *Manager) ApplyNetplan() error {
	// Generate the configuration first
	if err := m.generateNetplan(); err != nil {
		return fmt.Errorf("failed to generate Netplan config: %w", err)
	}

	// Then apply it
	cmd := exec.Command("netplan", "apply")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply Netplan configuration: %w, output: %s", err, string(output))
	}
	return nil
}

// generateNetplan generates the Netplan configuration without applying it
func (m *Manager) generateNetplan() error {
	cmd := exec.Command("netplan", "generate")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to generate Netplan configuration: %w, output: %s", err, string(output))
	}
	return nil
}

// loadNetplanConfig loads the current Netplan configuration
func (m *Manager) loadNetplanConfig() (*NetplanConfiguration, error) {
	configPath := m.config.Netplan.ConfigPath

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create a new empty configuration
		return &NetplanConfiguration{
			Network: NetplanNetwork{
				Version:   2,
				Ethernets: make(map[string]NetplanInterface),
			},
		}, nil
	}

	// Read existing configuration
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Netplan config file: %w", err)
	}

	var netplanConfig NetplanConfiguration
	if err := yaml.Unmarshal(data, &netplanConfig); err != nil {
		return nil, fmt.Errorf("failed to parse Netplan config: %w", err)
	}

	// Ensure ethernets map is initialized
	if netplanConfig.Network.Ethernets == nil {
		netplanConfig.Network.Ethernets = make(map[string]NetplanInterface)
	}

	return &netplanConfig, nil
}

// saveNetplanConfig saves the Netplan configuration to file
func (m *Manager) saveNetplanConfig(netplanConfig *NetplanConfiguration) error {
	configPath := m.config.Netplan.ConfigPath

	// Create backup if enabled
	if m.config.Netplan.BackupEnabled {
		if err := m.createBackup(configPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(netplanConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal Netplan config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write Netplan config file: %w", err)
	}

	return nil
}

// createBackup creates a backup of the existing Netplan configuration
func (m *Manager) createBackup(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No existing file to backup
		return nil
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.backup-%s", configPath, timestamp)

	src, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy backup file: %w", err)
	}

	return nil
}

// GetTrackedAddresses returns the currently tracked IP addresses
func (m *Manager) GetTrackedAddresses() map[string]string {
	result := make(map[string]string)
	for ip, iface := range m.addresses {
		result[ip] = iface
	}
	return result
}

// getSubnetMaskForIP finds the appropriate subnet mask for the given IP address
// based on the configured subnet mappings
func (m *Manager) getSubnetMaskForIP(ipAddr string) (string, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	for _, mapping := range m.config.Netplan.InterfaceMappings {
		for _, subnet := range mapping.Subnets {
			_, cidr, err := net.ParseCIDR(subnet)
			if err != nil {
				continue // Skip invalid CIDR
			}
			if cidr.Contains(ip) {
				// Extract the subnet mask from the CIDR notation
				ones, _ := cidr.Mask.Size()
				return fmt.Sprintf("/%d", ones), nil
			}
		}
	}

	return "/32", fmt.Errorf("no subnet found for IP %s, defaulting to /32", ipAddr)
}

// Transaction management methods

// AddIPAddressToTransaction adds an IP address change to a transaction
func (m *Manager) AddIPAddressToTransaction(transactionID, ipAddr string, port int) error {
	if ipAddr == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	logger.GetLogger().Debug("Adding IP address to transaction",
		zap.String("transaction_id", transactionID),
		zap.String("ip_address", ipAddr),
		zap.Int("port", port))

	// Find the appropriate interface for this IP
	interfaceName, err := m.config.FindInterfaceForIP(ipAddr)
	if err != nil {
		return fmt.Errorf("failed to find interface for IP %s: %w", ipAddr, err)
	}

	// Get the appropriate subnet mask for this IP
	subnetMask, err := m.getSubnetMaskForIP(ipAddr)
	if err != nil {
		logger.GetLogger().Warn("Failed to determine subnet mask, defaulting to /32",
			zap.String("ip_address", ipAddr),
			zap.Error(err))
		subnetMask = "/32"
	}

	// Add to transaction
	return m.addChangeToTransaction(transactionID, TransactionChange{
		Operation:  "add",
		IPAddress:  ipAddr,
		Interface:  interfaceName,
		Port:       port,
		SubnetMask: subnetMask,
	})
}

// RemoveIPAddressFromTransaction adds an IP address removal to a transaction
func (m *Manager) RemoveIPAddressFromTransaction(transactionID, ipAddr string) error {
	if ipAddr == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	logger.GetLogger().Debug("Removing IP address from transaction",
		zap.String("transaction_id", transactionID),
		zap.String("ip_address", ipAddr))

	// Find the appropriate interface for this IP
	interfaceName, err := m.config.FindInterfaceForIP(ipAddr)
	if err != nil {
		return fmt.Errorf("failed to find interface for IP %s: %w", ipAddr, err)
	}

	// Add to transaction
	return m.addChangeToTransaction(transactionID, TransactionChange{
		Operation: "remove",
		IPAddress: ipAddr,
		Interface: interfaceName,
	})
}

// CommitTransaction applies all changes in a transaction
func (m *Manager) CommitTransaction(transactionID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	logger.GetLogger().Info("Committing Netplan transaction",
		zap.String("transaction_id", transactionID))

	// Load the transaction
	transaction, err := m.loadTransaction(transactionID)
	if err != nil {
		return fmt.Errorf("failed to load transaction %s: %w", transactionID, err)
	}

	if transaction.Status != "pending" {
		return fmt.Errorf("transaction %s is not in pending status: %s", transactionID, transaction.Status)
	}

	// Load current Netplan configuration
	netplanConfig, err := m.loadNetplanConfig()
	if err != nil {
		return fmt.Errorf("failed to load Netplan config: %w", err)
	}

	// Apply all changes in the transaction
	for _, change := range transaction.Changes {
		if err := m.applyChange(netplanConfig, change); err != nil {
			// Mark transaction as failed
			logger.GetLogger().Error("Failed to apply transaction change",
				zap.String("transaction_id", transactionID),
				zap.Any("change", change),
				zap.Error(err))
			m.markTransactionFailed(transactionID, err)
			return fmt.Errorf("failed to apply change %+v: %w", change, err)
		}
	}

	// Save the Netplan configuration
	if err := m.saveNetplanConfig(netplanConfig); err != nil {
		m.markTransactionFailed(transactionID, err)
		return fmt.Errorf("failed to save Netplan config: %w", err)
	}

	// Update tracking state
	for _, change := range transaction.Changes {
		switch change.Operation {
		case "add":
			m.addresses[change.IPAddress] = change.Interface
		case "remove":
			delete(m.addresses, change.IPAddress)
		}
	}

	// Mark transaction as committed
	transaction.Status = "committed"
	if err := m.saveTransaction(transaction); err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	// Move transaction to committed directory
	if err := m.moveTransactionToCommitted(transactionID); err != nil {
		return fmt.Errorf("failed to move transaction to committed: %w", err)
	}

	logger.GetLogger().Info("Successfully committed Netplan transaction",
		zap.String("transaction_id", transactionID),
		zap.Int("changes_applied", len(transaction.Changes)))

	return nil
}

// addChangeToTransaction adds a change to an existing transaction or creates a new one
func (m *Manager) addChangeToTransaction(transactionID string, change TransactionChange) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var transaction *Transaction
	var err error

	// Try to load existing transaction
	transaction, err = m.loadTransaction(transactionID)
	if err != nil {
		// Create new transaction if it doesn't exist
		transaction = &Transaction{
			TransactionID: transactionID,
			CreatedAt:     time.Now(),
			Status:        "pending",
			Changes:       []TransactionChange{},
		}
	}

	if transaction.Status != "pending" {
		return fmt.Errorf("cannot add change to transaction %s with status %s", transactionID, transaction.Status)
	}

	// Add the change
	transaction.Changes = append(transaction.Changes, change)

	// Save the transaction
	return m.saveTransaction(transaction)
}

// loadTransaction loads a transaction from file
func (m *Manager) loadTransaction(transactionID string) (*Transaction, error) {
	filePath := filepath.Join(m.transactionDir, fmt.Sprintf("transaction-%s.json", transactionID))
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var transaction Transaction
	if err := json.Unmarshal(data, &transaction); err != nil {
		return nil, fmt.Errorf("failed to parse transaction file: %w", err)
	}

	return &transaction, nil
}

// saveTransaction saves a transaction to file
func (m *Manager) saveTransaction(transaction *Transaction) error {
	filePath := filepath.Join(m.transactionDir, fmt.Sprintf("transaction-%s.json", transaction.TransactionID))
	
	data, err := json.MarshalIndent(transaction, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write transaction file: %w", err)
	}

	return nil
}

// applyChange applies a single change to the Netplan configuration
func (m *Manager) applyChange(netplanConfig *NetplanConfiguration, change TransactionChange) error {
	if netplanConfig.Network.Ethernets == nil {
		netplanConfig.Network.Ethernets = make(map[string]NetplanInterface)
	}

	iface := netplanConfig.Network.Ethernets[change.Interface]

	switch change.Operation {
	case "add":
		fullAddr := fmt.Sprintf("%s%s", change.IPAddress, change.SubnetMask)
		
		// Check if IP already exists
		for _, addr := range iface.Addresses {
			if strings.HasPrefix(addr, change.IPAddress) {
				// IP already exists, no need to add
				return nil
			}
		}
		
		// Add the new IP address
		iface.Addresses = append(iface.Addresses, fullAddr)
		netplanConfig.Network.Ethernets[change.Interface] = iface

	case "remove":
		// Filter out the IP address
		var newAddresses []string
		for _, addr := range iface.Addresses {
			if !strings.HasPrefix(addr, change.IPAddress) {
				newAddresses = append(newAddresses, addr)
			}
		}
		
		iface.Addresses = newAddresses
		netplanConfig.Network.Ethernets[change.Interface] = iface
		
		// If no addresses left, remove the interface from config
		if len(iface.Addresses) == 0 {
			delete(netplanConfig.Network.Ethernets, change.Interface)
		}

	default:
		return fmt.Errorf("unknown operation: %s", change.Operation)
	}

	return nil
}

// markTransactionFailed marks a transaction as failed
func (m *Manager) markTransactionFailed(transactionID string, err error) {
	transaction, loadErr := m.loadTransaction(transactionID)
	if loadErr != nil {
		return
	}
	
	transaction.Status = "failed"
	// Could add error details to transaction if needed
	_ = m.saveTransaction(transaction)
}

// moveTransactionToCommitted moves a transaction file to the committed directory
func (m *Manager) moveTransactionToCommitted(transactionID string) error {
	srcPath := filepath.Join(m.transactionDir, fmt.Sprintf("transaction-%s.json", transactionID))
	dstPath := filepath.Join(m.transactionDir, "committed", fmt.Sprintf("transaction-%s.json", transactionID))
	
	return os.Rename(srcPath, dstPath)
}
