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
	Operation  string `json:"operation"` // "add" or "remove"
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
	config         *config.Config
	addresses      map[string]string // IP -> Interface mapping for tracking
	transactionDir string            // Directory for transaction files
	mutex          sync.RWMutex      // Protects addresses map
}

// NetplanConfiguration represents the structure of a Netplan YAML file
type NetplanConfiguration struct {
	Network NetplanNetwork `yaml:"network"`
}

// NetplanNetwork represents the network section of Netplan
type NetplanNetwork struct {
	Version   int                         `yaml:"version"`
	Ethernets map[string]NetplanInterface `yaml:"ethernets,omitempty"`
	Vlans     map[string]NetplanVLAN      `yaml:"vlans,omitempty"`
}

// NetplanInterface represents a network interface configuration
type NetplanInterface struct {
	Addresses   []string               `yaml:"addresses,omitempty"`
	DHCP4       bool                   `yaml:"dhcp4,omitempty"`
	DHCP6       bool                   `yaml:"dhcp6,omitempty"`
	Gateway4    string                 `yaml:"gateway4,omitempty"`
	Gateway6    string                 `yaml:"gateway6,omitempty"`
	MTU         int                    `yaml:"mtu,omitempty"`
	MACAddress  string                 `yaml:"macaddress,omitempty"`
	Critical    bool                   `yaml:"critical,omitempty"`
	Optional    bool                   `yaml:"optional,omitempty"`
	Routes      []NetplanRoute         `yaml:"routes,omitempty"`
	Nameservers *NetplanNameservers    `yaml:"nameservers,omitempty"`
	Renderer    string                 `yaml:"renderer,omitempty"`
	Match       *NetplanMatch          `yaml:"match,omitempty"`
	SetName     string                 `yaml:"set-name,omitempty"`
	Additional  map[string]interface{} `yaml:",inline"` // Preserve unknown fields
}

// NetplanVLAN represents a VLAN interface configuration
type NetplanVLAN struct {
	ID          int                    `yaml:"id"`
	Link        string                 `yaml:"link"`
	Optional    bool                   `yaml:"optional,omitempty"`
	Addresses   []string               `yaml:"addresses,omitempty"`
	DHCP4       bool                   `yaml:"dhcp4,omitempty"`
	DHCP6       bool                   `yaml:"dhcp6,omitempty"`
	Gateway4    string                 `yaml:"gateway4,omitempty"`
	Gateway6    string                 `yaml:"gateway6,omitempty"`
	MTU         int                    `yaml:"mtu,omitempty"`
	Critical    bool                   `yaml:"critical,omitempty"`
	Routes      []NetplanRoute         `yaml:"routes,omitempty"`
	Nameservers *NetplanNameservers    `yaml:"nameservers,omitempty"`
	Renderer    string                 `yaml:"renderer,omitempty"`
	Additional  map[string]interface{} `yaml:",inline"` // Preserve unknown fields
}

// NetplanNameservers represents DNS configuration
type NetplanNameservers struct {
	Addresses []string `yaml:"addresses,omitempty"`
	Search    []string `yaml:"search,omitempty"`
}

// NetplanRoute represents a route configuration
type NetplanRoute struct {
	To     string `yaml:"to"`
	Via    string `yaml:"via,omitempty"`
	From   string `yaml:"from,omitempty"`
	Metric int    `yaml:"metric,omitempty"`
	OnLink bool   `yaml:"on-link,omitempty"`
	Type   string `yaml:"type,omitempty"`
	Scope  string `yaml:"scope,omitempty"`
	Table  int    `yaml:"table,omitempty"`
}

// NetplanMatch represents match conditions for interface selection
type NetplanMatch struct {
	Name       string `yaml:"name,omitempty"`
	MACAddress string `yaml:"macaddress,omitempty"`
	Driver     string `yaml:"driver,omitempty"`
}

