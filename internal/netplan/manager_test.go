package netplan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bear-san/haproxy-configurator/internal/config"
	"github.com/bear-san/haproxy-configurator/internal/logger"
)

// setupTest initializes the logger for tests
func setupTest() {
	_ = logger.InitLogger(true)
}

func TestGetSubnetMaskForIP(t *testing.T) {
	setupTest()
	
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
	setupTest()
	
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
		return
	}
	if manager.config != cfg {
		t.Error("Manager config not set correctly")
	}
	if manager.addresses == nil {
		t.Error("Manager addresses map not initialized")
	}
}

func TestAddIPAddressWithoutNetplanCommand(t *testing.T) {
	setupTest()
	// Skip this test if running in CI or environments without netplan
	if _, err := os.Stat("/usr/sbin/netplan"); os.IsNotExist(err) {
		t.Skip("Skipping test: netplan command not available")
	}

	// Create temporary directory for test config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-netplan.yaml")

	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
			},
			ConfigPath:    configPath,
			BackupEnabled: false, // Disable backup for test
		},
	}

	manager := NewManager(cfg)

	// Test adding IP address
	err := manager.AddIPAddress("192.168.1.100", 80)
	if err != nil {
		t.Errorf("AddIPAddress failed: %v", err)
	}

	// Verify IP is tracked
	tracked := manager.GetTrackedAddresses()
	if len(tracked) != 1 {
		t.Errorf("Expected 1 tracked address, got %d", len(tracked))
	}
	if tracked["192.168.1.100"] != "eth0" {
		t.Errorf("Expected IP 192.168.1.100 to be tracked on eth0, got %s", tracked["192.168.1.100"])
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Netplan config file was not created")
	}

	// Test adding duplicate IP (should not error)
	err = manager.AddIPAddress("192.168.1.100", 443)
	if err != nil {
		t.Errorf("Adding duplicate IP should not error: %v", err)
	}

	// Test adding IP from unknown subnet
	err = manager.AddIPAddress("203.0.113.1", 80)
	if err == nil {
		t.Error("Expected error for IP in unknown subnet")
	}
}

func TestRemoveIPAddressWithoutNetplanCommand(t *testing.T) {
	setupTest()
	// Skip this test if running in CI or environments without netplan
	if _, err := os.Stat("/usr/sbin/netplan"); os.IsNotExist(err) {
		t.Skip("Skipping test: netplan command not available")
	}

	// Create temporary directory for test config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-netplan.yaml")

	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
			},
			ConfigPath:    configPath,
			BackupEnabled: false, // Disable backup for test
		},
	}

	manager := NewManager(cfg)

	// First add an IP
	err := manager.AddIPAddress("192.168.1.100", 80)
	if err != nil {
		t.Fatalf("AddIPAddress failed: %v", err)
	}

	// Verify it's tracked
	tracked := manager.GetTrackedAddresses()
	if len(tracked) != 1 {
		t.Fatalf("Expected 1 tracked address, got %d", len(tracked))
	}

	// Now remove it
	err = manager.RemoveIPAddress("192.168.1.100")
	if err != nil {
		t.Errorf("RemoveIPAddress failed: %v", err)
	}

	// Verify it's no longer tracked
	tracked = manager.GetTrackedAddresses()
	if len(tracked) != 0 {
		t.Errorf("Expected 0 tracked addresses after removal, got %d", len(tracked))
	}

	// Test removing non-existent IP
	err = manager.RemoveIPAddress("192.168.1.200")
	if err == nil {
		t.Error("Expected error when removing non-existent IP")
	}
}

func TestAddIPAddressValidation(t *testing.T) {
	setupTest()
	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
			},
			ConfigPath: "/tmp/test-netplan.yaml",
		},
	}

	manager := NewManager(cfg)

	// Test empty IP address
	err := manager.AddIPAddress("", 80)
	if err == nil {
		t.Error("Expected error for empty IP address")
	}

	// Test invalid IP address format
	err = manager.AddIPAddress("invalid-ip", 80)
	if err == nil {
		t.Error("Expected error for invalid IP address format")
	}
}

func TestTrackingMechanism(t *testing.T) {
	setupTest()
	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
				{
					Interface: "eth1",
					Subnets:   []string{"10.0.0.0/8"},
				},
			},
		},
	}

	manager := NewManager(cfg)

	// Test tracking state is initially empty
	tracked := manager.GetTrackedAddresses()
	if len(tracked) != 0 {
		t.Errorf("Expected empty tracking map, got %d entries", len(tracked))
	}

	// Manually add to tracking (simulating successful add)
	manager.addresses["192.168.1.100"] = "eth0"
	manager.addresses["10.0.0.50"] = "eth1"

	// Verify tracking
	tracked = manager.GetTrackedAddresses()
	if len(tracked) != 2 {
		t.Errorf("Expected 2 tracked addresses, got %d", len(tracked))
	}
	if tracked["192.168.1.100"] != "eth0" {
		t.Errorf("Expected 192.168.1.100 on eth0, got %s", tracked["192.168.1.100"])
	}
	if tracked["10.0.0.50"] != "eth1" {
		t.Errorf("Expected 10.0.0.50 on eth1, got %s", tracked["10.0.0.50"])
	}
}

