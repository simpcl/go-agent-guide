# Agent Guide

An agent gateway service that integrates with X402 payment facilitator for payment verification and settlement. This gateway provides reverse proxy capabilities, resource management, authentication, and monitoring.

## Features

- ✅ **Resource Gateway** - Reverse proxy with payment integration
- ✅ **Resource Management** - JSON-based resource configuration with hot-reload support
- ✅ **Payment Integration** - Automatic X402 payment verification and settlement
- ✅ **Authentication** - API key and Bearer token authentication middleware
- ✅ **Monitoring** - Prometheus metrics and structured logging

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP API      │    │   Resource      │    │   X402          │
│   (Gin)         │───▶│   Gateway       │───▶│   Facilitator   │
│                 │    │                 │    │   (Library)     │
│ /api/*          │    │ - Resource mgmt │    │ - Verify        │
│ /resources/*    │    │ - Payment proc  │    │ - Settle        │
│ /health         │    │ - Reverse proxy │    │                 │
│ /ready          │    │ - Hot reload    │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.23 or later
- X402 Facilitator library (go-x402-facilitator)

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
   
   cp resources.json.example resources.json
   # Edit resources.json with your resource configurations
   ```

4. **Set environment variables (optional):**
   ```bash
   export AGENTGUIDE_FACILITATOR_PRIVATE_KEY="your-private-key"
   export AGENTGUIDE_AUTH_API_KEYS="key1,key2,key3"
   export AGENTGUIDE_AUTH_JWT_SECRET="your-jwt-secret"
   ```

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

1. **Configuration file** (`config.yaml`)
2. **Environment variables** (prefixed with `AGENTGUIDE_`)

## API Endpoints

### Resource Discovery

Discover available resources:

```
GET /resources/discover?type=http&limit=20&offset=0
```

Query parameters:
- `type` (optional): Filter by resource type (e.g., "http")
- `limit` (optional): Maximum number of results (default: 20, max: 100)
- `offset` (optional): Pagination offset (default: 0)

### Access Resources

Access resources through the gateway:

```
GET /api/{resource-path}
X-Payment: <payment-payload-json>
```

The gateway will:
1. Validate authentication (if required)
2. Verify X402 payment (if configured)
3. Proxy the request to the target URL

### Health Checks

- `GET /health` - Basic health status
- `GET /ready` - Detailed readiness status (checks facilitator initialization)
- `GET /metrics` - Prometheus metrics (on port 9090)

## Resource Configuration

Resources are configured in `resources.json`. The file is automatically reloaded when modified.

### Resource Configuration Format

```json
{
  "resources": [
    {
      "resource": "/api/premium-data",
      "type": "http",
      "middlewares": ["auth", "x402"],
      "auth": {
        "type": "bearer",
        "token": "1234567890"
      },
      "x402": {
        "x402Version": 1,
        "scheme": "exact",
        "network": "sepolia",
        "resource": "/premium-data",
        "description": "Access to premium market data",
        "maxAmountRequired": "100000",
        "payTo": "0x93866dBB587db8b9f2C36570Ae083E3F9814e508",
        "assetType": "ERC20",
        "asset": "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238",
        "tokenName": "AgentNetworkCoin",
        "tokenVersion": "1"
      },
      "targetUrl": "https://api.example.com/premium-data"
    }
  ]
}
```

### Resource Fields

- `resource` (required): The API endpoint path prefix
- `type` (required): Resource type (e.g., "http")
- `middlewares` (optional): Array of middleware names to apply:
  - `"auth"`: Apply authentication middleware
  - `"x402"`: Apply X402 payment middleware
- `auth` (optional): Authentication configuration:
  - `type`: Authentication type (e.g., "bearer")
  - `token`: Token value for bearer authentication
- `x402` (optional): X402 payment configuration:
  - `x402Version`: X402 protocol version
  - `scheme`: Payment scheme (e.g., "exact")
  - `network`: Blockchain network name
  - `resource`: Resource identifier for payment
  - `description`: Human-readable description
  - `maxAmountRequired`: Maximum payment amount required
  - `payTo`: Payment recipient address
  - `assetType`: Asset type (e.g., "ERC20")
  - `asset`: Asset contract address
  - `tokenName`: Token name
  - `tokenVersion`: Token version
- `targetUrl` (required): Backend URL to proxy requests to

### Middleware Behavior

- **Auth Middleware**: Validates authentication based on resource configuration. If `auth` is configured, requests must include a valid Bearer token.
- **X402 Payment Middleware**: Validates and processes X402 payments. If `x402` is configured, requests must include a valid `X-Payment` header with payment information.

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
│   └── server/              # HTTP server implementation
├── examples/                # Example code and scripts
├── docs/                    # Documentation
├── config.example.yaml      # Example configuration file
└── resources.json.example   # Example resources configuration
```

### Building

```bash
# Build binary
go build -o agent-guide ./cmd

# Build with version info
go build -ldflags "-X main.AppVersion=1.0.0" -o agent-guide ./cmd
```

## License

This project is licensed under the Apache License - see the [LICENSE](LICENSE) file for details.