// NewManagerWithConfig creates a new Netplan manager using unified config
func NewManagerWithConfig(cfg *config.Config) *Manager {
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

// parseInterfaceName parses an interface name that might be in VLAN format (vlan@nic)
func parseInterfaceName(interfaceName string) (vlanName, nicName string, isVLAN bool) {
	parts := strings.Split(interfaceName, "@")
	if len(parts) == 2 {
		// VLAN format: vlan@nic
		return parts[0], parts[1], true
	}
	// Regular interface
	return "", interfaceName, false
}

// AddIPAddress adds an IP address to the appropriate network interface based on subnet mappings.
// It determines the correct interface, applies the appropriate subnet mask, and updates the Netplan configuration.
// Returns an error if the IP address is invalid or no interface mapping is found.
func (m *Manager) AddIPAddress(ipAddr string, _ int) error {
	if ipAddr == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	logger.GetLogger().Debug("Adding IP address to interface",
		zap.String("ip_address", ipAddr))

	// Find the appropriate interface for this IP
	interfaceName, err := m.findInterfaceForIP(ipAddr)
	if err != nil {
		return fmt.Errorf("failed to find interface for IP %s: %w", ipAddr, err)
	}

	// Load current Netplan configuration
	netplanConfig, err := m.loadNetplanConfig()
	if err != nil {
		return fmt.Errorf("failed to load Netplan config: %w", err)
	}

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

	// Parse interface name to check if it's a VLAN
	vlanName, nicName, isVLAN := parseInterfaceName(interfaceName)

	if isVLAN {
		// Handle VLAN interface
		if netplanConfig.Network.Vlans == nil {
			netplanConfig.Network.Vlans = make(map[string]NetplanVLAN)
		}

		vlan := netplanConfig.Network.Vlans[vlanName]

		// Check if IP already exists
		for _, addr := range vlan.Addresses {
			if strings.HasPrefix(addr, ipAddr) {
				// IP already exists, no need to add
				m.addresses[ipAddr] = interfaceName
				return nil
			}
		}

		// Add the new IP address
		vlan.Addresses = append(vlan.Addresses, fullAddr)

		// Ensure link is set to the correct NIC
		if vlan.Link == "" {
			vlan.Link = nicName
		}

		netplanConfig.Network.Vlans[vlanName] = vlan
	} else {
		// Handle regular Ethernet interface
		if netplanConfig.Network.Ethernets == nil {
			netplanConfig.Network.Ethernets = make(map[string]NetplanInterface)
		}

		iface := netplanConfig.Network.Ethernets[interfaceName]

		// Check if IP already exists
		for _, addr := range iface.Addresses {
			if strings.HasPrefix(addr, ipAddr) {
				// IP already exists, no need to add
				m.addresses[ipAddr] = interfaceName
				return nil
			}
		}

		// Add the new IP address
		iface.Addresses = append(iface.Addresses, fullAddr)
		netplanConfig.Network.Ethernets[interfaceName] = iface
	}

	// Save the configuration
	if err := m.saveNetplanConfig(netplanConfig); err != nil {
		return fmt.Errorf("failed to save Netplan config: %w", err)
	}

	// Track the IP address
	m.addresses[ipAddr] = interfaceName

	return nil
}

// RemoveIPAddress removes an IP address from its assigned network interface.
// It first checks the tracking map, then falls back to finding the interface via subnet mappings.
// Returns an error if the IP address is not found or cannot be removed.
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

	// Parse interface name to check if it's a VLAN
	vlanName, _, isVLAN := parseInterfaceName(interfaceName)

	if isVLAN {
		// Handle VLAN interface
		if netplanConfig.Network.Vlans == nil {
			return fmt.Errorf("no VLAN interfaces configured")
		}

		vlan, exists := netplanConfig.Network.Vlans[vlanName]
		if !exists {
			return fmt.Errorf("VLAN %s not found in Netplan config", vlanName)
		}

		// Filter out the IP address
		var newAddresses []string
		for _, addr := range vlan.Addresses {
			if !strings.HasPrefix(addr, ipAddr) {
				newAddresses = append(newAddresses, addr)
			}
		}

		vlan.Addresses = newAddresses
		netplanConfig.Network.Vlans[vlanName] = vlan

		// If no addresses left, remove the VLAN from config
		if len(vlan.Addresses) == 0 {
			delete(netplanConfig.Network.Vlans, vlanName)
		}
	} else {
		// Handle regular Ethernet interface
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
	}

	// Save the configuration
	if err := m.saveNetplanConfig(netplanConfig); err != nil {
		return fmt.Errorf("failed to save Netplan config: %w", err)
	}

	// Remove from tracking
	delete(m.addresses, ipAddr)

	return nil
}

