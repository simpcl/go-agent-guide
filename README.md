# Agent Guide

An agent gateway service that integrates with X402 payment facilitator for payment verification and settlement. This gateway provides reverse proxy capabilities, resource management, authentication, and monitoring.

## Features

- ✅ **Dual Server Architecture** - Separate gateway and admin servers for better isolation
- ✅ **Resource Gateway** - Reverse proxy with payment integration
- ✅ **Resource Management** - YAML-based resource configuration with dynamic reloading
- ✅ **Payment Integration** - Automatic X402 payment verification and settlement (buyer and seller modes)
- ✅ **Authentication** - Multi-layer authentication (resource-level and admin-level)
- ✅ **Monitoring** - Prometheus metrics and structured logging
- ✅ **CORS Support** - Cross-origin resource sharing enabled by default
- ✅ **Graceful Shutdown** - Clean shutdown with configurable timeout
- ✅ **Request Tracing** - Request ID middleware for distributed tracing

## Architecture

```
┌─────────────────────┐         ┌─────────────────────┐
│   Gateway Server    │         │   Admin Server      │
│   (Port 8080)       │         │   (Port 8081)        │
│                     │         │                     │
│  /discover/*        │         │  /health            │
│  /api/{resource}    │         │  /ready             │
│                     │         │  /metrics           │
└──────────┬──────────┘         └──────────┬──────────┘
           │                               │
           └───────────────┬───────────────┘
                           │
           ┌───────────────▼───────────────┐
           │   Resource Gateway            │
           │   - Resource management       │
           │   - Payment processing        │
           │   - Reverse proxy             │
           └───────────────┬───────────────┘
                           │
           ┌───────────────▼───────────────┐
           │   X402 Facilitator            │
           │   (Library)                   │
           │   - Verify                   │
           │   - Settle                   │
           └───────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.23 or later
- X402 Facilitator library (github.com/agent-guide/go-x402-facilitator)

### Installation

1. **Install dependencies:**
   ```bash
   go mod download
   go mod tidy
   ```

2. **Build and install:**
   ```bash
   # Option 1: Install to $GOPATH/bin
   go install ./cmd
   
   # Option 2: Build binary file
   go build -o agent-guide ./cmd
   ```

3. **Configure:**
   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your settings
   # Resources are configured in the endpoints section of config.yaml
   ```

4. **Set environment variables (optional):**
   ```bash
   # Facilitator private key (required for payment settlement)
   export AGENTGUIDE_FACILITATOR_PRIVATE_KEY="your-private-key"
   
   # Admin server authentication tokens (comma-separated)
   export AGENTGUIDE_ADMIN_SERVER_AUTH_TOKENS="token1,token2,token3"
   ```
   
   **Note:** All configuration values can be overridden via environment variables using the `AGENTGUIDE_` prefix. Use underscores instead of dots for nested keys (e.g., `AGENTGUIDE_GATEWAY_SERVER_PORT=9090`).

5. **Run:**
   ```bash
   # If using go install
   agent-guide -config config.yaml
   
   # If using go build
   ./agent-guide -config config.yaml
   
   # Or run directly
   go run ./cmd -config config.yaml
   
   # Show version
   agent-guide -version
   ```

## Configuration

The gateway can be configured via:

1. **Configuration file** (`config.yaml`) - See `config.example.yaml` for reference
2. **Environment variables** (prefixed with `AGENTGUIDE_`)

Environment variables take precedence over configuration file values. Use underscores instead of dots for nested configuration keys. For example:
- `AGENTGUIDE_GATEWAY_SERVER_PORT=9090` overrides `gateway_server.port`
- `AGENTGUIDE_ADMIN_SERVER_LOG_LEVEL=debug` overrides `admin_server.log_level`
- `AGENTGUIDE_FACILITATOR_PRIVATE_KEY=...` overrides `facilitator.private_key`

### Configuration Sections

- **`gateway_server`**: Gateway server configuration (host, port, timeouts)
- **`admin_server`**: Admin server configuration (host, port, timeouts, metrics, logging, authentication)
- **`endpoints`**: Resource endpoint configurations
- **`facilitator`**: X402 facilitator configuration (private key, chain networks, supported schemes)

### Admin Server Configuration

The admin server provides additional configuration options:

#### Authentication

The admin server supports multiple authentication types:
- `bearer`: Bearer token authentication (Authorization: Bearer <token>)
- `basic`: Basic authentication (Authorization: Basic <base64-encoded-credentials>)
- `api_key`: API key authentication (X-API-Key header or api_key query parameter)

Configure authentication in `admin_server` section:
```yaml
admin_server:
  auth_enabled: true
  auth_type: "bearer"  # bearer, basic, or api_key
  auth_tokens: ["token1", "token2"]
```

#### Logging

Configure logging behavior:
```yaml
admin_server:
  log_level: "info"   # trace, debug, info, warn, error, fatal, panic
  log_format: "json"   # json or console
```

#### Metrics

Enable or disable Prometheus metrics:
```yaml
admin_server:
  metrics_enabled: true  # Enable /metrics endpoint
```

## API Endpoints

### Gateway Server (Port 8080)

#### Resource Discovery

Discover available resources:

```
GET /discover/resources?type=http&limit=20&offset=0
```

Query parameters:
- `type` (optional): Filter by resource type (e.g., "http")
- `limit` (optional): Maximum number of results (default: 20, max: 100)
- `offset` (optional): Pagination offset (default: 0)

#### Access Resources

Access resources through the gateway:

```
GET /api/{resource-path}
Authorization: Bearer <token>  # If auth middleware is enabled
X-Payment: <payment-payload-json>  # If x402-seller middleware is enabled
```

