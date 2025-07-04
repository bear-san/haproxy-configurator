package server

import (
	"context"

	pb "github.com/bear-san/haproxy-network-manager/pkg/haproxy/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HAProxyManagerServer implements the HAProxyManagerServiceServer interface
type HAProxyManagerServer struct {
	pb.UnimplementedHAProxyManagerServiceServer
}

// NewHAProxyManagerServer creates a new HAProxyManagerServer instance
func NewHAProxyManagerServer() *HAProxyManagerServer {
	return &HAProxyManagerServer{}
}

// Transaction operations
func (s *HAProxyManagerServer) GetVersion(ctx context.Context, req *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetVersion not implemented")
}

func (s *HAProxyManagerServer) CreateTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.CreateTransactionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateTransaction not implemented")
}

func (s *HAProxyManagerServer) GetTransaction(ctx context.Context, req *pb.GetTransactionRequest) (*pb.GetTransactionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTransaction not implemented")
}

func (s *HAProxyManagerServer) CommitTransaction(ctx context.Context, req *pb.CommitTransactionRequest) (*pb.CommitTransactionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CommitTransaction not implemented")
}

func (s *HAProxyManagerServer) CloseTransaction(ctx context.Context, req *pb.CloseTransactionRequest) (*pb.CloseTransactionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CloseTransaction not implemented")
}

// Backend operations
func (s *HAProxyManagerServer) CreateBackend(ctx context.Context, req *pb.CreateBackendRequest) (*pb.CreateBackendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateBackend not implemented")
}

func (s *HAProxyManagerServer) GetBackend(ctx context.Context, req *pb.GetBackendRequest) (*pb.GetBackendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBackend not implemented")
}

func (s *HAProxyManagerServer) ListBackends(ctx context.Context, req *pb.ListBackendsRequest) (*pb.ListBackendsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListBackends not implemented")
}

func (s *HAProxyManagerServer) UpdateBackend(ctx context.Context, req *pb.UpdateBackendRequest) (*pb.UpdateBackendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateBackend not implemented")
}

func (s *HAProxyManagerServer) DeleteBackend(ctx context.Context, req *pb.DeleteBackendRequest) (*pb.DeleteBackendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteBackend not implemented")
}

// Frontend operations
func (s *HAProxyManagerServer) CreateFrontend(ctx context.Context, req *pb.CreateFrontendRequest) (*pb.CreateFrontendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateFrontend not implemented")
}

func (s *HAProxyManagerServer) GetFrontend(ctx context.Context, req *pb.GetFrontendRequest) (*pb.GetFrontendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetFrontend not implemented")
}

func (s *HAProxyManagerServer) ListFrontends(ctx context.Context, req *pb.ListFrontendsRequest) (*pb.ListFrontendsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListFrontends not implemented")
}

func (s *HAProxyManagerServer) UpdateFrontend(ctx context.Context, req *pb.UpdateFrontendRequest) (*pb.UpdateFrontendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateFrontend not implemented")
}

func (s *HAProxyManagerServer) DeleteFrontend(ctx context.Context, req *pb.DeleteFrontendRequest) (*pb.DeleteFrontendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteFrontend not implemented")
}

// Bind operations
func (s *HAProxyManagerServer) CreateBind(ctx context.Context, req *pb.CreateBindRequest) (*pb.CreateBindResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateBind not implemented")
}

func (s *HAProxyManagerServer) GetBind(ctx context.Context, req *pb.GetBindRequest) (*pb.GetBindResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBind not implemented")
}

func (s *HAProxyManagerServer) ListBinds(ctx context.Context, req *pb.ListBindsRequest) (*pb.ListBindsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListBinds not implemented")
}

func (s *HAProxyManagerServer) UpdateBind(ctx context.Context, req *pb.UpdateBindRequest) (*pb.UpdateBindResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateBind not implemented")
}

func (s *HAProxyManagerServer) DeleteBind(ctx context.Context, req *pb.DeleteBindRequest) (*pb.DeleteBindResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteBind not implemented")
}

// Server operations
func (s *HAProxyManagerServer) CreateServer(ctx context.Context, req *pb.CreateServerRequest) (*pb.CreateServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateServer not implemented")
}

func (s *HAProxyManagerServer) GetServer(ctx context.Context, req *pb.GetServerRequest) (*pb.GetServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServer not implemented")
}

func (s *HAProxyManagerServer) ListServers(ctx context.Context, req *pb.ListServersRequest) (*pb.ListServersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListServers not implemented")
}

func (s *HAProxyManagerServer) UpdateServer(ctx context.Context, req *pb.UpdateServerRequest) (*pb.UpdateServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateServer not implemented")
}

func (s *HAProxyManagerServer) DeleteServer(ctx context.Context, req *pb.DeleteServerRequest) (*pb.DeleteServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteServer not implemented")
}