// ApplyNetplan applies the Netplan configuration to the system.
// It runs 'netplan apply' which generates and activates the configuration.
// Returns an error if the command fails.
func (m *Manager) ApplyNetplan() error {
	cmd := exec.Command("netplan", "apply")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to apply Netplan configuration: %w, output: %s", err, string(output))
	}
	return nil
}

// loadNetplanConfig loads the current Netplan configuration directly from the specified yaml file
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

// GetTrackedAddresses returns a copy of the currently tracked IP addresses.
// The returned map contains IP addresses as keys and their assigned interfaces as values.
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

// AddIPAddressToTransaction adds an IP address assignment to a pending transaction.
// The IP address will be added to the appropriate interface when the transaction is committed.
// Returns an error if the IP address is invalid or no interface mapping is found.
func (m *Manager) AddIPAddressToTransaction(transactionID, ipAddr string, port int) error {
	if ipAddr == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	logger.GetLogger().Debug("Adding IP address to transaction",
		zap.String("transaction_id", transactionID),
		zap.String("ip_address", ipAddr),
		zap.Int("port", port))

	// Find the appropriate interface for this IP
	interfaceName, err := m.findInterfaceForIP(ipAddr)
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

// RemoveIPAddressFromTransaction adds an IP address removal to a pending transaction.
// The IP address will be removed from its interface when the transaction is committed.
// Returns an error if the IP address is invalid or no interface mapping is found.
func (m *Manager) RemoveIPAddressFromTransaction(transactionID, ipAddr string) error {
	if ipAddr == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	logger.GetLogger().Debug("Removing IP address from transaction",
		zap.String("transaction_id", transactionID),
		zap.String("ip_address", ipAddr))

	// Find the appropriate interface for this IP
	interfaceName, err := m.findInterfaceForIP(ipAddr)
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

// CommitTransaction applies all pending changes in a transaction to the Netplan configuration.
// It loads the transaction, applies all changes to the actual netplan yaml file, updates tracking,
// and runs netplan apply to activate changes.
// Returns an error if the transaction cannot be loaded, applied, or if any changes fail.
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

	// Load current Netplan configuration from actual netplan yaml file
	netplanConfig, err := m.loadNetplanConfig()
	if err != nil {
		return fmt.Errorf("failed to load Netplan config: %w", err)
	}

	// Apply all changes in the transaction to the netplan configuration
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

	// Save the updated configuration to the actual netplan yaml file
	if err := m.saveNetplanConfig(netplanConfig); err != nil {
		m.markTransactionFailed(transactionID, err)
		return fmt.Errorf("failed to save Netplan config: %w", err)
	}

	// Apply the netplan configuration to the system
	if err := m.ApplyNetplan(); err != nil {
		m.markTransactionFailed(transactionID, err)
		return fmt.Errorf("failed to apply Netplan configuration: %w", err)
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

	logger.GetLogger().Info("Successfully committed Netplan transaction and applied to system",
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
	// Parse interface name to check if it's a VLAN
	vlanName, nicName, isVLAN := parseInterfaceName(change.Interface)

	if isVLAN {
		// Handle VLAN interface
		if netplanConfig.Network.Vlans == nil {
			netplanConfig.Network.Vlans = make(map[string]NetplanVLAN)
		}

		vlan := netplanConfig.Network.Vlans[vlanName]

		switch change.Operation {
		case "add":
			fullAddr := fmt.Sprintf("%s%s", change.IPAddress, change.SubnetMask)

			// Check if IP already exists
			for _, addr := range vlan.Addresses {
				if strings.HasPrefix(addr, change.IPAddress) {
					// IP already exists, no need to add
					return nil
				}
			}

			// Add the new IP address
			vlan.Addresses = append(vlan.Addresses, fullAddr)

			// Ensure link is set to the correct NIC
			if vlan.Link == "" {
				vlan.Link = nicName
			}

			netplanConfig.Network.Vlans[vlanName] = vlan

		case "remove":
			// Filter out the IP address
			var newAddresses []string
			for _, addr := range vlan.Addresses {
				if !strings.HasPrefix(addr, change.IPAddress) {
					newAddresses = append(newAddresses, addr)
				}
			}

			vlan.Addresses = newAddresses
			netplanConfig.Network.Vlans[vlanName] = vlan

			// If no addresses left, remove the VLAN from config
			if len(vlan.Addresses) == 0 {
				delete(netplanConfig.Network.Vlans, vlanName)
			}

		default:
			return fmt.Errorf("unknown operation: %s", change.Operation)
		}
	} else {
		// Handle regular Ethernet interface
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
	}

	return nil
}

// markTransactionFailed marks a transaction as failed
func (m *Manager) markTransactionFailed(transactionID string, _ error) {
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

// findInterfaceForIP finds the appropriate interface for the given IP address
func (m *Manager) findInterfaceForIP(ipAddr string) (string, error) {
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
				return mapping.Interface, nil
			}
		}
	}

	return "", fmt.Errorf("no interface mapping found for IP %s", ipAddr)
}

// UnmarshalYAML implements custom YAML unmarshaling to preserve unknown fields
func (n *NetplanInterface) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First unmarshal into a generic map
	var raw map[string]interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	// Initialize the Additional map
	n.Additional = make(map[string]interface{})

	// Process known fields
	if v, ok := raw["addresses"]; ok {
		if addrs, ok := v.([]interface{}); ok {
			n.Addresses = make([]string, 0, len(addrs))
			for _, addr := range addrs {
				if s, ok := addr.(string); ok {
					n.Addresses = append(n.Addresses, s)
				}
			}
		}
		delete(raw, "addresses")
	}

	if v, ok := raw["dhcp4"]; ok {
		if b, ok := v.(bool); ok {
			n.DHCP4 = b
		}
		delete(raw, "dhcp4")
	}

	if v, ok := raw["dhcp6"]; ok {
		if b, ok := v.(bool); ok {
			n.DHCP6 = b
		}
		delete(raw, "dhcp6")
	}

	if v, ok := raw["gateway4"]; ok {
		if s, ok := v.(string); ok {
			n.Gateway4 = s
		}
		delete(raw, "gateway4")
	}

	if v, ok := raw["gateway6"]; ok {
		if s, ok := v.(string); ok {
			n.Gateway6 = s
		}
		delete(raw, "gateway6")
	}

	if v, ok := raw["mtu"]; ok {
		switch val := v.(type) {
		case int:
			n.MTU = val
		case float64:
			n.MTU = int(val)
		}
		delete(raw, "mtu")
	}

	if v, ok := raw["macaddress"]; ok {
		if s, ok := v.(string); ok {
			n.MACAddress = s
		}
		delete(raw, "macaddress")
	}

	if v, ok := raw["critical"]; ok {
		if b, ok := v.(bool); ok {
			n.Critical = b
		}
		delete(raw, "critical")
	}

	if v, ok := raw["optional"]; ok {
		if b, ok := v.(bool); ok {
			n.Optional = b
		}
		delete(raw, "optional")
	}

	if v, ok := raw["routes"]; ok {
		if routes, ok := v.([]interface{}); ok {
			n.Routes = make([]NetplanRoute, 0, len(routes))
			for _, route := range routes {
				var r NetplanRoute
				if routeData, err := yaml.Marshal(route); err == nil {
					if err := yaml.Unmarshal(routeData, &r); err == nil {
						n.Routes = append(n.Routes, r)
					}
				}
			}
		}
		delete(raw, "routes")
	}

	if v, ok := raw["nameservers"]; ok {
		var ns NetplanNameservers
		if nsData, err := yaml.Marshal(v); err == nil {
			if err := yaml.Unmarshal(nsData, &ns); err == nil {
				n.Nameservers = &ns
			}
		}
		delete(raw, "nameservers")
	}

	if v, ok := raw["renderer"]; ok {
		if s, ok := v.(string); ok {
			n.Renderer = s
		}
		delete(raw, "renderer")
	}

	if v, ok := raw["match"]; ok {
		var m NetplanMatch
		if matchData, err := yaml.Marshal(v); err == nil {
			if err := yaml.Unmarshal(matchData, &m); err == nil {
				n.Match = &m
			}
		}
		delete(raw, "match")
	}

	if v, ok := raw["set-name"]; ok {
		if s, ok := v.(string); ok {
			n.SetName = s
		}
		delete(raw, "set-name")
	}

	// Store remaining fields in Additional
	for k, v := range raw {
		n.Additional[k] = v
	}

	return nil
}

