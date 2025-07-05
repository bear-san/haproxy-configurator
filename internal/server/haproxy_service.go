package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	v3 "github.com/bear-san/haproxy-go/dataplane/v3"
	pb "github.com/bear-san/haproxy-network-manager/pkg/haproxy/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HAProxyManagerServer implements the HAProxyManagerServiceServer interface
type HAProxyManagerServer struct {
	pb.UnimplementedHAProxyManagerServiceServer
	client v3.Client
}

// NewHAProxyManagerServer creates a new HAProxyManagerServer instance
func NewHAProxyManagerServer() *HAProxyManagerServer {
	// Get configuration from environment variables
	baseURL := os.Getenv("HAPROXY_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5555"
	}

	username := os.Getenv("HAPROXY_API_USERNAME")
	if username == "" {
		username = "admin"
	}

	password := os.Getenv("HAPROXY_API_PASSWORD")
	if password == "" {
		password = "admin"
	}

	// Create base64 encoded credentials
	credential := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))

	return &HAProxyManagerServer{
		client: v3.Client{
			BaseUrl:    baseURL,
			Credential: credential,
		},
	}
}

// Transaction operations
func (s *HAProxyManagerServer) GetVersion(ctx context.Context, req *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	version, err := s.client.GetVersion()
	if err != nil {
		return nil, handleHAProxyError(err)
	}
	return &pb.GetVersionResponse{
		Version: derefInt(version),
	}, nil
}

func (s *HAProxyManagerServer) CreateTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.CreateTransactionResponse, error) {
	transaction, err := s.client.CreateTransaction(int(req.Version))
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.CreateTransactionResponse{
		Transaction: convertTransactionToProto(transaction),
	}, nil
}

func (s *HAProxyManagerServer) GetTransaction(ctx context.Context, req *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
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

func (s *HAProxyManagerServer) CommitTransaction(ctx context.Context, req *pb.CommitTransactionRequest) (*pb.CommitTransactionResponse, error) {
	if req.TransactionId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "transaction ID is required")
	}

	transaction, err := s.client.CommitTransaction(req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.CommitTransactionResponse{
		Transaction: convertTransactionToProto(transaction),
	}, nil
}

func (s *HAProxyManagerServer) CloseTransaction(ctx context.Context, req *pb.CloseTransactionRequest) (*pb.CloseTransactionResponse, error) {
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

// Backend operations
func (s *HAProxyManagerServer) CreateBackend(ctx context.Context, req *pb.CreateBackendRequest) (*pb.CreateBackendResponse, error) {
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

func (s *HAProxyManagerServer) GetBackend(ctx context.Context, req *pb.GetBackendRequest) (*pb.GetBackendResponse, error) {
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

func (s *HAProxyManagerServer) ListBackends(ctx context.Context, req *pb.ListBackendsRequest) (*pb.ListBackendsResponse, error) {
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

func (s *HAProxyManagerServer) UpdateBackend(ctx context.Context, req *pb.UpdateBackendRequest) (*pb.UpdateBackendResponse, error) {
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

func (s *HAProxyManagerServer) DeleteBackend(ctx context.Context, req *pb.DeleteBackendRequest) (*pb.DeleteBackendResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "backend name is required")
	}

	err := s.client.DeleteBackend(req.Name, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.DeleteBackendResponse{}, nil
}

// Frontend operations
func (s *HAProxyManagerServer) CreateFrontend(ctx context.Context, req *pb.CreateFrontendRequest) (*pb.CreateFrontendResponse, error) {
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

func (s *HAProxyManagerServer) GetFrontend(ctx context.Context, req *pb.GetFrontendRequest) (*pb.GetFrontendResponse, error) {
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

func (s *HAProxyManagerServer) ListFrontends(ctx context.Context, req *pb.ListFrontendsRequest) (*pb.ListFrontendsResponse, error) {
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

func (s *HAProxyManagerServer) UpdateFrontend(ctx context.Context, req *pb.UpdateFrontendRequest) (*pb.UpdateFrontendResponse, error) {
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

func (s *HAProxyManagerServer) DeleteFrontend(ctx context.Context, req *pb.DeleteFrontendRequest) (*pb.DeleteFrontendResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}

	err := s.client.DeleteFrontend(req.Name, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.DeleteFrontendResponse{}, nil
}

// Bind operations
func (s *HAProxyManagerServer) CreateBind(ctx context.Context, req *pb.CreateBindRequest) (*pb.CreateBindResponse, error) {
	if req.FrontendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}
	if req.Bind == nil {
		return nil, status.Errorf(codes.InvalidArgument, "bind is required")
	}

	bind := convertBindFromProto(req.Bind)
	created, err := s.client.AddBind(req.FrontendName, req.TransactionId, *bind)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.CreateBindResponse{
		Bind: convertBindToProto(created),
	}, nil
}

func (s *HAProxyManagerServer) GetBind(ctx context.Context, req *pb.GetBindRequest) (*pb.GetBindResponse, error) {
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

func (s *HAProxyManagerServer) ListBinds(ctx context.Context, req *pb.ListBindsRequest) (*pb.ListBindsResponse, error) {
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

func (s *HAProxyManagerServer) UpdateBind(ctx context.Context, req *pb.UpdateBindRequest) (*pb.UpdateBindResponse, error) {
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

func (s *HAProxyManagerServer) DeleteBind(ctx context.Context, req *pb.DeleteBindRequest) (*pb.DeleteBindResponse, error) {
	if req.FrontendName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "frontend name is required")
	}
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "bind name is required")
	}

	err := s.client.DeleteBind(req.Name, req.FrontendName, req.TransactionId)
	if err != nil {
		return nil, handleHAProxyError(err)
	}

	return &pb.DeleteBindResponse{}, nil
}

// Server operations
func (s *HAProxyManagerServer) CreateServer(ctx context.Context, req *pb.CreateServerRequest) (*pb.CreateServerResponse, error) {
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

func (s *HAProxyManagerServer) GetServer(ctx context.Context, req *pb.GetServerRequest) (*pb.GetServerResponse, error) {
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

func (s *HAProxyManagerServer) ListServers(ctx context.Context, req *pb.ListServersRequest) (*pb.ListServersResponse, error) {
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

func (s *HAProxyManagerServer) UpdateServer(ctx context.Context, req *pb.UpdateServerRequest) (*pb.UpdateServerResponse, error) {
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

func (s *HAProxyManagerServer) DeleteServer(ctx context.Context, req *pb.DeleteServerRequest) (*pb.DeleteServerResponse, error) {
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