func TestBackupFileCreation(t *testing.T) {
	setupTest()
	// This test verifies the backup logic without executing netplan commands
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-netplan.yaml")

	// Create an existing config file
	existingContent := []byte("network:\n  version: 2\n")
	if err := os.WriteFile(configPath, existingContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
			},
			ConfigPath:    configPath,
			BackupEnabled: true,
		},
	}

	manager := NewManager(cfg)

	// Call createBackup directly
	err := manager.createBackup(configPath)
	if err != nil {
		t.Errorf("createBackup failed: %v", err)
	}

	// Check if backup file was created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	backupFound := false
	for _, file := range files {
		if strings.Contains(file.Name(), "backup") && strings.Contains(file.Name(), "test-netplan.yaml") {
			backupFound = true
			break
		}
	}

	if !backupFound {
		// List all files for debugging
		t.Logf("Files in directory:")
		for _, file := range files {
			t.Logf("  %s", file.Name())
		}
		t.Error("Backup file was not created")
	}
}

func TestTransactionBasicFlow(t *testing.T) {
	setupTest()
	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
			},
			ConfigPath: "/tmp/test-netplan-transaction.yaml",
		},
	}

	manager := NewManager(cfg)
	transactionID := "test-tx-123"

	// Test adding IP address to transaction
	err := manager.AddIPAddressToTransaction(transactionID, "192.168.1.100", 80)
	if err != nil {
		t.Errorf("Failed to add IP to transaction: %v", err)
	}

	// Test adding removal to same transaction
	err = manager.RemoveIPAddressFromTransaction(transactionID, "192.168.1.101")
	if err != nil {
		t.Errorf("Failed to add removal to transaction: %v", err)
	}

	// Load transaction and verify changes
	transaction, err := manager.loadTransaction(transactionID)
	if err != nil {
		t.Errorf("Failed to load transaction: %v", err)
	}

	if len(transaction.Changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(transaction.Changes))
	}

	if transaction.Changes[0].Operation != "add" || transaction.Changes[0].IPAddress != "192.168.1.100" {
		t.Errorf("First change incorrect: %+v", transaction.Changes[0])
	}

	if transaction.Changes[1].Operation != "remove" || transaction.Changes[1].IPAddress != "192.168.1.101" {
		t.Errorf("Second change incorrect: %+v", transaction.Changes[1])
	}

	// Clean up transaction file
	transactionFile := filepath.Join(manager.transactionDir, fmt.Sprintf("transaction-%s.json", transactionID))
	_ = os.Remove(transactionFile)
}

func TestTransactionCommit(t *testing.T) {
	setupTest()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-netplan.yaml")

	cfg := &config.NetplanConfig{
		Netplan: config.NetplanSettings{
			InterfaceMappings: []config.InterfaceMapping{
				{
					Interface: "eth0",
					Subnets:   []string{"192.168.1.0/24"},
				},
			},
			ConfigPath:    configPath,
			BackupEnabled: false,
		},
	}

	manager := NewManager(cfg)
	transactionID := "test-commit-tx-456"

	// Add changes to transaction
	err := manager.AddIPAddressToTransaction(transactionID, "192.168.1.100", 80)
	if err != nil {
		t.Errorf("Failed to add IP to transaction: %v", err)
	}

	err = manager.AddIPAddressToTransaction(transactionID, "192.168.1.101", 443)
	if err != nil {
		t.Errorf("Failed to add second IP to transaction: %v", err)
	}

	// Commit transaction
	err = manager.CommitTransaction(transactionID)
	if err != nil {
		t.Errorf("Failed to commit transaction: %v", err)
	}

	// Verify Netplan config was updated
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Netplan config file was not created")
	}

	// Verify tracking was updated
	tracked := manager.GetTrackedAddresses()
	if len(tracked) != 2 {
		t.Errorf("Expected 2 tracked addresses, got %d", len(tracked))
	}

	if tracked["192.168.1.100"] != "eth0" {
		t.Errorf("IP 192.168.1.100 not tracked correctly")
	}

	if tracked["192.168.1.101"] != "eth0" {
		t.Errorf("IP 192.168.1.101 not tracked correctly")
	}

	// Verify transaction was moved to committed directory
	committedFile := filepath.Join(manager.transactionDir, "committed", fmt.Sprintf("transaction-%s.json", transactionID))
	if _, err := os.Stat(committedFile); os.IsNotExist(err) {
		t.Error("Transaction was not moved to committed directory")
	}
}

