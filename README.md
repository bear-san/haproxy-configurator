# HAProxy Configurator

A gRPC-based HAProxy configuration management service with Protocol Buffers.

## Features

- **Unified gRPC API**: Single service interface for all HAProxy configuration operations
- **Transaction Support**: Safe configuration changes with transaction management
- **Comprehensive Coverage**: Manage backends, frontends, binds, servers, and transactions
- **Protocol Buffers**: Type-safe API definitions with Go code generation
- **Buf Integration**: Simplified protobuf build toolchain
- **Netplan Integration**: Automatic NIC IP address management synchronized with HAProxy bind configurations

## Quick Start

### Build and Run Server

```bash
# Build the server
go build -o bin/haproxy-configurator ./cmd/server

# Run the server (default port 50051, using environment variables)
./bin/haproxy-configurator

# Run on custom port
./bin/haproxy-configurator -port 8080

# Run with unified configuration file (recommended)
./bin/haproxy-configurator -config /path/to/config.yaml

# Run with legacy Netplan integration only
./bin/haproxy-configurator -netplan-config /path/to/netplan-config.yaml
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

For local testing, you can run the server directly with a local HAProxy instance that has the Data Plane API enabled.

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
├── internal/
│   ├── config/            # Configuration structures and validation
│   ├── netplan/           # Netplan integration logic
│   └── server/            # gRPC server implementation
├── cmd/server/           # Server main entry point
├── examples/             # Configuration file examples
│   └── netplan-config.yaml # Sample Netplan configuration
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
- ✅ GoReleaser CI/CD pipeline
- ✅ Comprehensive error handling and type conversion
- ✅ Netplan integration for automatic IP address management
- ✅ File-based transaction management for concurrent operations

## Configuration

The HAProxy Configurator supports two configuration methods:
1. **Unified Configuration File** (recommended): Single YAML file containing both HAProxy and Netplan settings
2. **Environment Variables + Separate Netplan Config** (legacy): Environment variables for HAProxy, separate file for Netplan

### Unified Configuration File

Create a unified configuration file (e.g., `config.yaml`):

```yaml
# HAProxy Data Plane API configuration
haproxy:
  api_url: "http://localhost:5555"
  username: "admin"  
  password: "admin"

# Netplan integration configuration (optional)
netplan:
  interface_mappings:
    - interface: "eth0"
      subnets:
        - "192.168.1.0/24"
        - "10.0.0.0/24"
    - interface: "vlan100@eth0"
      subnets:
        - "10.100.0.0/24"
  netplan_config_path: "/etc/netplan/99-haproxy-configurator.yaml"
  backup_enabled: true
```

Run the server:
```bash
./bin/haproxy-configurator -config /path/to/config.yaml
```

### Environment Variables Configuration

The following environment variables are used when no configuration file is provided:

- `HAPROXY_API_URL`: HAProxy Data Plane API URL (default: `http://localhost:5555`)
- `HAPROXY_API_USERNAME`: API username (default: `admin`)
- `HAPROXY_API_PASSWORD`: API password (default: `admin`)

## Netplan Integration

The HAProxy Configurator can automatically manage network interface IP addresses using Ubuntu's Netplan, ensuring that IP addresses are properly configured on network interfaces before HAProxy bind configurations are created.

### Features

- **Automatic IP Assignment**: When creating HAProxy bind configurations, IP addresses are automatically added to the appropriate network interfaces
- **Automatic Cleanup**: When deleting bind configurations, IP addresses are removed from network interfaces
- **Transaction-based Apply**: Netplan changes are only applied when HAProxy transactions are committed
- **Subnet-based Interface Mapping**: Configure which network interface should be used for different IP subnets
- **VLAN Support**: Full support for VLAN interfaces using the `vlan_name@parent_interface` format
- **Intelligent Subnet Mask Detection**: Automatically determines the correct subnet mask based on the configured subnet mappings (e.g., IP 192.168.1.100 in subnet 192.168.1.0/24 will be assigned as 192.168.1.100/24)
- **Backup Support**: Automatically backup existing Netplan configurations before making changes
- **Rollback Support**: If HAProxy configuration fails, Netplan changes are automatically rolled back

