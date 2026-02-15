# STORY_01: Project Setup & Scaffolding

**Status:** Completed  
**Effort:** 4-6 hours  
**Epic:** EPIC_01 (Go Orchestrator Core)  
**Created:** 2026-02-15

---

## User Story

**As a** developer  
**I want** a properly initialized Go project with clear directory structure and dependencies  
**So that** I can start building the orchestrator with best practices from day 1

---

## Acceptance Criteria

- [ ] Go module initialized with `go.mod` and correct module path
- [ ] Directory structure follows Go best practices (internal/, cmd/, pkg/)
- [ ] All dependencies declared and managed via `go.mod`
- [ ] Makefile with common targets (build, test, lint, run, proto)
- [ ] GitHub Actions CI/CD pipeline configured
- [ ] Docker multi-stage build working
- [ ] README.md with setup and run instructions
- [ ] Development environment documented

---

## Technical Requirements

### 1. Go Module Initialization

**Command:**
```bash
cd orchestrator
go mod init github.com/your-org/subgen/orchestrator
```

**Module Path:** `github.com/your-org/subgen/orchestrator`  
**Go Version:** 1.21+ (for `go.mod`)

---

### 2. Directory Structure

**Created Structure:**
```
orchestrator/
├── cmd/
│   └── orchestrator/
│       └── main.go              # Application entry point
├── internal/                     # Private application code
│   ├── config/                   # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── webhooks/                 # Webhook HTTP handlers
│   │   ├── handler.go
│   │   ├── plex.go
│   │   ├── jellyfin.go
│   │   ├── emby.go
│   │   ├── tautulli.go
│   │   └── webhooks_test.go
│   ├── queue/                    # Priority queue implementation
│   │   ├── queue.go
│   │   ├── priority.go
│   │   └── queue_test.go
│   ├── mediaserver/              # Media server API clients
│   │   ├── plex.go
│   │   ├── jellyfin.go
│   │   └── mediaserver_test.go
│   ├── worker/                   # Worker discovery and management
│   │   ├── discovery.go
│   │   ├── localhost.go
│   │   ├── kubernetes.go
│   │   ├── pool.go
│   │   └── worker_test.go
│   ├── grpcclient/              # gRPC client to workers
│   │   ├── client.go
│   │   └── client_test.go
│   ├── metrics/                 # Prometheus metrics
│   │   └── metrics.go
│   └── server/                  # HTTP server setup
│       └── server.go
├── pkg/                         # Public library code (if needed)
│   └── api/
│       └── v1/                  # Generated protobuf code
│           ├── transcription.pb.go
│           └── transcription_grpc.pb.go
├── test/                        # Integration tests
│   └── integration/
│       └── orchestrator_test.go
├── deploy/                      # Kubernetes manifests / Helm
│   └── values.yaml
├── Dockerfile                   # Multi-stage Docker build
├── Makefile                     # Build automation
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── .golangci.yml               # Linter configuration
├── .github/
│   └── workflows/
│       └── ci.yml              # GitHub Actions CI
└── README.md                    # Project documentation
```

**Rationale:**
- `cmd/`: Entry points (one per binary)
- `internal/`: Private code (not importable by other projects)
- `pkg/`: Public libraries (can be imported)
- `test/`: Integration tests separate from unit tests
- `deploy/`: Kubernetes/Helm deployment files

---

### 3. Dependencies

**Required Go Modules:**

```go
// go.mod dependencies
require (
    github.com/gofiber/fiber/v2 v2.52.0          // HTTP server
    github.com/spf13/viper v1.18.2                // Configuration
    github.com/sirupsen/logrus v1.9.3             // Logging
    github.com/prometheus/client_golang v1.18.0   // Metrics
    github.com/stretchr/testify v1.8.4            // Testing
    google.golang.org/grpc v1.60.1                // gRPC client
    google.golang.org/protobuf v1.32.0            // Protobuf
    k8s.io/client-go v0.29.0                      // Kubernetes client (Phase 2)
    k8s.io/apimachinery v0.29.0                   // K8s API types
)
```

**Installation Command:**
```bash
go get github.com/gofiber/fiber/v2@v2.52.0
go get github.com/spf13/viper@v1.18.2
go get github.com/sirupsen/logrus@v1.9.3
go get github.com/prometheus/client_golang@v1.18.0
go get github.com/stretchr/testify@v1.8.4
go get google.golang.org/grpc@v1.60.1
go get google.golang.org/protobuf@v1.32.0
go get k8s.io/client-go@v0.29.0
go get k8s.io/apimachinery@v0.29.0
```

---

### 4. Makefile

**File:** `orchestrator/Makefile`

