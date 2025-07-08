package server

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/bear-san/haproxy-configurator/internal/config"
	"github.com/bear-san/haproxy-configurator/internal/logger"
	"github.com/bear-san/haproxy-configurator/internal/netplan"
	pb "github.com/bear-san/haproxy-configurator/pkg/haproxy/v1"
	v3 "github.com/bear-san/haproxy-go/dataplane/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HAProxyManagerServer implements the HAProxyManagerServiceServer interface
type HAProxyManagerServer struct {
	pb.UnimplementedHAProxyManagerServiceServer
	client     v3.Client
	netplanMgr *netplan.Manager
	config     *config.Config
}


// NewHAProxyManagerServerWithConfig creates a new HAProxyManagerServer instance using a configuration file
func NewHAProxyManagerServerWithConfig(cfg *config.Config) *HAProxyManagerServer {
	logger.GetLogger().Info("Initializing HAProxy manager server with config",
		zap.String("base_url", cfg.HAProxy.APIURL),
		zap.String("username", cfg.HAProxy.Username))

	// Create base64 encoded credentials
	credential := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cfg.HAProxy.Username, cfg.HAProxy.Password)))

	server := &HAProxyManagerServer{
		client: v3.Client{
			BaseUrl:    cfg.HAProxy.APIURL,
			Credential: credential,
		},
		config: cfg,
	}

	// Initialize Netplan if configured
	if cfg.HasNetplanIntegration() {
		server.netplanMgr = netplan.NewManagerWithConfig(cfg)

		logger.GetLogger().Info("Netplan integration enabled via config file",
			zap.String("config_path", cfg.Netplan.ConfigPath))
	}

	return server
}

// GetVersion retrieves the current HAProxy configuration version from the HAProxy Data Plane API
func (s *HAProxyManagerServer) GetVersion(_ context.Context, _ *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	version, err := s.client.GetVersion()
	if err != nil {
		return nil, handleHAProxyError(err)
	}
	return &pb.GetVersionResponse{
		Version: derefInt(version),
	}, nil
}

// CreateTransaction creates a new configuration transaction in HAProxy
// The transaction must be committed or closed after making configuration changes
func (s *HAProxyManagerServer) CreateTransaction(_ context.Context, req *pb.CreateTransactionRequest) (*pb.CreateTransactionResponse, error) {
	transaction, err := s.client.CreateTransaction(int(req.Version))
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.CreateTransactionResponse{
		Transaction: convertTransactionToProto(transaction),
	}, nil
}

// GetTransaction retrieves the details of a specific transaction by its ID
func (s *HAProxyManagerServer) GetTransaction(_ context.Context, req *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "transaction ID is required")
	}

	transaction, err := s.client.GetTransaction(req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.GetTransactionResponse{
		Transaction: convertTransactionToProto(transaction),
	}, nil
}

// CommitTransaction commits a transaction, applying all configuration changes to HAProxy
func (s *HAProxyManagerServer) CommitTransaction(_ context.Context, req *pb.CommitTransactionRequest) (*pb.CommitTransactionResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "transaction ID is required")
	}

	// Use Netplan-aware transaction commit
	return s.CommitTransactionWithNetplan(req)
}

// CloseTransaction closes a transaction without committing any changes
func (s *HAProxyManagerServer) CloseTransaction(_ context.Context, req *pb.CloseTransactionRequest) (*pb.CloseTransactionResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "transaction ID is required")
	}

	message, err := s.client.CloseTransaction(req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.CloseTransactionResponse{
		Message: derefString(message),
	}, nil
}