// MarshalYAML implements custom YAML marshaling to include unknown fields
func (n NetplanInterface) MarshalYAML() (interface{}, error) {
	// Start with the additional fields
	result := make(map[string]interface{})
	for k, v := range n.Additional {
		result[k] = v
	}

	// Add known fields (only if they have non-zero values)
	if len(n.Addresses) > 0 {
		result["addresses"] = n.Addresses
	}
	if n.DHCP4 {
		result["dhcp4"] = n.DHCP4
	}
	if n.DHCP6 {
		result["dhcp6"] = n.DHCP6
	}
	if n.Gateway4 != "" {
		result["gateway4"] = n.Gateway4
	}
	if n.Gateway6 != "" {
		result["gateway6"] = n.Gateway6
	}
	if n.MTU != 0 {
		result["mtu"] = n.MTU
	}
	if n.MACAddress != "" {
		result["macaddress"] = n.MACAddress
	}
	if n.Critical {
		result["critical"] = n.Critical
	}
	if n.Optional {
		result["optional"] = n.Optional
	}
	if len(n.Routes) > 0 {
		result["routes"] = n.Routes
	}
	if n.Nameservers != nil {
		result["nameservers"] = n.Nameservers
	}
	if n.Renderer != "" {
		result["renderer"] = n.Renderer
	}
	if n.Match != nil {
		result["match"] = n.Match
	}
	if n.SetName != "" {
		result["set-name"] = n.SetName
	}

	return result, nil
}

