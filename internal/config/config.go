package config

import (
	"fmt"
	"net"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the unified configuration for the HAProxy Configurator
type Config struct {
	HAProxy HAProxySettings `yaml:"haproxy"`
	Netplan NetplanSettings `yaml:"netplan,omitempty"`
}

// HAProxySettings contains the HAProxy Data Plane API settings
type HAProxySettings struct {
	APIURL   string `yaml:"api_url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// NetplanSettings contains the Netplan-specific settings
type NetplanSettings struct {
	InterfaceMappings []InterfaceMapping `yaml:"interface_mappings"`
	ConfigPath        string             `yaml:"netplan_config_path"`
	BackupEnabled     bool               `yaml:"backup_enabled"`
	TransactionDir    string             `yaml:"transaction_dir,omitempty"`
}

// InterfaceMapping defines which subnets can be assigned to which interface
type InterfaceMapping struct {
	Interface string   `yaml:"interface"`
	Subnets   []string `yaml:"subnets"`
}

// LoadConfig loads the unified configuration from a file
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is required")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults for HAProxy settings if not specified
	if config.HAProxy.APIURL == "" {
		config.HAProxy.APIURL = getEnvWithDefault("HAPROXY_API_URL", "http://localhost:5555")
	}
	if config.HAProxy.Username == "" {
		config.HAProxy.Username = getEnvWithDefault("HAPROXY_API_USERNAME", "admin")
	}
	if config.HAProxy.Password == "" {
		config.HAProxy.Password = getEnvWithDefault("HAPROXY_API_PASSWORD", "admin")
	}

	return &config, nil
}

// getEnvWithDefault returns the environment variable value or a default if not set
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// FindInterfaceForIP finds the appropriate interface for the given IP address
func (c *Config) FindInterfaceForIP(ipAddr string) (string, error) {
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

// ValidateConfig validates the configuration
func (c *Config) ValidateConfig() error {
	// Validate HAProxy settings
	if c.HAProxy.APIURL == "" {
		return fmt.Errorf("HAProxy API URL is required")
	}
	if c.HAProxy.Username == "" {
		return fmt.Errorf("HAProxy API username is required")
	}
	if c.HAProxy.Password == "" {
		return fmt.Errorf("HAProxy API password is required")
	}

	// Validate Netplan settings (only if Netplan integration is enabled)
	if len(c.Netplan.InterfaceMappings) > 0 {
		if c.Netplan.ConfigPath == "" {
			c.Netplan.ConfigPath = "/etc/netplan/99-haproxy-configurator.yaml"
		}

		if c.Netplan.TransactionDir == "" {
			c.Netplan.TransactionDir = "/tmp/haproxy-netplan-transactions"
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
	}

	return nil
}

// HasNetplanIntegration returns true if Netplan integration is configured
func (c *Config) HasNetplanIntegration() bool {
	return len(c.Netplan.InterfaceMappings) > 0
}