// CreateBackend creates a new backend configuration in HAProxy
// A backend defines a set of servers to which the proxy will connect to forward incoming requests
func (s *HAProxyManagerServer) CreateBackend(_ context.Context, req *pb.CreateBackendRequest) (*pb.CreateBackendResponse, error) {
	if req.Backend == nil {
		return nil, status.Errorf(codes.InvalidArgument, "backend is required")
	}

	backend := convertBackendFromProto(req.Backend)
	created, err := s.client.AddBackend(*backend, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.CreateBackendResponse{
		Backend: convertBackendToProto(created),
	}, nil
}

// GetBackend retrieves a specific backend configuration by name
func (s *HAProxyManagerServer) GetBackend(_ context.Context, req *pb.GetBackendRequest) (*pb.GetBackendResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}

	backend, err := s.client.GetBackend(req.Name, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.GetBackendResponse{
		Backend: convertBackendToProto(backend),
	}, nil
}

// ListBackends retrieves all backend configurations from HAProxy
func (s *HAProxyManagerServer) ListBackends(_ context.Context, req *pb.ListBackendsRequest) (*pb.ListBackendsResponse, error) {
	backends, err := s.client.ListBackends(req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	var pbBackends []*pb.Backend
	for _, backend := range backends {
		pbBackends = append(pbBackends, convertBackendToProto(&backend))
	}

	return &pb.ListBackendsResponse{
		Backends: pbBackends,
	}, nil
}

// UpdateBackend updates an existing backend configuration
func (s *HAProxyManagerServer) UpdateBackend(_ context.Context, req *pb.UpdateBackendRequest) (*pb.UpdateBackendResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}
	if req.Backend == nil {
		return nil, status.Errorf(codes.InvalidArgument, "backend is required")
	}

	backend := convertBackendFromProto(req.Backend)
	updated, err := s.client.ReplaceBackend(req.Name, *backend, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.UpdateBackendResponse{
		Backend: convertBackendToProto(updated),
	}, nil
}

// DeleteBackend removes a backend configuration from HAProxy
func (s *HAProxyManagerServer) DeleteBackend(_ context.Context, req *pb.DeleteBackendRequest) (*pb.DeleteBackendResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}

	err := s.client.DeleteBackend(req.Name, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.DeleteBackendResponse{}, nil
}

// CreateFrontend creates a new frontend configuration in HAProxy
// A frontend defines how requests should be received and which backend to route them to
func (s *HAProxyManagerServer) CreateFrontend(_ context.Context, req *pb.CreateFrontendRequest) (*pb.CreateFrontendResponse, error) {
	if req.Frontend == nil {
		return nil, status.Errorf(codes.InvalidArgument, "frontend is required")
	}

	frontend := convertFrontendFromProto(req.Frontend)
	created, err := s.client.AddFrontend(*frontend, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.CreateFrontendResponse{
		Frontend: convertFrontendToProto(created),
	}, nil
}

// GetFrontend retrieves a specific frontend configuration by name
func (s *HAProxyManagerServer) GetFrontend(_ context.Context, req *pb.GetFrontendRequest) (*pb.GetFrontendResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}

	frontend, err := s.client.GetFrontend(req.Name, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.GetFrontendResponse{
		Frontend: convertFrontendToProto(frontend),
	}, nil
}

// ListFrontends retrieves all frontend configurations from HAProxy
func (s *HAProxyManagerServer) ListFrontends(_ context.Context, req *pb.ListFrontendsRequest) (*pb.ListFrontendsResponse, error) {
	frontends, err := s.client.ListFrontends(req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	var pbFrontends []*pb.Frontend
	for _, frontend := range frontends {
		pbFrontends = append(pbFrontends, convertFrontendToProto(&frontend))
	}

	return &pb.ListFrontendsResponse{
		Frontends: pbFrontends,
	}, nil
}

// UpdateFrontend updates an existing frontend configuration
func (s *HAProxyManagerServer) UpdateFrontend(_ context.Context, req *pb.UpdateFrontendRequest) (*pb.UpdateFrontendResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}
	if req.Frontend == nil {
		return nil, status.Errorf(codes.InvalidArgument, "frontend is required")
	}

	frontend := convertFrontendFromProto(req.Frontend)
	updated, err := s.client.ReplaceFrontend(req.Name, *frontend, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.UpdateFrontendResponse{
		Frontend: convertFrontendToProto(updated),
	}, nil
}

// DeleteFrontend removes a frontend configuration from HAProxy
func (s *HAProxyManagerServer) DeleteFrontend(_ context.Context, req *pb.DeleteFrontendRequest) (*pb.DeleteFrontendResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}

	err := s.client.DeleteFrontend(req.Name, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.DeleteFrontendResponse{}, nil
}

// CreateBind creates a new bind configuration for a frontend in HAProxy
// A bind defines the listening address and port for a frontend
func (s *HAProxyManagerServer) CreateBind(_ context.Context, req *pb.CreateBindRequest) (*pb.CreateBindResponse, error) {
	if req.FrontendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}
	if req.Bind == nil {
		return nil, status.Errorf(codes.InvalidArgument, "bind is required")
	}

	// Use Netplan-aware bind creation
	return s.CreateBindWithNetplan(req)
}

// GetBind retrieves a specific bind configuration by name from a frontend
func (s *HAProxyManagerServer) GetBind(_ context.Context, req *pb.GetBindRequest) (*pb.GetBindResponse, error) {
	if req.FrontendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "bind name is required")
	}

	bind, err := s.client.GetBind(req.Name, req.FrontendName, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.GetBindResponse{
		Bind: convertBindToProto(bind),
	}, nil
}

// ListBinds retrieves all bind configurations for a specific frontend
func (s *HAProxyManagerServer) ListBinds(_ context.Context, req *pb.ListBindsRequest) (*pb.ListBindsResponse, error) {
	if req.FrontendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}

	binds, err := s.client.ListBinds(req.FrontendName, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	var pbBinds []*pb.Bind
	for _, bind := range binds {
		pbBinds = append(pbBinds, convertBindToProto(&bind))
	}

	return &pb.ListBindsResponse{
		Binds: pbBinds,
	}, nil
}

// UpdateBind updates an existing bind configuration for a frontend
func (s *HAProxyManagerServer) UpdateBind(_ context.Context, req *pb.UpdateBindRequest) (*pb.UpdateBindResponse, error) {
	if req.FrontendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}
	if req.Bind == nil {
		return nil, status.Errorf(codes.InvalidArgument, "bind is required")
	}

	bind := convertBindFromProto(req.Bind)
	updated, err := s.client.ReplaceBind(req.FrontendName, req.TransactionId, *bind)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.UpdateBindResponse{
		Bind: convertBindToProto(updated),
	}, nil
}

// DeleteBind removes a bind configuration from a frontend
func (s *HAProxyManagerServer) DeleteBind(_ context.Context, req *pb.DeleteBindRequest) (*pb.DeleteBindResponse, error) {
	if req.FrontendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "bind name is required")
	}

	// Use Netplan-aware bind deletion
	return s.DeleteBindWithNetplan(req)
}

