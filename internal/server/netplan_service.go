package server

import (
	"github.com/bear-san/haproxy-configurator/internal/logger"
	pb "github.com/bear-san/haproxy-configurator/pkg/haproxy/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


// CreateBindWithNetplan creates a bind configuration and manages IP address assignment
func (s *HAProxyManagerServer) CreateBindWithNetplan(req *pb.CreateBindRequest) (*pb.CreateBindResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "transaction ID is required")
	}

	logger.GetLogger().Info("Creating bind with Netplan integration",
		zap.String("frontend_name", req.FrontendName),
		zap.String("address", req.Bind.Address),
		zap.Int32("port", req.Bind.Port),
		zap.String("transaction_id", req.TransactionId))

	// Handle Netplan IP address assignment via transaction
	if s.netplanMgr != nil && req.Bind != nil && req.Bind.Address != "" {
		port := int(req.Bind.Port)
		logger.GetLogger().Debug("Adding IP address to Netplan transaction",
			zap.String("ip_address", req.Bind.Address),
			zap.String("transaction_id", req.TransactionId))

		if err := s.netplanMgr.AddIPAddressToTransaction(req.TransactionId, req.Bind.Address, port); err != nil {
			logger.GetLogger().Warn("Failed to add IP address to Netplan transaction, continuing without Netplan integration",
				zap.String("ip_address", req.Bind.Address),
				zap.String("transaction_id", req.TransactionId),
				zap.Error(err))
			// Continue with bind creation even if Netplan transaction fails
		} else {
			logger.GetLogger().Debug("Successfully added IP address to Netplan transaction",
				zap.String("ip_address", req.Bind.Address),
				zap.String("transaction_id", req.TransactionId))
		}
	} else {
		logger.GetLogger().Debug("Netplan integration disabled or no IP address specified, creating HAProxy bind only")
	}

	// Create the bind in HAProxy
	bind := convertBindFromProto(req.Bind)
	created, err := s.client.AddBind(req.FrontendName, req.TransactionId, *bind)
	if err != nil {
		// HAProxy bind creation failed - no need to rollback since we're using transactions
		// The transaction will not be committed if HAProxy fails
		logger.GetLogger().Error("HAProxy bind creation failed",
			zap.String("frontend_name", req.FrontendName),
			zap.String("transaction_id", req.TransactionId),
			zap.Error(err))
		return nil, handleHAProxyError(err)
	}

	return &pb.CreateBindResponse{
		Bind: convertBindToProto(created),
	}, nil
}

// DeleteBindWithNetplan removes a bind configuration and cleans up IP address assignment
func (s *HAProxyManagerServer) DeleteBindWithNetplan(req *pb.DeleteBindRequest) (*pb.DeleteBindResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "transaction ID is required")
	}

	logger.GetLogger().Info("Deleting bind with Netplan integration",
		zap.String("frontend_name", req.FrontendName),
		zap.String("bind_name", req.Name),
		zap.String("transaction_id", req.TransactionId))

	// Get the bind configuration first to extract the IP address
	var bindAddress string
	if s.netplanMgr != nil {
		bind, err := s.client.GetBind(req.Name, req.FrontendName, req.TransactionId)
		if err == nil && bind.Address != nil {
			bindAddress = *bind.Address
			logger.GetLogger().Debug("Found bind address for Netplan transaction removal",
				zap.String("bind_address", bindAddress))
		} else {
			logger.GetLogger().Warn("Could not retrieve bind address for Netplan transaction",
				zap.String("bind_name", req.Name),
				zap.Error(err))
		}
	}

	// Delete the bind from HAProxy
	logger.GetLogger().Debug("Deleting bind from HAProxy",
		zap.String("bind_name", req.Name))
	err := s.client.DeleteBind(req.Name, req.FrontendName, req.TransactionId)
	if err != nil {
		logger.GetLogger().Error("Failed to delete bind from HAProxy",
			zap.String("bind_name", req.Name),
			zap.Error(err))
		return nil, handleHAProxyError(err)
	}
	logger.GetLogger().Debug("Successfully deleted bind from HAProxy",
		zap.String("bind_name", req.Name))

	// Add IP address removal to Netplan transaction
	if s.netplanMgr != nil && bindAddress != "" {
		logger.GetLogger().Debug("Adding IP address removal to Netplan transaction",
			zap.String("ip_address", bindAddress),
			zap.String("transaction_id", req.TransactionId))
		if err := s.netplanMgr.RemoveIPAddressFromTransaction(req.TransactionId, bindAddress); err != nil {
			logger.GetLogger().Warn("Failed to add IP address removal to Netplan transaction",
				zap.String("ip_address", bindAddress),
				zap.String("transaction_id", req.TransactionId),
				zap.Error(err))
			// Don't fail the entire operation for Netplan transaction errors
		} else {
			logger.GetLogger().Debug("Successfully added IP address removal to Netplan transaction",
				zap.String("ip_address", bindAddress),
				zap.String("transaction_id", req.TransactionId))
		}
	} else {
		logger.GetLogger().Debug("No IP address to remove from Netplan or Netplan integration disabled")
	}

	return &pb.DeleteBindResponse{}, nil
}

