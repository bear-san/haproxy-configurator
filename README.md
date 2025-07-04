# HAProxy Network Manager

A gRPC-based HAProxy configuration management service with Protocol Buffers.

## Features

- **Unified gRPC API**: Single service interface for all HAProxy configuration operations
- **Transaction Support**: Safe configuration changes with transaction management
- **Comprehensive Coverage**: Manage backends, frontends, binds, servers, and transactions
- **Protocol Buffers**: Type-safe API definitions with Go code generation
- **Buf Integration**: Simplified protobuf build toolchain

## Quick Start

### Build and Run Server

```bash
# Build the server
go build -o bin/haproxy-server ./cmd/server

# Run the server (default port 50051)
./bin/haproxy-server

# Run on custom port
./bin/haproxy-server -port 8080
```

### Protocol Buffer Generation

```bash
# Generate Go code from proto definitions
buf generate

# Lint proto files
buf lint
```

## API Overview

The service provides a unified `HAProxyManagerService` with operations for:

- **Transaction Management**: Version, create, get, commit, close transactions
- **Backend Operations**: CRUD operations for HAProxy backends
- **Frontend Operations**: CRUD operations for HAProxy frontends  
- **Bind Operations**: CRUD operations for frontend binds
- **Server Operations**: CRUD operations for backend servers

## Development

### Testing with grpcurl

```bash
# List available services
grpcurl -plaintext localhost:50051 list

# List service methods
grpcurl -plaintext localhost:50051 list haproxy.v1.HAProxyManagerService

# Test GetVersion (example)
grpcurl -plaintext localhost:50051 haproxy.v1.HAProxyManagerService/GetVersion
```

### Project Structure

```
├── proto/                  # Protocol Buffer definitions
├── pkg/haproxy/v1/        # Generated Go protobuf code
├── internal/server/       # gRPC server implementation
├── cmd/server/           # Server main entry point
├── buf.yaml              # Buf configuration
└── buf.gen.yaml          # Buf code generation config
```

## Current Status

This is a foundational implementation with:
- ✅ Complete proto definitions
- ✅ Generated Go protobuf code
- ✅ gRPC server scaffold with all method stubs
- ⚠️ Methods return "Unimplemented" - ready for HAProxy integration

Next steps: Implement actual HAProxy configuration management logic in the service methods.