// CreateServer creates a new server configuration in a backend
// A server represents a backend server that will handle forwarded requests
func (s *HAProxyManagerServer) CreateServer(_ context.Context, req *pb.CreateServerRequest) (*pb.CreateServerResponse, error) {
	if req.BackendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}
	if req.Server == nil {
		return nil, status.Errorf(codes.InvalidArgument, "server is required")
	}

	server := convertServerFromProto(req.Server)
	created, err := s.client.AddServer(req.BackendName, req.TransactionId, *server)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.CreateServerResponse{
		Server: convertServerToProto(created),
	}, nil
}

// GetServer retrieves a specific server configuration by name from a backend
func (s *HAProxyManagerServer) GetServer(_ context.Context, req *pb.GetServerRequest) (*pb.GetServerResponse, error) {
	if req.BackendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "server name is required")
	}

	server, err := s.client.GetServer(req.Name, req.BackendName, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.GetServerResponse{
		Server: convertServerToProto(server),
	}, nil
}

// ListServers retrieves all server configurations for a specific backend
func (s *HAProxyManagerServer) ListServers(_ context.Context, req *pb.ListServersRequest) (*pb.ListServersResponse, error) {
	if req.BackendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}

	servers, err := s.client.ListServers(req.BackendName, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	var pbServers []*pb.Server
	for _, server := range servers {
		pbServers = append(pbServers, convertServerToProto(&server))
	}

	return &pb.ListServersResponse{
		Servers: pbServers,
	}, nil
}

// UpdateServer updates an existing server configuration in a backend
func (s *HAProxyManagerServer) UpdateServer(_ context.Context, req *pb.UpdateServerRequest) (*pb.UpdateServerResponse, error) {
	if req.BackendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "server name is required")
	}
	if req.Server == nil {
		return nil, status.Errorf(codes.InvalidArgument, "server is required")
	}

	server := convertServerFromProto(req.Server)
	updated, err := s.client.ReplaceServer(req.BackendName, req.TransactionId, *server)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.UpdateServerResponse{
		Server: convertServerToProto(updated),
	}, nil
}

// DeleteServer removes a server configuration from a backend
func (s *HAProxyManagerServer) DeleteServer(_ context.Context, req *pb.DeleteServerRequest) (*pb.DeleteServerResponse, error) {
	if req.BackendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "server name is required")
	}

	err := s.client.DeleteServer(req.Name, req.BackendName, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.DeleteServerResponse{}, nil
}