// CommitTransactionWithNetplan commits the transaction and applies Netplan changes
func (s *HAProxyManagerServer) CommitTransactionWithNetplan(req *pb.CommitTransactionRequest) (*pb.CommitTransactionResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "transaction ID is required")
	}

	logger.GetLogger().Info("Committing transaction with Netplan integration",
		zap.String("transaction_id", req.TransactionId))

	// Commit HAProxy transaction first
	logger.GetLogger().Debug("Committing HAProxy transaction",
		zap.String("transaction_id", req.TransactionId))
	transaction, err := s.client.CommitTransaction(req.TransactionId)
	if err != nil {
		logger.GetLogger().Error("Failed to commit HAProxy transaction",
			zap.String("transaction_id", req.TransactionId),
			zap.Error(err))
		return nil, handleHAProxyError(err)
	}
	logger.GetLogger().Info("Successfully committed HAProxy transaction",
		zap.String("transaction_id", req.TransactionId))

	// Commit Netplan transaction and apply configuration after successful HAProxy commit
	if s.netplanMgr != nil {
		logger.GetLogger().Debug("Committing Netplan transaction",
			zap.String("transaction_id", req.TransactionId))
		if netplanErr := s.netplanMgr.CommitTransaction(req.TransactionId); netplanErr != nil {
			logger.GetLogger().Warn("Failed to commit Netplan transaction, HAProxy changes are committed but Netplan changes may not be applied",
				zap.String("transaction_id", req.TransactionId),
				zap.Error(netplanErr))
			// Log the error but don't fail the transaction commit
			// The HAProxy changes are already committed at this point
		} else {
			logger.GetLogger().Info("Successfully committed Netplan transaction",
				zap.String("transaction_id", req.TransactionId))

			// Apply Netplan configuration after successful transaction commit
			logger.GetLogger().Debug("Applying Netplan configuration")
			if applyErr := s.netplanMgr.ApplyNetplan(); applyErr != nil {
				logger.GetLogger().Warn("Failed to apply Netplan configuration, files updated but network changes may not be active",
					zap.Error(applyErr))
			} else {
				logger.GetLogger().Info("Successfully applied Netplan configuration")
			}
		}
	} else {
		logger.GetLogger().Debug("Netplan integration disabled, transaction commit complete")
	}

	return &pb.CommitTransactionResponse{
		Transaction: convertTransactionToProto(transaction),
	}, nil
}

// GetNetplanStatus returns the current status of Netplan integration
func (s *HAProxyManagerServer) GetNetplanStatus() map[string]interface{} {
	status := make(map[string]interface{})

	if s.config == nil || !s.config.HasNetplanIntegration() {
		status["enabled"] = false
		status["message"] = "Netplan integration disabled"
		return status
	}

	status["enabled"] = true
	status["config_path"] = s.config.Netplan.ConfigPath
	status["backup_enabled"] = s.config.Netplan.BackupEnabled
	status["interface_mappings"] = len(s.config.Netplan.InterfaceMappings)

	if s.netplanMgr != nil {
		status["tracked_addresses"] = s.netplanMgr.GetTrackedAddresses()
	}

	return status
}
