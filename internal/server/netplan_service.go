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

	// Handle Netplan IP address assignment via transaction
	if s.netplanMgr != nil && req.Bind != nil && req.Bind.Address != "" {
		port := int(req.Bind.Port)
		log.Printf("Adding IP address %s to Netplan transaction %s", req.Bind.Address, req.TransactionId)

		if err := s.netplanMgr.AddIPAddressToTransaction(req.TransactionId, req.Bind.Address, port); err != nil {
			log.Printf("WARNING: Failed to add IP address %s to Netplan transaction: %v", req.Bind.Address, err)
			log.Printf("Continuing with HAProxy bind creation without Netplan integration")
			// Continue with bind creation even if Netplan transaction fails
		} else {
			log.Printf("Successfully added IP address %s to Netplan transaction %s", req.Bind.Address, req.TransactionId)
		}
	} else {
		log.Printf("Netplan integration disabled or no IP address specified, creating HAProxy bind only")
	}

	// Create the bind in HAProxy
	bind := convertBindFromProto(req.Bind)
	created, err := s.client.AddBind(req.FrontendName, req.TransactionId, *bind)
	if err != nil {
		// HAProxy bind creation failed - no need to rollback since we're using transactions
		// The transaction will not be committed if HAProxy fails
		log.Printf("HAProxy bind creation failed: %v", err)
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
			log.Printf("Found bind address %s to add to Netplan transaction for removal", bindAddress)
		} else {
			log.Printf("Could not retrieve bind address for Netplan transaction: %v", err)
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

	// Add IP address removal to Netplan transaction
	if s.netplanMgr != nil && bindAddress != "" {
		log.Printf("Adding IP address %s removal to Netplan transaction %s", bindAddress, req.TransactionId)
		if err := s.netplanMgr.RemoveIPAddressFromTransaction(req.TransactionId, bindAddress); err != nil {
			log.Printf("WARNING: Failed to add IP address %s removal to Netplan transaction: %v", bindAddress, err)
			// Don't fail the entire operation for Netplan transaction errors
		} else {
			log.Printf("Successfully added IP address %s removal to Netplan transaction %s", bindAddress, req.TransactionId)
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

	// Commit Netplan transaction and apply configuration after successful HAProxy commit
	if s.netplanMgr != nil {
		log.Printf("Committing Netplan transaction %s", req.TransactionId)
		if netplanErr := s.netplanMgr.CommitTransaction(req.TransactionId); netplanErr != nil {
			log.Printf("WARNING: Failed to commit Netplan transaction %s: %v", req.TransactionId, netplanErr)
			log.Printf("HAProxy transaction has been committed successfully, but Netplan changes may not be applied")
			// Log the error but don't fail the transaction commit
			// The HAProxy changes are already committed at this point
		} else {
			log.Printf("Successfully committed Netplan transaction %s", req.TransactionId)

			// Apply Netplan configuration after successful transaction commit
			log.Printf("Applying Netplan configuration")
			if applyErr := s.netplanMgr.ApplyNetplan(); applyErr != nil {
				log.Printf("WARNING: Failed to apply Netplan configuration: %v", applyErr)
				log.Printf("Netplan configuration files have been updated, but network changes may not be active")
			} else {
				log.Printf("Successfully applied Netplan configuration")
			}
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
