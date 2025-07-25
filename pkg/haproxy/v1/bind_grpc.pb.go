// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: bind.proto

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	BindService_CreateBind_FullMethodName = "/haproxy.v1.BindService/CreateBind"
	BindService_GetBind_FullMethodName    = "/haproxy.v1.BindService/GetBind"
	BindService_ListBinds_FullMethodName  = "/haproxy.v1.BindService/ListBinds"
	BindService_UpdateBind_FullMethodName = "/haproxy.v1.BindService/UpdateBind"
	BindService_DeleteBind_FullMethodName = "/haproxy.v1.BindService/DeleteBind"
)

// BindServiceClient is the client API for BindService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// BindService provides CRUD operations for HAProxy bind configuration
type BindServiceClient interface {
	CreateBind(ctx context.Context, in *CreateBindRequest, opts ...grpc.CallOption) (*CreateBindResponse, error)
	GetBind(ctx context.Context, in *GetBindRequest, opts ...grpc.CallOption) (*GetBindResponse, error)
	ListBinds(ctx context.Context, in *ListBindsRequest, opts ...grpc.CallOption) (*ListBindsResponse, error)
	UpdateBind(ctx context.Context, in *UpdateBindRequest, opts ...grpc.CallOption) (*UpdateBindResponse, error)
	DeleteBind(ctx context.Context, in *DeleteBindRequest, opts ...grpc.CallOption) (*DeleteBindResponse, error)
}

type bindServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewBindServiceClient(cc grpc.ClientConnInterface) BindServiceClient {
	return &bindServiceClient{cc}
}

func (c *bindServiceClient) CreateBind(ctx context.Context, in *CreateBindRequest, opts ...grpc.CallOption) (*CreateBindResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CreateBindResponse)
	err := c.cc.Invoke(ctx, BindService_CreateBind_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bindServiceClient) GetBind(ctx context.Context, in *GetBindRequest, opts ...grpc.CallOption) (*GetBindResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetBindResponse)
	err := c.cc.Invoke(ctx, BindService_GetBind_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bindServiceClient) ListBinds(ctx context.Context, in *ListBindsRequest, opts ...grpc.CallOption) (*ListBindsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListBindsResponse)
	err := c.cc.Invoke(ctx, BindService_ListBinds_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bindServiceClient) UpdateBind(ctx context.Context, in *UpdateBindRequest, opts ...grpc.CallOption) (*UpdateBindResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateBindResponse)
	err := c.cc.Invoke(ctx, BindService_UpdateBind_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bindServiceClient) DeleteBind(ctx context.Context, in *DeleteBindRequest, opts ...grpc.CallOption) (*DeleteBindResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteBindResponse)
	err := c.cc.Invoke(ctx, BindService_DeleteBind_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BindServiceServer is the server API for BindService service.
// All implementations must embed UnimplementedBindServiceServer
// for forward compatibility.
//
// BindService provides CRUD operations for HAProxy bind configuration
type BindServiceServer interface {
	CreateBind(context.Context, *CreateBindRequest) (*CreateBindResponse, error)
	GetBind(context.Context, *GetBindRequest) (*GetBindResponse, error)
	ListBinds(context.Context, *ListBindsRequest) (*ListBindsResponse, error)
	UpdateBind(context.Context, *UpdateBindRequest) (*UpdateBindResponse, error)
	DeleteBind(context.Context, *DeleteBindRequest) (*DeleteBindResponse, error)
	mustEmbedUnimplementedBindServiceServer()
}

// UnimplementedBindServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedBindServiceServer struct{}

func (UnimplementedBindServiceServer) CreateBind(context.Context, *CreateBindRequest) (*CreateBindResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateBind not implemented")
}
func (UnimplementedBindServiceServer) GetBind(context.Context, *GetBindRequest) (*GetBindResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBind not implemented")
}
func (UnimplementedBindServiceServer) ListBinds(context.Context, *ListBindsRequest) (*ListBindsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListBinds not implemented")
}
func (UnimplementedBindServiceServer) UpdateBind(context.Context, *UpdateBindRequest) (*UpdateBindResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateBind not implemented")
}
func (UnimplementedBindServiceServer) DeleteBind(context.Context, *DeleteBindRequest) (*DeleteBindResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteBind not implemented")
}
func (UnimplementedBindServiceServer) mustEmbedUnimplementedBindServiceServer() {}
func (UnimplementedBindServiceServer) testEmbeddedByValue()                     {}

// UnsafeBindServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BindServiceServer will
// result in compilation errors.
type UnsafeBindServiceServer interface {
	mustEmbedUnimplementedBindServiceServer()
}

func RegisterBindServiceServer(s grpc.ServiceRegistrar, srv BindServiceServer) {
	// If the following call pancis, it indicates UnimplementedBindServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&BindService_ServiceDesc, srv)
}

func _BindService_CreateBind_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateBindRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BindServiceServer).CreateBind(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BindService_CreateBind_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BindServiceServer).CreateBind(ctx, req.(*CreateBindRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BindService_GetBind_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetBindRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BindServiceServer).GetBind(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BindService_GetBind_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BindServiceServer).GetBind(ctx, req.(*GetBindRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BindService_ListBinds_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListBindsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BindServiceServer).ListBinds(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BindService_ListBinds_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BindServiceServer).ListBinds(ctx, req.(*ListBindsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BindService_UpdateBind_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateBindRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BindServiceServer).UpdateBind(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BindService_UpdateBind_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BindServiceServer).UpdateBind(ctx, req.(*UpdateBindRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BindService_DeleteBind_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteBindRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BindServiceServer).DeleteBind(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: BindService_DeleteBind_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BindServiceServer).DeleteBind(ctx, req.(*DeleteBindRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// BindService_ServiceDesc is the grpc.ServiceDesc for BindService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var BindService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "haproxy.v1.BindService",
	HandlerType: (*BindServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateBind",
			Handler:    _BindService_CreateBind_Handler,
		},
		{
			MethodName: "GetBind",
			Handler:    _BindService_GetBind_Handler,
		},
		{
			MethodName: "ListBinds",
			Handler:    _BindService_ListBinds_Handler,
		},
		{
			MethodName: "UpdateBind",
			Handler:    _BindService_UpdateBind_Handler,
		},
		{
			MethodName: "DeleteBind",
			Handler:    _BindService_DeleteBind_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "bind.proto",
}
