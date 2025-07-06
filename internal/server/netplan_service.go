package server

import (
	"log"

	"github.com/bear-san/haproxy-configurator/internal/config"
	"github.com/bear-san/haproxy-configurator/internal/netplan"
	pb "github.com/bear-san/haproxy-configurator/pkg/haproxy/v1"
)

// SetNetplanConfig initializes the Netplan configuration for the server
func (s *HAProxyManagerServer) SetNetplanConfig(configPath string) error {
	if configPath == "" {
		// Netplan integration is disabled
		s.netplanConfig = nil
		s.netplanMgr = nil
		return nil
	}

	// Load Netplan configuration
	cfg, err := config.LoadNetplanConfig(configPath)
	if err != nil {
		return err
	}

	// Validate configuration
	if err := cfg.ValidateConfig(); err != nil {
		return err
	}

	// Initialize Netplan manager
	s.netplanConfig = cfg
	s.netplanMgr = netplan.NewManager(cfg)

	log.Printf("Netplan integration enabled with config: %s", configPath)
	return nil
}

// CreateBindWithNetplan creates a bind configuration and manages IP address assignment
func (s *HAProxyManagerServer) CreateBindWithNetplan(req *pb.CreateBindRequest) (*pb.CreateBindResponse, error) {
	log.Printf("Creating bind with Netplan integration for frontend %s, address %s:%d",
		req.FrontendName, req.Bind.Address, req.Bind.Port)

	// Handle Netplan IP address assignment before creating bind
	if s.netplanMgr != nil && req.Bind != nil && req.Bind.Address != "" {
		port := int(req.Bind.Port)
		log.Printf("Attempting to add IP address %s to Netplan before HAProxy bind creation", req.Bind.Address)

		if err := s.netplanMgr.AddIPAddress(req.Bind.Address, port); err != nil {
			log.Printf("WARNING: Failed to add IP address %s to Netplan: %v", req.Bind.Address, err)
			log.Printf("Continuing with HAProxy bind creation without Netplan integration")
			// Continue with bind creation even if Netplan fails
		} else {
			log.Printf("Successfully added IP address %s to Netplan configuration", req.Bind.Address)
		}
	} else {
		log.Printf("Netplan integration disabled or no IP address specified, creating HAProxy bind only")
	}

	// Create the bind in HAProxy
	bind := convertBindFromProto(req.Bind)
	created, err := s.client.AddBind(req.FrontendName, req.TransactionId, *bind)
	if err != nil {
		// If HAProxy bind creation fails, try to rollback Netplan changes
		if s.netplanMgr != nil && req.Bind != nil && req.Bind.Address != "" {
			if rollbackErr := s.netplanMgr.RemoveIPAddress(req.Bind.Address); rollbackErr != nil {
				log.Printf("Failed to rollback Netplan IP address %s: %v", req.Bind.Address, rollbackErr)
			}
		}
		return nil, handleHAProxyError(err)
	}

	return &pb.CreateBindResponse{
		Bind: convertBindToProto(created),
	}, nil
}

// DeleteBindWithNetplan removes a bind configuration and cleans up IP address assignment
func (s *HAProxyManagerServer) DeleteBindWithNetplan(req *pb.DeleteBindRequest) (*pb.DeleteBindResponse, error) {
	log.Printf("Deleting bind with Netplan integration for frontend %s, bind %s",
		req.FrontendName, req.Name)

	// Get the bind configuration first to extract the IP address
	var bindAddress string
	if s.netplanMgr != nil {
		bind, err := s.client.GetBind(req.Name, req.FrontendName, req.TransactionId)
		if err == nil && bind.Address != nil {
			bindAddress = *bind.Address
			log.Printf("Found bind address %s to clean up from Netplan", bindAddress)
		} else {
			log.Printf("Could not retrieve bind address for Netplan cleanup: %v", err)
		}
	}

	// Delete the bind from HAProxy
	log.Printf("Deleting bind %s from HAProxy", req.Name)
	err := s.client.DeleteBind(req.Name, req.FrontendName, req.TransactionId)
	if err != nil {
		log.Printf("Failed to delete bind %s from HAProxy: %v", req.Name, err)
		return nil, handleHAProxyError(err)
	}
	log.Printf("Successfully deleted bind %s from HAProxy", req.Name)

	// Remove IP address from Netplan after successful HAProxy deletion
	if s.netplanMgr != nil && bindAddress != "" {
		log.Printf("Attempting to remove IP address %s from Netplan", bindAddress)
		if netplanErr := s.netplanMgr.RemoveIPAddress(bindAddress); netplanErr != nil {
			log.Printf("WARNING: Failed to remove IP address %s from Netplan: %v", bindAddress, netplanErr)
			// Don't fail the entire operation for Netplan errors
		} else {
			log.Printf("Successfully removed IP address %s from Netplan configuration", bindAddress)
		}
	} else {
		log.Printf("No IP address to remove from Netplan or Netplan integration disabled")
	}

	return &pb.DeleteBindResponse{}, nil
}

// CommitTransactionWithNetplan commits the transaction and applies Netplan changes
func (s *HAProxyManagerServer) CommitTransactionWithNetplan(req *pb.CommitTransactionRequest) (*pb.CommitTransactionResponse, error) {
	log.Printf("Committing transaction %s with Netplan integration", req.TransactionId)

	// Commit HAProxy transaction first
	log.Printf("Committing HAProxy transaction %s", req.TransactionId)
	transaction, err := s.client.CommitTransaction(req.TransactionId)
	if err != nil {
		log.Printf("Failed to commit HAProxy transaction %s: %v", req.TransactionId, err)
		return nil, handleHAProxyError(err)
	}
	log.Printf("Successfully committed HAProxy transaction %s", req.TransactionId)

	// Apply Netplan configuration after successful HAProxy commit
	if s.netplanMgr != nil {
		log.Printf("Applying Netplan configuration after successful HAProxy commit")
		if netplanErr := s.netplanMgr.ApplyNetplan(); netplanErr != nil {
			log.Printf("WARNING: Failed to apply Netplan configuration: %v", netplanErr)
			log.Printf("HAProxy transaction has been committed successfully, but Netplan changes may not be active")
			// Log the error but don't fail the transaction commit
			// The HAProxy changes are already committed at this point
		} else {
			log.Printf("Successfully applied Netplan configuration")
		}
	} else {
		log.Printf("Netplan integration disabled, transaction commit complete")
	}

	return &pb.CommitTransactionResponse{
		Transaction: convertTransactionToProto(transaction),
	}, nil
}

// GetNetplanStatus returns the current status of Netplan integration
func (s *HAProxyManagerServer) GetNetplanStatus() map[string]interface{} {
	status := make(map[string]interface{})

	if s.netplanConfig == nil {
		status["enabled"] = false
		status["message"] = "Netplan integration disabled"
		return status
	}

	status["enabled"] = true
	status["config_path"] = s.netplanConfig.Netplan.ConfigPath
	status["backup_enabled"] = s.netplanConfig.Netplan.BackupEnabled
	status["interface_mappings"] = len(s.netplanConfig.Netplan.InterfaceMappings)

	if s.netplanMgr != nil {
		status["tracked_addresses"] = s.netplanMgr.GetTrackedAddresses()
	}

	return status
}
