# HAProxy Configurator

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
go build -o bin/haproxy-configurator ./cmd/server

# Run the server (default port 50051)
./bin/haproxy-configurator

# Run on custom port
./bin/haproxy-configurator -port 8080
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

### Local Development Environment

Use the development environment in the `dev/` directory for local testing:

```bash
# Start development environment with HAProxy
cd dev
docker-compose up --build

# Or from project root
docker-compose -f dev/docker-compose.yml up --build
```

This provides:
- HAProxy Configurator gRPC server on `localhost:50051`
- HAProxy with Data Plane API on `localhost:5555`
- HAProxy stats page on `http://localhost:8404/stats`

See [dev/README.md](dev/README.md) for more details.

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
├── dev/                  # Development environment
│   ├── Dockerfile.dev    # Development Dockerfile
│   ├── docker-compose.yml # Local development setup
│   ├── haproxy.cfg       # HAProxy test configuration
│   └── README.md         # Development environment guide
├── Dockerfile            # Production Dockerfile (for GoReleaser)
├── .goreleaser.yml       # GoReleaser configuration
├── buf.yaml              # Buf configuration
└── buf.gen.yaml          # Buf code generation config
```

## Current Status

This implementation provides:
- ✅ Complete proto definitions with enums for type safety
- ✅ Generated Go protobuf code
- ✅ Full gRPC server implementation with HAProxy Data Plane API integration
- ✅ Transaction-based configuration management
- ✅ Docker multi-architecture builds (AMD64/ARM64)
- ✅ GoReleaser CI/CD pipeline
- ✅ Comprehensive error handling and type conversion
- ✅ Development environment with docker-compose

## Environment Variables

- `HAPROXY_API_URL`: HAProxy Data Plane API URL (default: `http://localhost:5555`)
- `HAPROXY_API_USERNAME`: API username (default: `admin`)
- `HAPROXY_API_PASSWORD`: API password (default: `admin`)

## Release

Releases are automated via GitHub Actions:
- Tag a version: `git tag v1.0.0 && git push origin v1.0.0`
- Automatically builds binaries and Docker images for multiple architectures
- Published to GitHub Releases and Docker registries
