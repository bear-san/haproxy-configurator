package config

import (
	"os"
	"testing"
)

func TestLoadNetplanConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `netplan:
  interface_mappings:
    - interface: "eth0"
      subnets:
        - "192.168.1.0/24"
        - "10.0.0.0/24"
    - interface: "eth1"
      subnets:
        - "172.16.0.0/16"
  netplan_config_path: "/etc/netplan/99-haproxy.yaml"
  backup_enabled: true
`

	tmpfile, err := os.CreateTemp("", "netplan-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading the config
	cfg, err := LoadNetplanConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify the loaded configuration
	if len(cfg.Netplan.InterfaceMappings) != 2 {
		t.Errorf("Expected 2 interface mappings, got %d", len(cfg.Netplan.InterfaceMappings))
	}

	if cfg.Netplan.ConfigPath != "/etc/netplan/99-haproxy.yaml" {
		t.Errorf("Expected config path '/etc/netplan/99-haproxy.yaml', got '%s'", cfg.Netplan.ConfigPath)
	}

	if !cfg.Netplan.BackupEnabled {
		t.Error("Expected backup to be enabled")
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := &NetplanConfig{
		Netplan: NetplanSettings{
			InterfaceMappings: []InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24", "10.0.0.0/24"},
				},
			},
			ConfigPath:    "/etc/netplan/99-haproxy.yaml",
			BackupEnabled: true,
		},
	}

	if err := cfg.ValidateConfig(); err != nil {
		t.Errorf("Valid config failed validation: %v", err)
	}
}

func TestValidateConfigWithInvalidCIDR(t *testing.T) {
	cfg := &NetplanConfig{
		Netplan: NetplanSettings{
			InterfaceMappings: []InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/99"}, // Invalid CIDR
				},
			},
			ConfigPath:    "/etc/netplan/99-haproxy.yaml",
			BackupEnabled: true,
		},
	}

	if err := cfg.ValidateConfig(); err == nil {
		t.Error("Expected validation to fail for invalid CIDR")
	}
}

func TestFindInterfaceForIP(t *testing.T) {
	cfg := &NetplanConfig{
		Netplan: NetplanSettings{
			InterfaceMappings: []InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24", "10.0.0.0/24"},
				},
				{
					Interface: "eth1",
					Subnets:   []string{"172.16.0.0/16"},
				},
			},
		},
	}

	testCases := []struct {
		ip           string
		expectedIface string
		expectError   bool
	}{
		{"192.168.1.100", "eth0", false},
		{"10.0.0.50", "eth0", false},
		{"172.16.1.1", "eth1", false},
		{"203.0.113.1", "", true}, // Not in any subnet
	}

	for _, tc := range testCases {
		iface, err := cfg.FindInterfaceForIP(tc.ip)
		if tc.expectError {
			if err == nil {
				t.Errorf("Expected error for IP %s, but got none", tc.ip)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for IP %s: %v", tc.ip, err)
			}
			if iface != tc.expectedIface {
				t.Errorf("Expected interface %s for IP %s, got %s", tc.expectedIface, tc.ip, iface)
			}
		}
	}
}

func TestFindInterfaceForIPParsesSubnets(t *testing.T) {
	cfg := &NetplanConfig{
		Netplan: NetplanSettings{
			InterfaceMappings: []InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
			},
		},
	}

	// This should work since we're testing the subnet parsing logic
	iface, err := cfg.FindInterfaceForIP("192.168.1.1")
	if err != nil {
		t.Errorf("Failed to find interface: %v", err)
	}
	if iface != "eth0" {
		t.Errorf("Expected eth0, got %s", iface)
	}
}