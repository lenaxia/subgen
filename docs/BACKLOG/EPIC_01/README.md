# EPIC_01: Go Orchestrator Core

**Status:** Not Started  
**Estimated Effort:** 56-72 hours  
**Duration:** 1.5-2 weeks  
**Can Parallelize:** ✅ Yes (with EPIC_02)

---

## Overview

Build production-grade Go orchestrator with **scalability built-in from Day 1**. The orchestrator handles webhooks, queue management, worker discovery, and media server integration. **Critical**: Worker discovery abstraction enables Phase 1 (localhost) → Phase 2 (K8s) scaling with zero code changes.

---

## Goals

1. Replace FastAPI webhook server with Go HTTP server
2. Implement bounded priority queue with deduplication
3. Build worker discovery abstraction (localhost + Kubernetes)
4. Implement gRPC client pool with load balancing
5. Integrate with Plex/Jellyfin APIs
6. Production-ready observability (metrics, logging, health checks)

---

## Design References

- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md) - System architecture
- [01_GRPC_PROTOCOL.md](../../DESIGN/01_GRPC_PROTOCOL.md) - gRPC communication
- [03_SCALING_STRATEGY.md](../../DESIGN/03_SCALING_STRATEGY.md) - Phase 1 → Phase 2 scaling

---

## User Stories

### [STORY_01: Project Setup & Scaffolding](./stories/STORY_01_project_setup.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Summary:** Initialize Go module, directory structure, dependencies, CI/CD

### [STORY_02: Configuration Management](./stories/STORY_02_configuration.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Summary:** Environment variable loading, validation, config struct with defaults

### [STORY_03: Webhook Handlers](./stories/STORY_03_webhooks.md)
**Status:** Not Started  
**Effort:** 10-12 hours  
**Summary:** HTTP server with 4 webhook handlers (Plex, Jellyfin, Emby, Tautulli)

### [STORY_04: Priority Queue System](./stories/STORY_04_queue.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Bounded priority queue with deduplication, thread-safe operations

### [STORY_05: Media Server Clients](./stories/STORY_05_media_server_clients.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Plex and Jellyfin API clients with retry logic

### [STORY_06: Worker Discovery Abstraction](./stories/STORY_06_worker_discovery.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Pluggable discovery (localhost + Kubernetes), worker health tracking

### [STORY_07: gRPC Client Pool](./stories/STORY_07_grpc_client.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Summary:** gRPC client to workers, connection pooling, load balancing

### [STORY_08: Observability](./stories/STORY_08_observability.md)
**Status:** Not Started  
**Effort:** 8-10 hours  
**Summary:** Prometheus metrics, structured logging, health endpoints

---

## Acceptance Criteria

- [ ] All 8 stories completed
- [ ] All tests passing (unit + integration)
- [ ] Type safety enforced (Go compiler)
- [ ] Code coverage > 70%
- [ ] Orchestrator can discover localhost worker (Phase 1)
- [ ] Orchestrator can discover K8s workers (Phase 2)
- [ ] Load balancing works (round-robin + least-loaded)
- [ ] Webhook handlers process Plex/Jellyfin events
- [ ] Queue bounded at configurable size
- [ ] Prometheus metrics exposed on `/metrics`
- [ ] Health check endpoint works
- [ ] No memory leaks (tested with 1000 tasks)
- [ ] Work logs created for all stories

---

## Dependencies

**Requires:**
- None (first epic)

**Blocks:**
- EPIC_03 (Integration & Testing) - requires orchestrator to be functional

**Parallelizable With:**
- EPIC_02 (Python Worker Refactor) - independent codebases

---

## Technical Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| **HTTP Server** | Fiber | Fast, Express-like API, zero-allocation |
| **gRPC Client** | google.golang.org/grpc | Official gRPC Go library |
| **Logging** | logrus | Structured logging with JSON output |
| **Config** | viper | Environment variable management |
| **Testing** | testify | Assertions and mocking |
| **Metrics** | prometheus/client_golang | Prometheus metrics |
| **K8s Client** | k8s.io/client-go | Kubernetes API access |

---

## Key Design Decisions

### 1. Worker Discovery Abstraction

**Decision:** Interface-based discovery with 2 implementations (localhost, Kubernetes)

