# Subgen Orchestrator (Go)

The orchestrator component of the Subgen subtitle generation service. Written in Go, it handles HTTP webhooks, queue management, worker discovery, and gRPC communication with Python workers.

## Architecture

This is part of the **Hybrid Go + Python Architecture**:

```
┌─────────────────────────────────────┐
│     Go Orchestrator (this)          │
│  • HTTP Webhook Server              │
│  • Priority Queue Management        │
│  • Media Server API Clients         │
│  • Worker Discovery & Load Balancing│
│  • gRPC Client to Workers           │
│  • Prometheus Metrics               │
└─────────────────┬───────────────────┘
                  │ gRPC
                  ↓
┌─────────────────────────────────────┐
│     Python Worker                   │
│  • gRPC Server                      │
│  • Whisper Transcription            │
│  • Subtitle Generation              │
└─────────────────────────────────────┘
```

## Features

- **Webhook Handlers**: Plex, Jellyfin, Emby, Tautulli, Bazarr
- **Priority Queue**: Bounded, deduplicated, thread-safe
- **Worker Discovery**: Localhost (Phase 1) + Kubernetes (Phase 2)
- **Load Balancing**: Round-robin and least-loaded strategies
- **Observability**: Prometheus metrics, structured logging, health checks
- **Memory Safety**: No unbounded maps, context-based cleanup, no leaks

## Building

### Local Development

```bash
# Install dependencies
go mod download

# Run tests
go test ./... -v

# Build binary
go build -o bin/orchestrator ./cmd/orchestrator

# Run
./bin/orchestrator
```

### Docker Build

```bash
docker build -t subgen-orchestrator:dev -f orchestrator/Dockerfile .
```

## Configuration

Environment variables (see `internal/config/config.go`):

### HTTP Server
- `WEBHOOK_PORT` (default: 9000) - HTTP webhook server port
- `METRICS_PORT` (default: 9090) - Prometheus metrics port

### Queue
- `QUEUE_MAX_SIZE` (default: 1000) - Maximum queue size
- `QUEUE_WORKERS` (default: 4) - Number of queue worker goroutines

### Worker Discovery
- `WORKER_DISCOVERY` (default: localhost) - Discovery mode: localhost, kubernetes
- `WORKER_ADDRESS` (default: localhost:50051) - Worker gRPC address (localhost mode)
- `WORKER_NAMESPACE` (default: media) - Kubernetes namespace (k8s mode)
- `WORKER_SERVICE_NAME` (default: subgen-worker) - K8s service name

### Media Servers
- `PLEX_SERVER` (default: http://plex:32400) - Plex server URL
- `PLEX_TOKEN` - Plex authentication token
- `JELLYFIN_SERVER` (default: http://jellyfin:8096) - Jellyfin server URL
- `JELLYFIN_TOKEN` - Jellyfin authentication token

### Logging
- `LOG_LEVEL` (default: info) - Log level: debug, info, warn, error
- `LOG_FORMAT` (default: json) - Log format: json, text

## Endpoints

### Webhooks (Port 9000)

- `POST /plex` - Plex webhook handler
- `POST /jellyfin` - Jellyfin webhook handler
- `POST /emby` - Emby webhook handler
- `POST /tautulli` - Tautulli webhook handler
- `POST /asr` - Bazarr ASR provider endpoint

### Observability

- `GET /health` - Health check (alive/ready)
- `GET /metrics` - Prometheus metrics (port 9090)
- `GET /version` - Version information

## Testing

```bash
# Unit tests
go test ./... -v

# With race detector
go test ./... -v -race

# With coverage
go test ./... -v -coverprofile=coverage.out
go tool cover -html=coverage.out

# Integration tests
go test ./test/integration/... -v
```

## Project Structure

```
orchestrator/
├── cmd/
│   └── orchestrator/
│       ├── main.go          # Entry point
│       └── main_test.go     # Main package tests
├── internal/
│   ├── config/              # Configuration management
│   ├── webhooks/            # HTTP webhook handlers
│   ├── queue/               # Priority queue
│   ├── mediaserver/         # Plex/Jellyfin API clients
│   ├── discovery/           # Worker discovery
│   ├── grpc_client/         # gRPC client to workers
│   └── observability/       # Metrics, logging, health
├── pkg/
│   └── pb/                  # Generated protobuf code
├── test/
│   ├── integration/         # Integration tests
│   └── fixtures/            # Test data
├── go.mod
├── go.sum
├── Dockerfile
└── README.md (this file)
```

## Dependencies

- **HTTP Server**: [Fiber v2](https://gofiber.io/) - Fast, Express-like framework
- **gRPC**: [google.golang.org/grpc](https://grpc.io) - Official gRPC Go library
- **Logging**: [logrus](https://github.com/sirupsen/logrus) - Structured JSON logging
- **Config**: [caarlos0/env](https://github.com/caarlos0/env) - Environment variable parsing
- **Testing**: [testify](https://github.com/stretchr/testify) - Test assertions
- **Metrics**: [prometheus/client_golang](https://github.com/prometheus/client_golang) - Prometheus client
- **TTL Cache**: [ttlcache/v3](https://github.com/jellydator/ttlcache) - Memory-safe cache

## Memory Management

The orchestrator is designed to have **zero memory leaks**:

1. **Bounded Queue**: Max size limit prevents unbounded growth
2. **No Task Storage**: Results logged, not stored in memory
3. **Context Cleanup**: All long-running operations use context.Context
4. **Defer Pattern**: Resources cleaned up with defer statements
5. **TTL Cache**: Bounded cache with automatic expiration

See `docs/DESIGN/02_MEMORY_MANAGEMENT.md` for details.

## Development

### Adding a New Webhook Handler

1. Add handler to `internal/webhooks/`
2. Register route in `internal/webhooks/server.go`
3. Write tests in `*_test.go`
4. Update documentation

### Adding a New Media Server Client

1. Add client to `internal/mediaserver/`
2. Implement interface methods
3. Add configuration fields
4. Write integration tests

## Deployment

### Kubernetes (bjw-s app-template)

See `deploy/values.yaml` for Kubernetes deployment configuration.

**Phase 1 (Single Pod)**:
- Orchestrator + Worker in same pod
- gRPC via localhost:50051

**Phase 2 (Scaled)**:
- Orchestrator: 1 replica
- Workers: N replicas (StatefulSet)
- gRPC via K8s Service

## Contributing

Follow TDD principles:
1. Write tests FIRST
2. Implement code to pass tests
3. Ensure all tests pass
4. Run linter: `golangci-lint run`

## License

Same as parent project (Subgen).

## References

- [Architecture Design](../docs/DESIGN/00_HYBRID_ARCHITECTURE.md)
- [gRPC Protocol](../docs/DESIGN/01_GRPC_PROTOCOL.md)
- [Memory Management](../docs/DESIGN/02_MEMORY_MANAGEMENT.md)
- [Parent Project](../README.md)
