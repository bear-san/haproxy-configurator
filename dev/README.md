# Development Environment

This directory contains files for local development and testing.

## Files

- **Dockerfile.dev**: Development Dockerfile that builds from source
- **docker-compose.yml**: Local development environment with HAProxy
- **haproxy.cfg**: HAProxy configuration for testing

## Usage

### Start Development Environment

```bash
# From the dev directory
cd dev
docker-compose up --build

# Or from the project root
docker-compose -f dev/docker-compose.yml up --build
```

### Access Services

- **gRPC Server**: `localhost:50051`
- **HAProxy Stats**: `http://localhost:8404/stats`
- **HAProxy Data Plane API**: `http://localhost:5555`

### Environment Variables

The development environment uses these default values:
- `HAPROXY_API_URL=http://haproxy:5555`
- `HAPROXY_API_USERNAME=admin`
- `HAPROXY_API_PASSWORD=admin`

### Testing with grpcurl

```bash
# List available services
grpcurl -plaintext localhost:50051 list

# Get version
grpcurl -plaintext localhost:50051 haproxy.v1.HAProxyManagerService/GetVersion

# Create a transaction
grpcurl -plaintext -d '{"version": 1}' localhost:50051 haproxy.v1.HAProxyManagerService/CreateTransaction
```

## Development Workflow

1. Make changes to the source code
2. Rebuild and restart: `docker-compose up --build`
3. Test your changes using grpcurl or your preferred gRPC client