package netplan

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bear-san/haproxy-configurator/internal/config"
	"gopkg.in/yaml.v3"
)

// Manager handles Netplan configuration operations
type Manager struct {
	config    *config.NetplanConfig
	addresses map[string]string // IP -> Interface mapping for tracking
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
	return &Manager{
		config:    cfg,
		addresses: make(map[string]string),
	}
}

// AddIPAddress adds an IP address to the appropriate interface
func (m *Manager) AddIPAddress(ipAddr string, port int) error {
	if ipAddr == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

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
		return fmt.Errorf("failed to determine subnet mask for IP %s: %w", ipAddr, err)
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

	// Generate netplan configuration (but don't apply yet)
	if err := m.generateNetplan(); err != nil {
		return fmt.Errorf("failed to generate Netplan config: %w", err)
	}

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

	// Generate netplan configuration (but don't apply yet)
	if err := m.generateNetplan(); err != nil {
		return fmt.Errorf("failed to generate Netplan config: %w", err)
	}

	return nil
}

// ApplyNetplan applies the Netplan configuration
func (m *Manager) ApplyNetplan() error {
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
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

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