// UnmarshalYAML implements custom YAML unmarshaling for NetplanVLAN to preserve unknown fields
func (n *NetplanVLAN) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First unmarshal into a generic map
	var raw map[string]interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	// Initialize the Additional map
	n.Additional = make(map[string]interface{})

	// Process known fields
	if v, ok := raw["id"]; ok {
		switch val := v.(type) {
		case int:
			n.ID = val
		case float64:
			n.ID = int(val)
		}
		delete(raw, "id")
	}

	if v, ok := raw["link"]; ok {
		if s, ok := v.(string); ok {
			n.Link = s
		}
		delete(raw, "link")
	}

	if v, ok := raw["optional"]; ok {
		if b, ok := v.(bool); ok {
			n.Optional = b
		}
		delete(raw, "optional")
	}

	if v, ok := raw["addresses"]; ok {
		if addrs, ok := v.([]interface{}); ok {
			n.Addresses = make([]string, 0, len(addrs))
			for _, addr := range addrs {
				if s, ok := addr.(string); ok {
					n.Addresses = append(n.Addresses, s)
				}
			}
		}
		delete(raw, "addresses")
	}

	if v, ok := raw["dhcp4"]; ok {
		if b, ok := v.(bool); ok {
			n.DHCP4 = b
		}
		delete(raw, "dhcp4")
	}

	if v, ok := raw["dhcp6"]; ok {
		if b, ok := v.(bool); ok {
			n.DHCP6 = b
		}
		delete(raw, "dhcp6")
	}

	if v, ok := raw["gateway4"]; ok {
		if s, ok := v.(string); ok {
			n.Gateway4 = s
		}
		delete(raw, "gateway4")
	}

	if v, ok := raw["gateway6"]; ok {
		if s, ok := v.(string); ok {
			n.Gateway6 = s
		}
		delete(raw, "gateway6")
	}

	if v, ok := raw["mtu"]; ok {
		switch val := v.(type) {
		case int:
			n.MTU = val
		case float64:
			n.MTU = int(val)
		}
		delete(raw, "mtu")
	}

	if v, ok := raw["critical"]; ok {
		if b, ok := v.(bool); ok {
			n.Critical = b
		}
		delete(raw, "critical")
	}

	if v, ok := raw["routes"]; ok {
		if routes, ok := v.([]interface{}); ok {
			n.Routes = make([]NetplanRoute, 0, len(routes))
			for _, route := range routes {
				var r NetplanRoute
				if routeData, err := yaml.Marshal(route); err == nil {
					if err := yaml.Unmarshal(routeData, &r); err == nil {
						n.Routes = append(n.Routes, r)
					}
				}
			}
		}
		delete(raw, "routes")
	}

	if v, ok := raw["nameservers"]; ok {
		var ns NetplanNameservers
		if nsData, err := yaml.Marshal(v); err == nil {
			if err := yaml.Unmarshal(nsData, &ns); err == nil {
				n.Nameservers = &ns
			}
		}
		delete(raw, "nameservers")
	}

	if v, ok := raw["renderer"]; ok {
		if s, ok := v.(string); ok {
			n.Renderer = s
		}
		delete(raw, "renderer")
	}

	// Store remaining fields in Additional
	for k, v := range raw {
		n.Additional[k] = v
	}

	return nil
}

