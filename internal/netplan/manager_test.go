package netplan

import (
	"testing"

	"github.com/bear-san/haproxy-configurator/internal/config"
)

func TestGetSubnetMaskForIP(t *testing.T) {
	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24", "10.0.0.0/8"},
				},
				{
					Interface: "eth1",
					Subnets:   []string{"172.16.0.0/16"},
				},
			},
		},
	}

	manager := NewManager(cfg)

	testCases := []struct {
		ip           string
		expectedMask string
		expectError  bool
	}{
		{"192.168.1.100", "/24", false},
		{"10.5.5.5", "/8", false},
		{"172.16.1.1", "/16", false},
		{"203.0.113.1", "/32", true}, // Not in any subnet, should default to /32
		{"invalid-ip", "", true},     // Invalid IP
	}

	for _, tc := range testCases {
		mask, err := manager.getSubnetMaskForIP(tc.ip)
		if tc.expectError {
			if err == nil {
				t.Errorf("Expected error for IP %s, but got none", tc.ip)
			}
			// For IPs not in any subnet, we still expect /32 to be returned
			if tc.ip == "203.0.113.1" && mask != "/32" {
				t.Errorf("Expected /32 for IP %s not in any subnet, got %s", tc.ip, mask)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for IP %s: %v", tc.ip, err)
			}
			if mask != tc.expectedMask {
				t.Errorf("Expected mask %s for IP %s, got %s", tc.expectedMask, tc.ip, mask)
			}
		}
	}
}

func TestNewManager(t *testing.T) {
	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
			},
		},
	}

	manager := NewManager(cfg)
	if manager == nil {
		t.Error("NewManager returned nil")
	}
	if manager.config != cfg {
		t.Error("Manager config not set correctly")
	}
	if manager.addresses == nil {
		t.Error("Manager addresses map not initialized")
	}
}