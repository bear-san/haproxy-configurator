package server

import (
	v3 "github.com/bear-san/haproxy-go/dataplane/v3"
	pb "github.com/bear-san/haproxy-network-manager/pkg/haproxy/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Helper functions for error handling
func handleHAProxyError(err error) error {
	if err == nil {
		return nil
	}

	switch e := err.(type) {
	case *v3.NotFoundError:
		return status.Errorf(codes.NotFound, "resource not found: %s", e.Message)
	case *v3.UnauthorizedError:
		return status.Errorf(codes.Unauthenticated, "authentication failed: %s", e.Message)
	case *v3.BadRequestError:
		return status.Errorf(codes.InvalidArgument, "bad request: %s", e.Message)
	case *v3.ConflictError:
		return status.Errorf(codes.AlreadyExists, "conflict: %s", e.Message)
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}

// Helper functions for type conversions
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(i int32) *int {
	val := int(i)
	return &val
}

func boolPtr(b bool) *bool {
	return &b
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int32 {
	if i == nil {
		return 0
	}
	return int32(*i)
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// Conversion functions between protobuf and haproxy-go types

// convertProxyMode converts between pb.ProxyMode and string
func convertProxyMode(mode pb.ProxyMode) string {
	switch mode {
	case pb.ProxyMode_PROXY_MODE_TCP:
		return "tcp"
	case pb.ProxyMode_PROXY_MODE_HTTP:
		return "http"
	default:
		return "tcp"
	}
}

// convertProxyModeToProto converts string to pb.ProxyMode
func convertProxyModeToProto(mode string) pb.ProxyMode {
	switch mode {
	case "tcp":
		return pb.ProxyMode_PROXY_MODE_TCP
	case "http":
		return pb.ProxyMode_PROXY_MODE_HTTP
	default:
		return pb.ProxyMode_PROXY_MODE_UNSPECIFIED
	}
}

// convertBalanceAlgorithm converts between pb.BalanceAlgorithm and string
func convertBalanceAlgorithm(algo pb.BalanceAlgorithm) string {
	switch algo {
	case pb.BalanceAlgorithm_BALANCE_ALGORITHM_FIRST:
		return "first"
	case pb.BalanceAlgorithm_BALANCE_ALGORITHM_HASH:
		return "hash"
	case pb.BalanceAlgorithm_BALANCE_ALGORITHM_RANDOM:
		return "random"
	case pb.BalanceAlgorithm_BALANCE_ALGORITHM_ROUNDROBIN:
		return "roundrobin"
	default:
		return "roundrobin"
	}
}

// convertBalanceAlgorithmToProto converts string to pb.BalanceAlgorithm
func convertBalanceAlgorithmToProto(algo string) pb.BalanceAlgorithm {
	switch algo {
	case "first":
		return pb.BalanceAlgorithm_BALANCE_ALGORITHM_FIRST
	case "hash":
		return pb.BalanceAlgorithm_BALANCE_ALGORITHM_HASH
	case "random":
		return pb.BalanceAlgorithm_BALANCE_ALGORITHM_RANDOM
	case "roundrobin":
		return pb.BalanceAlgorithm_BALANCE_ALGORITHM_ROUNDROBIN
	default:
		return pb.BalanceAlgorithm_BALANCE_ALGORITHM_UNSPECIFIED
	}
}

// convertBackendToProto converts v3.Backend to pb.Backend
func convertBackendToProto(backend *v3.Backend) *pb.Backend {
	if backend == nil {
		return nil
	}

	result := &pb.Backend{
		Id:   derefInt(backend.Id),
		Name: derefString(backend.Name),
		Mode: convertProxyModeToProto(backend.Mode),
	}

	if backend.Balance != nil && backend.Balance.Algorithm != "" {
		result.Balance = &pb.BackendBalance{
			Algorithm: convertBalanceAlgorithmToProto(backend.Balance.Algorithm),
		}
	}

	return result
}

// convertBackendFromProto converts pb.Backend to v3.Backend
func convertBackendFromProto(backend *pb.Backend) *v3.Backend {
	if backend == nil {
		return nil
	}

	mode := convertProxyMode(backend.Mode)
	result := &v3.Backend{
		Id:   intPtr(backend.Id),
		Name: stringPtr(backend.Name),
		Mode: mode,
	}

	if backend.Balance != nil {
		algo := convertBalanceAlgorithm(backend.Balance.Algorithm)
		result.Balance = &v3.BackendBalance{
			Algorithm: algo,
		}
	}

	return result
}

// convertFrontendToProto converts v3.Frontend to pb.Frontend
func convertFrontendToProto(frontend *v3.Frontend) *pb.Frontend {
	if frontend == nil {
		return nil
	}

	return &pb.Frontend{
		Id:             derefInt(frontend.Id),
		Name:           derefString(frontend.Name),
		DefaultBackend: derefString(frontend.DefaultBackend),
		Description:    derefString(frontend.Description),
		Disabled:       derefBool(frontend.Disabled),
		Enabled:        derefBool(frontend.Enabled),
		Mode:           convertProxyModeToProto(derefString(frontend.Mode)),
	}
}

// convertFrontendFromProto converts pb.Frontend to v3.Frontend
func convertFrontendFromProto(frontend *pb.Frontend) *v3.Frontend {
	if frontend == nil {
		return nil
	}

	mode := convertProxyMode(frontend.Mode)
	return &v3.Frontend{
		Id:             intPtr(frontend.Id),
		Name:           stringPtr(frontend.Name),
		DefaultBackend: stringPtr(frontend.DefaultBackend),
		Description:    stringPtr(frontend.Description),
		Disabled:       boolPtr(frontend.Disabled),
		Enabled:        boolPtr(frontend.Enabled),
		Mode:           &mode,
	}
}

// convertServerToProto converts v3.Server to pb.Server
func convertServerToProto(server *v3.Server) *pb.Server {
	if server == nil {
		return nil
	}

	return &pb.Server{
		Id:      derefString(server.Id),
		Name:    derefString(server.Name),
		Address: derefString(server.Address),
		Port:    derefInt(server.Port),
	}
}

// convertServerFromProto converts pb.Server to v3.Server
func convertServerFromProto(server *pb.Server) *v3.Server {
	if server == nil {
		return nil
	}

	return &v3.Server{
		Id:      stringPtr(server.Id),
		Name:    stringPtr(server.Name),
		Address: stringPtr(server.Address),
		Port:    intPtr(server.Port),
	}
}

// convertBindToProto converts v3.Bind to pb.Bind
func convertBindToProto(bind *v3.Bind) *pb.Bind {
	if bind == nil {
		return nil
	}

	return &pb.Bind{
		Id:      derefString(bind.Id),
		Name:    derefString(bind.Name),
		Address: derefString(bind.Address),
		Port:    derefInt(bind.Port),
		V4V6:    derefBool(bind.V4V6),
		V6Only:  derefBool(bind.V6Only),
	}
}

// convertBindFromProto converts pb.Bind to v3.Bind
func convertBindFromProto(bind *pb.Bind) *v3.Bind {
	if bind == nil {
		return nil
	}

	return &v3.Bind{
		Id:      stringPtr(bind.Id),
		Name:    stringPtr(bind.Name),
		Address: stringPtr(bind.Address),
		Port:    intPtr(bind.Port),
		V4V6:    boolPtr(bind.V4V6),
		V6Only:  boolPtr(bind.V6Only),
	}
}

// convertTransactionToProto converts v3.Transaction to pb.Transaction
func convertTransactionToProto(transaction *v3.Transaction) *pb.Transaction {
	if transaction == nil {
		return nil
	}

	return &pb.Transaction{
		Id:     derefString(transaction.Id),
		Status: derefString(transaction.Status),
	}
}