### Configuration

Create a Netplan configuration file (e.g., `netplan-config.yaml`):

```yaml
netplan:
  interface_mappings:
    - interface: "eth0"
      subnets:
        - "192.168.1.0/24"
        - "10.0.0.0/24"
    - interface: "eth1"
      subnets:
        - "172.16.0.0/16"
    # VLAN interface example
    - interface: "vlan100@eth0"
      subnets:
        - "10.100.0.0/24"
  netplan_config_path: "/etc/netplan/99-haproxy.yaml"
  backup_enabled: true
```

### Configuration Options

- `interface_mappings`: List of interface to subnet mappings
  - `interface`: Network interface name (e.g., "eth0", "ens3", "vlan2@eth0")
    - For VLAN interfaces, use the format `vlan_name@parent_interface` (e.g., "vlan2@eth0")
  - `subnets`: List of CIDR subnets that should be assigned to this interface
- `netplan_config_path`: Path where Netplan configuration will be written
- `backup_enabled`: Whether to create backup files before modifying Netplan configuration

### Usage

1. Create your Netplan configuration file
2. Start the server with Netplan integration:
   ```bash
   ./bin/haproxy-configurator -netplan-config /path/to/netplan-config.yaml
   ```
3. Create HAProxy bind configurations as usual - IP addresses will be automatically managed

### How it Works

1. **Bind Creation**: When a bind is created, the system:
   - Determines which network interface should host the IP address based on subnet mappings
   - For VLAN interfaces (e.g., `vlan100@eth0`), creates/updates the VLAN section in Netplan
   - For regular interfaces, updates the ethernets section in Netplan
   - Adds the IP address to the Netplan configuration for that interface
   - Generates the Netplan configuration (but doesn't apply it yet)
   - Creates the HAProxy bind configuration
   - If HAProxy creation fails, rolls back the Netplan changes

2. **Transaction Commit**: When a transaction is committed:
   - Commits the HAProxy transaction first
   - If successful, applies the Netplan configuration with `netplan apply`

3. **Bind Deletion**: When a bind is deleted:
   - Deletes the HAProxy bind configuration first
   - If successful, removes the IP address from the Netplan configuration
   - Note: Netplan apply only happens during transaction commits

### Example Workflow

```bash
# 1. Create a transaction
grpcurl -plaintext -d '{"version": 1}' localhost:50051 haproxy.v1.HAProxyManagerService/CreateTransaction

# 2. Create a frontend
grpcurl -plaintext -d '{"frontend": {"name": "test-frontend"}, "transaction_id": "transaction-id"}' localhost:50051 haproxy.v1.HAProxyManagerService/CreateFrontend

# 3. Create a bind (IP address automatically added to Netplan)
grpcurl -plaintext -d '{"frontend_name": "test-frontend", "bind": {"name": "test-bind", "address": "192.168.1.100", "port": 80}, "transaction_id": "transaction-id"}' localhost:50051 haproxy.v1.HAProxyManagerService/CreateBind

# 4. Commit transaction (Netplan apply executed here)
grpcurl -plaintext -d '{"transaction_id": "transaction-id"}' localhost:50051 haproxy.v1.HAProxyManagerService/CommitTransaction
```

### Security Considerations

- The service must run with appropriate permissions to:
  - Read and write Netplan configuration files
  - Execute `netplan generate` and `netplan apply` commands
- Consider running the service as a dedicated user with minimal required permissions
- Netplan configuration files should be properly secured

### Troubleshooting

- Check server logs for detailed information about Netplan operations
- Verify that the specified network interfaces exist on the system
- Ensure proper permissions for Netplan configuration files and commands
- Use `netplan try` to test configurations manually if needed

## Release

Releases are automated via GitHub Actions:
- Tag a version: `git tag v1.0.0 && git push origin v1.0.0`
- Automatically builds binaries for multiple architectures
- Published to GitHub Releases