// MarshalYAML implements custom YAML marshaling for NetplanVLAN to include unknown fields
func (n NetplanVLAN) MarshalYAML() (interface{}, error) {
	// Start with the additional fields
	result := make(map[string]interface{})
	for k, v := range n.Additional {
		result[k] = v
	}

	// Add known fields (always include id and link as they are required)
	result["id"] = n.ID
	result["link"] = n.Link

	// Add other fields only if they have non-zero values
	if n.Optional {
		result["optional"] = n.Optional
	}
	if len(n.Addresses) > 0 {
		result["addresses"] = n.Addresses
	}
	if n.DHCP4 {
		result["dhcp4"] = n.DHCP4
	}
	if n.DHCP6 {
		result["dhcp6"] = n.DHCP6
	}
	if n.Gateway4 != "" {
		result["gateway4"] = n.Gateway4
	}
	if n.Gateway6 != "" {
		result["gateway6"] = n.Gateway6
	}
	if n.MTU != 0 {
		result["mtu"] = n.MTU
	}
	if n.Critical {
		result["critical"] = n.Critical
	}
	if len(n.Routes) > 0 {
		result["routes"] = n.Routes
	}
	if n.Nameservers != nil {
		result["nameservers"] = n.Nameservers
	}
	if n.Renderer != "" {
		result["renderer"] = n.Renderer
	}

	return result, nil
}
