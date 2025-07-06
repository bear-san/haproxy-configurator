package config

import (
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v3"
)

// NetplanConfig represents the configuration for Netplan integration
type NetplanConfig struct {
	Netplan NetplanSettings `yaml:"netplan"`
}

// NetplanSettings contains the Netplan-specific settings
type NetplanSettings struct {
	InterfaceMappings []InterfaceMapping `yaml:"interface_mappings"`
	ConfigPath        string             `yaml:"netplan_config_path"`
	BackupEnabled     bool               `yaml:"backup_enabled"`
}

// InterfaceMapping defines which subnets can be assigned to which interface
type InterfaceMapping struct {
	Interface string   `yaml:"interface"`
	Subnets   []string `yaml:"subnets"`
}

// LoadNetplanConfig loads the Netplan configuration from a file
func LoadNetplanConfig(configPath string) (*NetplanConfig, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is required")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config NetplanConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// FindInterfaceForIP finds the appropriate interface for the given IP address
func (c *NetplanConfig) FindInterfaceForIP(ipAddr string) (string, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	for _, mapping := range c.Netplan.InterfaceMappings {
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

// ValidateConfig validates the Netplan configuration
func (c *NetplanConfig) ValidateConfig() error {
	if len(c.Netplan.InterfaceMappings) == 0 {
		return fmt.Errorf("at least one interface mapping is required")
	}

	if c.Netplan.ConfigPath == "" {
		c.Netplan.ConfigPath = "/etc/netplan/99-haproxy-configurator.yaml"
	}

	for i, mapping := range c.Netplan.InterfaceMappings {
		if mapping.Interface == "" {
			return fmt.Errorf("interface name is required for mapping %d", i)
		}
		if len(mapping.Subnets) == 0 {
			return fmt.Errorf("at least one subnet is required for interface %s", mapping.Interface)
		}
		for j, subnet := range mapping.Subnets {
			if _, _, err := net.ParseCIDR(subnet); err != nil {
				return fmt.Errorf("invalid CIDR %s for interface %s at index %d: %w", subnet, mapping.Interface, j, err)
			}
		}
	}

	return nil
}