```makefile
.PHONY: build test lint run proto clean docker-build docker-run

# Variables
APP_NAME := orchestrator
DOCKER_IMAGE := subgen-orchestrator
VERSION := $(shell git describe --tags --always --dirty)
GOFLAGS := -v

# Build
build:
	@echo "Building $(APP_NAME)..."
	go build $(GOFLAGS) -o bin/$(APP_NAME) ./cmd/orchestrator

# Test
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Test with coverage report
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Lint
lint:
	@echo "Running linters..."
	golangci-lint run ./...

# Run locally
run: build
	@echo "Running $(APP_NAME)..."
	./bin/$(APP_NAME)

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	cd ../api && ./generate.sh

# Clean
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

# Docker run (development)
docker-run:
	@echo "Running Docker container..."
	docker run --rm -p 9000:9000 -p 9090:9090 \
		-e PLEX_TOKEN=${PLEX_TOKEN} \
		-e PLEX_SERVER=${PLEX_SERVER} \
		$(DOCKER_IMAGE):latest

# Install dev tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  test           - Run tests with race detector"
	@echo "  test-coverage  - Generate test coverage report"
	@echo "  lint           - Run linters"
	@echo "  run            - Build and run locally"
	@echo "  proto          - Generate protobuf code"
	@echo "  clean          - Remove build artifacts"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  install-tools  - Install development tools"
```

---

### 5. GitHub Actions CI/CD

**File:** `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Cache Go modules
        uses: actions/cache@v3
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-
      
      - name: Download dependencies
        working-directory: ./orchestrator
        run: go mod download
      
      - name: Run tests
        working-directory: ./orchestrator
        run: go test -v -race -coverprofile=coverage.out ./...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./orchestrator/coverage.out
  
  lint:
    name: Lint
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
          working-directory: ./orchestrator
  
  build:
    name: Build
    runs-on: ubuntu-latest
    needs: [test, lint]
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Build
        working-directory: ./orchestrator
        run: go build -v -o bin/orchestrator ./cmd/orchestrator
      
      - name: Upload artifact
        uses: actions/upload-artifact@v3
        with:
          name: orchestrator
          path: orchestrator/bin/orchestrator
  
  docker:
    name: Docker Build
    runs-on: ubuntu-latest
    needs: [test, lint]
    if: github.event_name == 'push'
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Build Docker image
        uses: docker/build-push-action@v5
        with:
          context: ./orchestrator
          file: ./orchestrator/Dockerfile
          push: false
          tags: subgen-orchestrator:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

---

### 6. Dockerfile (Multi-Stage Build)

**File:** `orchestrator/Dockerfile`

```dockerfile
# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/orchestrator ./cmd/orchestrator

# Stage 2: Runtime
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 568 -S appuser && \
    adduser -u 568 -S appuser -G appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/orchestrator .

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser

# Expose ports
EXPOSE 9000 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/app/orchestrator", "healthcheck"]

# Run
ENTRYPOINT ["/app/orchestrator"]
```

**Build Size:** ~20-30MB (Alpine-based)

---

### 7. Linter Configuration

**File:** `.golangci.yml`

```yaml
linters-settings:
  govet:
    check-shadowing: true
  golint:
    min-confidence: 0.8
  gocyclo:
    min-complexity: 15
  maligned:
    suggest-new: true
  dupl:
    threshold: 100
  goconst:
    min-len: 2
    min-occurrences: 2
  misspell:
    locale: US
  lll:
    line-length: 140
  goimports:
    local-prefixes: github.com/your-org/subgen/orchestrator
  gocritic:
    enabled-tags:
      - diagnostic
      - experimental
      - opinionated
      - performance
      - style

linters:
  enable:
    - bodyclose
    - deadcode
    - depguard
    - dogsled
    - dupl
    - errcheck
    - exportloopref
    - exhaustive
    - goconst
    - gocritic
    - gocyclo
    - gofmt
    - goimports
    - gomnd
    - goprintffuncname
    - gosec
    - gosimple
    - govet
    - ineffassign
    - lll
    - misspell
    - nakedret
    - noctx
    - nolintlint
    - rowserrcheck
    - staticcheck
    - structcheck
    - stylecheck
    - typecheck
    - unconvert
    - unparam
    - unused
    - varcheck
    - whitespace

run:
  timeout: 5m
  skip-dirs:
    - pkg/api/v1  # Generated protobuf code
```

---

### 8. README.md

**File:** `orchestrator/README.md`

```markdown
# Subgen Orchestrator

Go-based orchestrator for the Subgen transcription system. Handles webhooks, queue management, worker discovery, and media server integration.

## Features

- **Webhook Receivers**: Plex, Jellyfin, Emby, Tautulli
- **Priority Queue**: Bounded queue with deduplication
- **Worker Discovery**: Localhost (Phase 1) + Kubernetes (Phase 2)
- **gRPC Client**: Communication with Python workers
- **Media Server Integration**: Plex and Jellyfin API clients
- **Observability**: Prometheus metrics, structured logging, health checks

## Prerequisites

- Go 1.21+
- Docker (for containerized deployment)
- Make