**Code Structure:**
```go
// internal/worker/discovery.go
type WorkerDiscovery interface {
    GetWorkers(ctx context.Context) ([]Worker, error)
    Watch(ctx context.Context) (<-chan WorkerEvent, error)
}

// Phase 1
type LocalhostDiscovery struct { ... }

// Phase 2
type KubernetesDiscovery struct { ... }
```

**Why:** Zero code changes to scale from Phase 1 → Phase 2 (config-driven)

---

### 2. Bounded Priority Queue

**Decision:** Use container/heap with max size limit and deduplication

**Why:**
- Prevents memory exhaustion (current subgen.py has unbounded queue)
- Priority: Language detection (0) > ASR (1) > Transcription (2)
- Deduplication prevents duplicate work

**Trade-off:** Queue can fill up (return 503), but better than OOM

---

### 3. Load Balancing Strategy

**Decision:** Support both round-robin and least-loaded

**Round-robin:** Simple, fair distribution  
**Least-loaded:** Better for varying job durations (recommended)

**Configurable via:** `LOAD_BALANCE_STRATEGY` environment variable

---

### 4. Context-Based Lifecycle

**Decision:** All long-running operations use context.Context

**Why:**
- Automatic cancellation on timeout
- No goroutine leaks
- No connection leaks
- Idiomatic Go

---

### 5. Structured Logging

**Decision:** logrus with JSON format

**Example:**
```go
log.WithFields(logrus.Fields{
    "task_id":   task.ID,
    "file_path": redactPath(task.FilePath),
    "worker_id": worker.ID,
}).Info("task dispatched")
```

**Why:** Easy to parse, aggregate, and search in log management systems

---

## Timeline

**Week 1:**
- Day 1-2: STORY_01 (Project Setup) + STORY_02 (Config)
- Day 3-4: STORY_03 (Webhooks)
- Day 5: STORY_04 (Queue) - start

**Week 2:**
- Day 1: STORY_04 (Queue) - complete
- Day 2: STORY_05 (Media Server Clients)
- Day 3: STORY_06 (Worker Discovery)
- Day 4: STORY_07 (gRPC Client)
- Day 5: STORY_08 (Observability)

**Buffer:** 2-3 days for issues, testing, integration

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| K8s client complexity | Medium | Use examples from client-go, test with kind cluster |
| gRPC connection pooling | Medium | Use grpc.DialPool, test with multiple workers |
| Queue thread safety | High | Use sync.RWMutex, test with race detector |
| Context cancellation bugs | Medium | Test with timeouts, use defer cleanup |
| Memory leaks | High | Test with 1000+ tasks, use pprof profiling |

---

## Testing Strategy

### Unit Tests (70%+ coverage)
- Config loading/validation
- Queue operations (push, pop, dedup)
- Worker discovery (localhost + K8s mocked)
- Load balancing algorithms
- Webhook payload parsing

### Integration Tests
- HTTP webhook → Queue flow
- Queue → Worker dispatch flow
- gRPC client → Mock worker
- K8s API mocked discovery

### Performance Tests
- 1000 tasks → Queue (no leaks)
- 100 concurrent webhook requests
- Worker discovery latency

---

## Definition of Done

- [ ] All 8 stories completed with ✅ status
- [ ] All tests passing (unit + integration)
- [ ] Code review completed
- [ ] Type safety enforced (go build succeeds)
- [ ] golangci-lint passes with zero warnings
- [ ] Memory leak tests pass (1000 tasks)
- [ ] Load tests pass (100 concurrent requests)
- [ ] Documentation complete (README, godoc comments)
- [ ] Work logs created for each story
- [ ] Docker image builds successfully
- [ ] GitHub Actions CI passes

---

## Next Epic

**EPIC_02: Python Worker Refactor** (can run in parallel)

**Integration Point:** After both EPIC_01 and EPIC_02 complete, proceed to EPIC_03 (Integration & Testing) to validate orchestrator ↔ worker communication.

---

## References

- README-LLM.md - Development workflow, critical rules
- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md)
- [01_GRPC_PROTOCOL.md](../../DESIGN/01_GRPC_PROTOCOL.md)
- [03_SCALING_STRATEGY.md](../../DESIGN/03_SCALING_STRATEGY.md)
- Legacy code: `subgen.py` (webhook handlers, queue logic to port)

---

**Epic Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