The gateway will:
1. Validate authentication (if `auth` middleware is configured)
2. Verify X402 payment (if `x402-seller` middleware is configured)
3. Proxy the request to the target URL

### Admin Server (Port 8081)

#### Health Checks

- `GET /health` - Basic health status (no authentication required)
- `GET /ready` - Detailed readiness status (checks facilitator initialization, no authentication required)
- `GET /metrics` - Prometheus metrics (authentication required if enabled)

**Note:** Health endpoints (`/health` and `/ready`) are accessible without authentication. All other admin endpoints require authentication if `admin_server.auth_enabled` is set to `true`.

## Resource Configuration

Resources are configured in the `endpoints` section of `config.yaml`. The resource mappings are dynamically reloaded from the in-memory configuration on each request, ensuring consistency with the current configuration state.

### Resource Configuration Format

Resources are defined in the `endpoints` section of your `config.yaml`:

```yaml
endpoints:
  - endpoint: "/api/premium-data"
    description: "Access to premium market data"
    type: "http"
    middlewares: ["x402-seller"]
    x402-seller:
      network: "sepolia"
      payTo: "0x93866dBB587db8b9f2C36570Ae083E3F9814e508"
      maxAmountRequired: "100000"
    targetUrl: "https://api.example.com/premium-data"
  
  - endpoint: "/api/weather-data"
    description: "Access to weather data API"
    type: "http"
    middlewares: ["auth", "x402-seller"]
    auth:
      type: "bearer"
      token: "1234567890"
    x402-seller:
      network: "localhost"
      payTo: "0x93866dBB587db8b9f2C36570Ae083E3F9814e508"
      maxAmountRequired: "100000"
    targetUrl: "https://api.example.com/weather-data"
```

### Resource Fields

- `endpoint` (required): The API endpoint path prefix (e.g., "/api/premium-data")
- `description` (optional): Human-readable description of the resource
- `type` (required): Resource type (e.g., "http")
- `middlewares` (optional): Array of middleware names to apply:
  - `"auth"`: Apply authentication middleware (requires `auth` configuration)
  - `"x402-buyer"`: Apply X402 buyer payment middleware
  - `"x402-seller"`: Apply X402 seller payment middleware (requires `X-Payment` header)
- `auth` (optional): Authentication configuration:
  - `type`: Authentication type (currently supports "bearer")
  - `token`: Token value for bearer authentication
- `x402-buyer` (optional): X402 buyer payment configuration:
  - `network`: Blockchain network name (must match a network in `facilitator.chain_networks`)
  - `payTo`: Payment recipient address
  - `maxAmountRequired`: Maximum payment amount required
- `x402-seller` (optional): X402 seller payment configuration:
  - `network`: Blockchain network name (must match a network in `facilitator.chain_networks`)
  - `payTo`: Payment recipient address
  - `maxAmountRequired`: Maximum payment amount required
- `targetUrl` (required): Backend URL to proxy requests to

**Note:** X402 configuration fields (scheme, asset, tokenName, etc.) are automatically populated from the `facilitator.chain_networks` configuration based on the specified `network` name.

### Middleware Behavior

- **Auth Middleware**: Validates authentication based on resource configuration. If `auth` is configured and `"auth"` is in the `middlewares` list, requests must include a valid Bearer token matching the configured token.
- **X402-Seller Middleware**: Validates and processes X402 payments. If `x402-seller` is configured and `"x402-seller"` is in the `middlewares` list, requests must include a valid `X-Payment` header with payment information. Returns `402 Payment Required` if payment is missing or invalid.
- **X402-Buyer Middleware**: Currently supported for configuration but buyer-side payment processing may be implemented differently.

### Chain Network Configuration

Chain networks are configured in the `facilitator.chain_networks` section:

```yaml
facilitator:
  chain_networks:
    - name: "sepolia"
      rpc: "https://ethereum-sepolia-rpc.publicnode.com"
      id: 11155111
      token_address: "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238"
      token_name: "USDC"
      token_version: "1"
      token_decimals: 6
      token_type: "ERC20"
```

The `network` field in `x402-buyer` or `x402-seller` must match one of the `name` values in `chain_networks`.

## Development

### Project Structure

```
go-agent-guide/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── gateway/             # Resource gateway implementation
│   ├── middleware/          # HTTP middlewares (auth, payment, metrics)
│   └── server/              # HTTP server implementation (gateway & admin)
├── examples/                # Example code and scripts
├── docs/                    # Documentation
├── config.example.yaml      # Example configuration file
├── ag-clientside/           # Client-side deployment files
└── ag-serverside/           # Server-side deployment files
```

### Building

```bash
# Build binary
go build -o agent-guide ./cmd

# Build with version info
go build -ldflags "-X main.AppVersion=1.0.0" -o agent-guide ./cmd
```

### Logging

The application uses structured logging with [zerolog](https://github.com/rs/zerolog). Logs can be configured in the `admin_server` section:

- **Log Level**: Controls verbosity (trace, debug, info, warn, error, fatal, panic)
- **Log Format**: Choose between JSON (production) or console (development) format

### Graceful Shutdown

The application supports graceful shutdown with a 30-second timeout. When receiving SIGINT or SIGTERM signals:
1. Both servers stop accepting new requests
2. Existing requests are allowed to complete
3. Resources are cleaned up
4. The application exits

### CORS

CORS (Cross-Origin Resource Sharing) is enabled by default on both servers, allowing requests from any origin. This can be customized by modifying the CORS middleware configuration in the server code.

## License

This project is licensed under the Apache License - see the [LICENSE](LICENSE) file for details.