## Development Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/your-org/subgen.git
   cd subgen/orchestrator
   ```

2. **Install dependencies:**
   ```bash
   go mod download
   ```

3. **Install dev tools:**
   ```bash
   make install-tools
   ```

4. **Generate protobuf code:**
   ```bash
   make proto
   ```

5. **Build:**
   ```bash
   make build
   ```

6. **Run tests:**
   ```bash
   make test
   ```

7. **Run locally:**
   ```bash
   export PLEX_TOKEN="your-token"
   export PLEX_SERVER="http://192.168.1.100:32400"
   make run
   ```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PLEX_TOKEN` | _(required)_ | Plex authentication token |
| `PLEX_SERVER` | `http://localhost:32400` | Plex server URL |
| `JELLYFIN_TOKEN` | _(optional)_ | Jellyfin authentication token |
| `JELLYFIN_SERVER` | `http://localhost:8096` | Jellyfin server URL |
| `WORKER_DISCOVERY` | `localhost` | `localhost` or `kubernetes` |
| `WORKER_ADDRESS` | `localhost:50051` | gRPC worker address (Phase 1) |
| `QUEUE_MAX_SIZE` | `1000` | Maximum queue size |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `WEBHOOK_PORT` | `9000` | HTTP webhook port |
| `METRICS_PORT` | `9090` | Prometheus metrics port |

## Docker

**Build:**
```bash
docker build -t subgen-orchestrator:latest .
```

**Run:**
```bash
docker run -d \
  -p 9000:9000 \
  -p 9090:9090 \
  -e PLEX_TOKEN="${PLEX_TOKEN}" \
  -e PLEX_SERVER="http://192.168.1.100:32400" \
  -e WORKER_ADDRESS="worker:50051" \
  subgen-orchestrator:latest
```

## Architecture

See [DESIGN/00_HYBRID_ARCHITECTURE.md](../docs/DESIGN/00_HYBRID_ARCHITECTURE.md)

## Testing

```bash
# Unit tests
make test

# Integration tests
go test -v ./test/integration/...

# Coverage report
make test-coverage
```

## Contributing

See [README-LLM.md](../README-LLM.md) for development workflow.

## License

MIT
```

---

## Test Cases

### Unit Tests

1. ✅ **Test: Go module initialized**
   - Verify `go.mod` exists with correct module path
   - Verify Go version is 1.21+

2. ✅ **Test: Directory structure created**
   - Verify all directories exist (`internal/`, `cmd/`, `pkg/`, etc.)
   - Verify placeholder files exist

3. ✅ **Test: Dependencies installed**
   - Run `go mod verify`
   - Verify all required packages importable

4. ✅ **Test: Makefile targets work**
   - Run `make build` → binary created
   - Run `make test` → passes (placeholder tests)
   - Run `make lint` → passes

5. ✅ **Test: Docker build succeeds**
   - Run `docker build .` → image created
   - Image size < 50MB

6. ✅ **Test: CI pipeline passes**
   - Push to GitHub → CI runs
   - All jobs (test, lint, build) succeed

---

## Implementation Steps

### Step 1: Initialize Go Module
```bash
cd orchestrator
go mod init github.com/your-org/subgen/orchestrator
```

### Step 2: Create Directory Structure
```bash
mkdir -p cmd/orchestrator
mkdir -p internal/{config,webhooks,queue,mediaserver,worker,grpcclient,metrics,server}
mkdir -p pkg/api/v1
mkdir -p test/integration
mkdir -p deploy
mkdir -p .github/workflows
```

### Step 3: Install Dependencies
```bash
go get github.com/gofiber/fiber/v2@v2.52.0
go get github.com/spf13/viper@v1.18.2
go get github.com/sirupsen/logrus@v1.9.3
go get github.com/prometheus/client_golang@v1.18.0
go get github.com/stretchr/testify@v1.8.4
go get google.golang.org/grpc@v1.60.1
go get google.golang.org/protobuf@v1.32.0
```

### Step 4: Create Placeholder Files
```bash
# main.go
cat > cmd/orchestrator/main.go <<'EOF'
package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.Info("Subgen Orchestrator starting...")
	fmt.Println("Hello from orchestrator!")
}
EOF

# Placeholder test
cat > internal/config/config_test.go <<'EOF'
package config

import "testing"

func TestPlaceholder(t *testing.T) {
	// Placeholder test
}
EOF
```

### Step 5: Create Makefile
- Copy Makefile content above

### Step 6: Create Dockerfile
- Copy Dockerfile content above

### Step 7: Create GitHub Actions CI
- Create `.github/workflows/ci.yml`
- Copy CI configuration above

### Step 8: Create .golangci.yml
- Copy linter configuration above

### Step 9: Create README.md
- Copy README content above

### Step 10: Initial Commit
```bash
git add .
git commit -m "feat: initialize Go orchestrator project structure"
git push origin develop
```

---

## Dependencies

**Requires:**
- None (first story)

**Blocks:**
- All other STORY_XX stories (need project structure)

---

## Notes

- ✅ Project structure already completed
- This story documents what was done
- Serves as reference for new developers
- All subsequent stories depend on this foundation

---

**Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
