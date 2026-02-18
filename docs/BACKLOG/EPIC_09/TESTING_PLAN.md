# EPIC_09: Testing Plan

**Document Version:** 1.0  
**Last Updated:** 2026-02-17  
**Status:** Final  
**Related Documents:**
- [EPIC_09 README](./README.md)
- [05_WORKER_POOL_CONCURRENCY.md](../../DESIGN/05_WORKER_POOL_CONCURRENCY.md)
- [06_K8S_API_ERROR_HANDLING.md](../../DESIGN/06_K8S_API_ERROR_HANDLING.md)

---

## Table of Contents

1. [Overview](#overview)
2. [Test Environment Setup](#test-environment-setup)
3. [Test Data Requirements](#test-data-requirements)
4. [Test Categories](#test-categories)
5. [Success Criteria](#success-criteria)
6. [Test Procedures](#test-procedures)
7. [Load Testing](#load-testing)
8. [Failure Scenario Testing](#failure-scenario-testing)

---

## Overview

### Purpose

Define comprehensive testing strategy for Epic 9 (Horizontal Scaling & Multi-Worker Support) to ensure:
1. Kubernetes worker discovery functions correctly
2. Load balancing distributes tasks evenly
3. System handles worker failures gracefully
4. Health checks accurately reflect system state
5. No race conditions or memory leaks

### Test Levels

```
Unit Tests (Go + Python)
    ↓
Integration Tests (Docker Compose)
    ↓
End-to-End Tests (Kind/Minikube)
    ↓
Load Tests (Real cluster)
    ↓
Chaos Tests (Failure scenarios)
```

---

## Test Environment Setup

### Option 1: Kind (Recommended for CI)

**Pros**:
- ✅ Fast setup (< 2 minutes)
- ✅ Consistent across machines
- ✅ Easy to reset
- ✅ Works in CI/CD pipelines

**Cons**:
- ❌ Requires Docker
- ❌ Not a "real" cluster

**Setup Script**:

```bash
#!/bin/bash
# test/setup-kind-cluster.sh

set -e

echo "Creating Kind cluster for Epic 9 testing..."

# Create cluster with specific K8s version
kind create cluster --name subgen-test --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
- role: worker
- role: worker
EOF

# Wait for cluster to be ready
kubectl wait --for=condition=Ready nodes --all --timeout=300s

# Install metrics-server (for HPA testing later)
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch -n kube-system deployment metrics-server --type=json \
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

# Create test namespace
kubectl create namespace subgen-test

echo "✅ Kind cluster ready"
echo "Nodes:"
kubectl get nodes
echo ""
echo "To use this cluster:"
echo "  kubectl config use-context kind-subgen-test"
```

**Teardown**:
```bash
kind delete cluster --name subgen-test
```

---

### Option 2: Minikube

**Pros**:
- ✅ More VM drivers (Docker, VirtualBox, KVM)
- ✅ Supports LoadBalancer services via `minikube tunnel`
- ✅ Built-in addons

**Cons**:
- ❌ Slower than Kind
- ❌ Requires more system resources

**Setup Script**:

```bash
#!/bin/bash
# test/setup-minikube-cluster.sh

set -e

echo "Creating Minikube cluster for Epic 9 testing..."

# Start Minikube with 3 nodes
minikube start \
  --nodes=3 \
  --cpus=4 \
  --memory=8192 \
  --kubernetes-version=v1.28.0 \
  --driver=docker \
  --profile=subgen-test

# Enable addons
minikube addons enable metrics-server -p subgen-test

# Create test namespace
kubectl create namespace subgen-test

echo "✅ Minikube cluster ready"
kubectl get nodes
```

**Teardown**:
```bash
minikube delete --profile=subgen-test
```

---

### Option 3: Real Cluster (For Production-like Testing)

**Requirements**:
- Kubernetes 1.24+ cluster
- RBAC enabled
- At least 3 worker nodes
- Sufficient resources (8 CPU, 16GB RAM)

**Not recommended for unit/integration testing** (use Kind instead)

---

## Test Data Requirements

### Sample Media Files

**Purpose**: Simulate real transcription workload for load testing

**Requirements**:
- **Count**: 10-20 sample files
- **Formats**: MP4, MKV, AVI
- **Duration**: Mix of short (1min), medium (5min), long (20min)
- **Audio tracks**: Mix of single and multi-audio
- **Total size**: ~500MB

**Generation Script**:

```bash
#!/bin/bash
# test/generate-test-media.sh

set -e

OUTPUT_DIR="test/testdata/epic09"
mkdir -p "$OUTPUT_DIR"

echo "Generating test media files..."

# Short files (1 minute)
for i in {1..5}; do
  ffmpeg -f lavfi -i testsrc=duration=60:size=640x480:rate=30 \
         -f lavfi -i sine=frequency=1000:duration=60 \
         -c:v libx264 -c:a aac \
         "$OUTPUT_DIR/short_${i}.mp4" -y
done

# Medium files (5 minutes)
for i in {1..3}; do
  ffmpeg -f lavfi -i testsrc=duration=300:size=1280x720:rate=30 \
         -f lavfi -i sine=frequency=1000:duration=300 \
         -c:v libx264 -c:a aac \
         "$OUTPUT_DIR/medium_${i}.mp4" -y
done

# Long files (20 minutes)
for i in {1..2}; do
  ffmpeg -f lavfi -i testsrc=duration=1200:size=1920x1080:rate=30 \
         -f lavfi -i sine=frequency=1000:duration=1200 \
         -c:v libx264 -c:a aac \
         "$OUTPUT_DIR/long_${i}.mp4" -y
done

echo "✅ Generated $(ls -1 $OUTPUT_DIR | wc -l) test files"
du -sh "$OUTPUT_DIR"
```

---

### Kubernetes Test Resources

**RBAC Configuration** (`test/k8s/rbac.yaml`):
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: subgen-orchestrator
  namespace: subgen-test

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: subgen-orchestrator
  namespace: subgen-test
rules:
- apiGroups: [""]
  resources: ["endpoints"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: subgen-orchestrator
  namespace: subgen-test
subjects:
- kind: ServiceAccount
  name: subgen-orchestrator
  namespace: subgen-test
roleRef:
  kind: Role
  name: subgen-orchestrator
  apiGroup: rbac.authorization.k8s.io
```

---

## Test Categories

### 1. Unit Tests (Go)

**Location**: `orchestrator/internal/discovery/*_test.go`

**Coverage Target**: 85%+

**Tests**:
- Worker pool operations (add, remove, get, list)
- Load balancing strategies (round-robin, least-loaded)
- Health status updates (atomic operations)
- Cache fallback logic
- Error handling (RBAC, NotFound, Timeout)
- Race detector tests (concurrent operations)

**Run Command**:
```bash
cd orchestrator
go test -v -race -count=10 ./internal/discovery/...
```

---

### 2. Unit Tests (Python)

**Location**: `worker/tests/test_http_server.py`

**Coverage Target**: 80%+

**Tests**:
- Health endpoint (always returns 200)
- Ready endpoint (various conditions)
- Metrics endpoint (comprehensive stats)
- Thread safety (HTTP + gRPC concurrency)

**Run Command**:
```bash
cd worker
pytest tests/test_http_server.py -v --cov=src/http_server
```

---

### 3. Integration Tests (Docker Compose)

**Location**: `test/integration/epic09/`

**Purpose**: Test with real containers, no K8s complexity

**Tests**:
```bash
#!/bin/bash
# test/integration/epic09/test-docker-health.sh

set -e

echo "Testing health endpoints with Docker Compose..."

# Start services
docker-compose -f docker-compose.test.yml up -d

# Wait for services
sleep 30

# Test orchestrator health
curl -f http://localhost:9000/health || exit 1
curl -f http://localhost:9000/healthz || exit 1
curl -f http://localhost:9000/ready || exit 1

# Test worker health
curl -f http://localhost:8080/health || exit 1
curl -f http://localhost:8080/ready || exit 1
curl -f http://localhost:8080/metrics || exit 1

# Cleanup
docker-compose -f docker-compose.test.yml down

echo "✅ Docker health checks passed"
```

---

### 4. End-to-End Tests (Kind)

**Location**: `test/e2e/epic09/`

**Purpose**: Test full Phase 2 deployment with K8s

**Test Suite**:

#### Test 1: Worker Discovery

```bash
#!/bin/bash
# test/e2e/epic09/test-worker-discovery.sh

set -e

echo "Test 1: Worker Discovery"

# Deploy workers (3 replicas)
helm install subgen-worker bjw-s/app-template \
  --namespace subgen-test \
  --values test/k8s/values-workers.yaml \
  --wait --timeout=300s

# Deploy orchestrator
helm install subgen-orchestrator bjw-s/app-template \
  --namespace subgen-test \
  --values test/k8s/values-orchestrator.yaml \
  --wait --timeout=60s

# Wait for orchestrator to discover workers
sleep 10

# Check orchestrator logs
DISCOVERED=$(kubectl logs -n subgen-test -l app.kubernetes.io/name=subgen-orchestrator --tail=50 | grep "Discovered.*workers")

if [[ -z "$DISCOVERED" ]]; then
  echo "❌ Workers not discovered"
  exit 1
fi

# Verify /ready endpoint reports 3 workers
READY=$(kubectl run -n subgen-test curl-test --image=curlimages/curl --rm -i --restart=Never -- \
  curl -s http://subgen-orchestrator:9000/ready)

WORKER_COUNT=$(echo "$READY" | jq -r '.workers_total')
if [[ "$WORKER_COUNT" != "3" ]]; then
  echo "❌ Expected 3 workers, found $WORKER_COUNT"
  exit 1
fi

echo "✅ Worker discovery test passed"
```

---

#### Test 2: Load Balancing (Round Robin)

```bash
#!/bin/bash
# test/e2e/epic09/test-load-balancing-roundrobin.sh

set -e

echo "Test 2: Load Balancing (Round Robin)"

# Set strategy to round-robin
kubectl set env deployment/subgen-orchestrator \
  -n subgen-test \
  LOAD_BALANCE_STRATEGY=round_robin

# Queue 12 tasks (should distribute 4-4-4)
for i in {1..12}; do
  kubectl run -n subgen-test queue-task-$i --image=curlimages/curl --rm -i --restart=Never -- \
    curl -X POST http://subgen-orchestrator:9000/batch \
    -d '{"path":"/media/test/file'$i'.mp4"}'
done

# Wait for tasks to complete
sleep 60

# Check worker metrics
METRICS=$(kubectl run -n subgen-test metrics-test --image=curlimages/curl --rm -i --restart=Never -- \
  curl -s http://subgen-orchestrator:9090/metrics)

# Parse job counts per worker
# Expected: ~4 jobs per worker (±1 acceptable)
W1=$(echo "$METRICS" | grep 'subgen_worker_jobs_processed{worker="subgen-worker-0"}' | awk '{print $2}')
W2=$(echo "$METRICS" | grep 'subgen_worker_jobs_processed{worker="subgen-worker-1"}' | awk '{print $2}')
W3=$(echo "$METRICS" | grep 'subgen_worker_jobs_processed{worker="subgen-worker-2"}' | awk '{print $2}')

echo "Worker 0: $W1 jobs"
echo "Worker 1: $W2 jobs"
echo "Worker 2: $W3 jobs"

# Verify distribution is even (±2 tolerance)
MAX_DIFF=2
if (( $(echo "$W1 - $W2" | bc) > MAX_DIFF )) || (( $(echo "$W2 - $W3" | bc) > MAX_DIFF )); then
  echo "❌ Load distribution uneven"
  exit 1
fi

echo "✅ Round-robin load balancing test passed"
```

---

#### Test 3: Dynamic Scaling

```bash
#!/bin/bash
# test/e2e/epic09/test-dynamic-scaling.sh

set -e

echo "Test 3: Dynamic Scaling"

# Start with 3 workers
kubectl scale statefulset subgen-worker --replicas=3 -n subgen-test
sleep 30

# Verify orchestrator knows about 3 workers
WORKERS=$(kubectl logs -n subgen-test -l app.kubernetes.io/name=subgen-orchestrator --tail=10 | grep "Discovered 3 workers")
if [[ -z "$WORKERS" ]]; then
  echo "❌ Initial 3 workers not discovered"
  exit 1
fi

# Scale up to 5 workers
echo "Scaling up to 5 workers..."
kubectl scale statefulset subgen-worker --replicas=5 -n subgen-test

# Wait for new pods to be ready
kubectl wait --for=condition=Ready pod/subgen-worker-3 -n subgen-test --timeout=120s
kubectl wait --for=condition=Ready pod/subgen-worker-4 -n subgen-test --timeout=120s

# Wait for orchestrator to discover new workers (watch should detect within 30s)
sleep 35

# Verify orchestrator discovered 5 workers
WORKERS=$(kubectl logs -n subgen-test -l app.kubernetes.io/name=subgen-orchestrator --tail=20 | grep "Discovered 5 workers")
if [[ -z "$WORKERS" ]]; then
  echo "❌ Scale-up not detected"
  exit 1
fi

# Scale down to 3 workers
echo "Scaling down to 3 workers..."
kubectl scale statefulset subgen-worker --replicas=3 -n subgen-test
sleep 35

# Verify orchestrator removed 2 workers
WORKERS=$(kubectl logs -n subgen-test -l app.kubernetes.io/name=subgen-orchestrator --tail=20 | grep "Discovered 3 workers")
if [[ -z "$WORKERS" ]]; then
  echo "❌ Scale-down not detected"
  exit 1
fi

echo "✅ Dynamic scaling test passed"
```

---

#### Test 4: Worker Failure Handling

```bash
#!/bin/bash
# test/e2e/epic09/test-worker-failure.sh

set -e

echo "Test 4: Worker Failure Handling"

# Start with 3 workers
kubectl scale statefulset subgen-worker --replicas=3 -n subgen-test
sleep 30

# Queue a task
TASK_ID=$(kubectl run -n subgen-test queue-task --image=curlimages/curl --rm -i --restart=Never -- \
  curl -X POST http://subgen-orchestrator:9000/batch \
  -d '{"path":"/media/test/long_1.mp4"}' | jq -r '.task_id')

# Wait for task to start (check logs for "dispatching")
sleep 10

# Kill the worker processing the task
# (Find which worker has active jobs)
ACTIVE_WORKER=$(kubectl get pods -n subgen-test -l app.kubernetes.io/name=subgen-worker -o json | \
  jq -r '.items[] | select(.status.phase=="Running") | .metadata.name' | head -1)

echo "Killing worker: $ACTIVE_WORKER"
kubectl delete pod $ACTIVE_WORKER -n subgen-test --grace-period=0 --force

# Verify orchestrator requeues task
sleep 10
REQUEUED=$(kubectl logs -n subgen-test -l app.kubernetes.io/name=subgen-orchestrator --tail=50 | grep "requeuing")
if [[ -z "$REQUEUED" ]]; then
  echo "❌ Task not requeued after worker failure"
  exit 1
fi

# Verify task eventually completes on another worker
sleep 120
COMPLETED=$(kubectl logs -n subgen-test -l app.kubernetes.io/name=subgen-orchestrator --tail=100 | grep "task completed")
if [[ -z "$COMPLETED" ]]; then
  echo "❌ Task did not complete after requeue"
  exit 1
fi

echo "✅ Worker failure handling test passed"
```

---

#### Test 5: RBAC Permission Denied

```bash
#!/bin/bash
# test/e2e/epic09/test-rbac-error.sh

set -e

echo "Test 5: RBAC Permission Denied"

# Deploy orchestrator WITHOUT RBAC
helm install subgen-orchestrator-norbac bjw-s/app-template \
  --namespace subgen-test \
  --values test/k8s/values-orchestrator-norbac.yaml \
  --wait --timeout=60s

# Check logs for RBAC error
sleep 10
RBAC_ERROR=$(kubectl logs -n subgen-test -l app.kubernetes.io/name=subgen-orchestrator-norbac --tail=50 | grep "RBAC")
if [[ -z "$RBAC_ERROR" ]]; then
  echo "❌ RBAC error not detected"
  exit 1
fi

# Verify orchestrator is still running (graceful degradation)
RUNNING=$(kubectl get pods -n subgen-test -l app.kubernetes.io/name=subgen-orchestrator-norbac -o jsonpath='{.items[0].status.phase}')
if [[ "$RUNNING" != "Running" ]]; then
  echo "❌ Orchestrator crashed instead of degrading gracefully"
  exit 1
fi

# Cleanup
helm uninstall subgen-orchestrator-norbac -n subgen-test

echo "✅ RBAC error handling test passed"
```

---

## Success Criteria

### Quantitative Metrics

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| **Worker Discovery Time** | < 30 seconds | Time from worker pod ready → orchestrator logs "discovered" |
| **Load Balance Distribution** | ±10% of ideal | Jobs per worker should be within 10% of N/workers |
| **Health Check Response Time** | < 100ms | `curl -w '%{time_total}'` on health endpoints |
| **Watch Reconnection Time** | < 60 seconds | Time from watch disconnect → reconnect logged |
| **Task Requeue Time** | < 5 seconds | Time from worker failure → task requeued |
| **Race Detector** | 0 races | `go test -race` must pass |
| **Unit Test Coverage** | > 80% | `go test -cover` orchestrator, `pytest --cov` worker |

---

### Qualitative Criteria

- [ ] No memory leaks (observe for 1 hour under load)
- [ ] No panics or crashes in any scenario
- [ ] Logs are clear and actionable
- [ ] Metrics accurately reflect system state
- [ ] Documentation is complete and accurate

---

## Load Testing

### Scenario: Sustained Load (1 hour)

**Purpose**: Validate no memory leaks, stable performance

**Setup**:
- 3 workers
- Queue 100 tasks (mix of short/medium/long files)
- Tasks queued at rate of 2/minute

**Script**:
```bash
#!/bin/bash
# test/load/sustained-load.sh

DURATION=3600  # 1 hour
RATE=30        # Seconds between tasks

for i in $(seq 1 120); do
  FILE="test/testdata/epic09/file_$(( RANDOM % 10 + 1 )).mp4"
  
  kubectl run -n subgen-test load-task-$i --image=curlimages/curl --rm -i --restart=Never -- \
    curl -X POST http://subgen-orchestrator:9000/batch -d "{\"path\":\"$FILE\"}"
  
  sleep $RATE
done
```

**Validation**:
- Memory usage stays stable (< 10% growth)
- Queue never fills up
- All tasks complete successfully
- No errors in logs

---

### Scenario: Burst Load

**Purpose**: Test queue handling under sudden spike

**Setup**:
- 3 workers
- Queue 50 tasks simultaneously
- Monitor queue size and processing time

**Script**:
```bash
#!/bin/bash
# test/load/burst-load.sh

for i in $(seq 1 50); do
  FILE="test/testdata/epic09/short_$(( RANDOM % 5 + 1 )).mp4"
  
  kubectl run -n subgen-test burst-task-$i --image=curlimages/curl --rm -i --restart=Never -- \
    curl -X POST http://subgen-orchestrator:9000/batch -d "{\"path\":\"$FILE\"}" &
done

wait

echo "All tasks queued, monitoring completion..."
# Wait for queue to drain
while true; do
  QUEUE_SIZE=$(kubectl run -n subgen-test check-queue --image=curlimages/curl --rm -i --restart=Never -- \
    curl -s http://subgen-orchestrator:9000/queue | jq -r '.size')
  
  if [[ "$QUEUE_SIZE" == "0" ]]; then
    break
  fi
  
  echo "Queue size: $QUEUE_SIZE"
  sleep 10
done

echo "✅ Burst load test completed"
```

---

## Failure Scenario Testing

### Scenario 1: K8s API Unavailable

**Simulate**:
```bash
# Block orchestrator's access to K8s API
kubectl create netpolicy -n subgen-test block-k8s-api --pod-selector app=orchestrator --deny-ingress --deny-egress
```

**Expected**:
- Orchestrator logs "K8s API unavailable, using cached workers"
- Tasks continue to be dispatched to cached workers
- Degraded mode metric = 1

**Restore**:
```bash
kubectl delete netpolicy block-k8s-api -n subgen-test
```

---

### Scenario 2: All Workers Fail

**Simulate**:
```bash
kubectl scale statefulset subgen-worker --replicas=0 -n subgen-test
```

**Expected**:
- Orchestrator `/ready` returns 503
- New webhook requests are accepted but tasks queue
- No crashes or panics

**Restore**:
```bash
kubectl scale statefulset subgen-worker --replicas=3 -n subgen-test
```

---

### Scenario 3: Slow Worker

**Simulate**:
```bash
# Inject artificial delay in one worker
kubectl exec -n subgen-test subgen-worker-0 -- \
  sh -c 'echo "time.sleep(30)" >> /app/transcribe.py'
```

**Expected**:
- Least-loaded strategy avoids slow worker
- Other workers handle majority of load
- Slow worker eventually completes tasks

---

## Test Execution Schedule

### Story Implementation

| Story | Unit Tests | Integration Tests | E2E Tests |
|-------|-----------|------------------|-----------|
| STORY_01 | During dev | After story done | N/A |
| STORY_02 | During dev | After story done | N/A |
| STORY_03 | During dev | After story done | After STORY_03 |
| STORY_04 | N/A | N/A | After STORY_04 |
| STORY_05 | N/A | N/A | After STORY_05 |
| STORY_06A | During dev | After story done | After STORY_06A |
| STORY_06B | During dev | After story done | After STORY_06B |

### Epic Validation

**After all stories complete**:
1. Run full unit test suite (Go + Python)
2. Run all integration tests (Docker Compose)
3. Run all E2E tests (Kind)
4. Run load tests (1 hour sustained + burst)
5. Run failure scenario tests

**Total estimated testing time**: 3-4 hours

---

## Continuous Integration

### GitHub Actions Workflow

**File**: `.github/workflows/test-epic09.yml`

```yaml
name: Epic 9 Tests

on:
  pull_request:
    paths:
      - 'orchestrator/internal/discovery/**'
      - 'worker/src/http_server.py'
      - 'test/e2e/epic09/**'

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      # Go tests
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - name: Run orchestrator tests
        run: |
          cd orchestrator
          go test -v -race -cover ./internal/discovery/...
      
      # Python tests
      - uses: actions/setup-python@v5
        with:
          python-version: '3.11'
      - name: Run worker tests
        run: |
          cd worker
          pip install -r requirements.txt -r requirements-dev.txt
          pytest tests/test_http_server.py -v --cov=src/http_server
  
  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Kind cluster
        uses: helm/kind-action@v1
        with:
          cluster_name: subgen-test
          node_image: kindest/node:v1.28.0
      
      - name: Run E2E tests
        run: |
          chmod +x test/e2e/epic09/*.sh
          test/e2e/epic09/test-worker-discovery.sh
          test/e2e/epic09/test-load-balancing-roundrobin.sh
          test/e2e/epic09/test-dynamic-scaling.sh
```

---

## Summary

### Test Coverage

- ✅ **Unit Tests**: 85%+ coverage (Go + Python)
- ✅ **Integration Tests**: All major features
- ✅ **E2E Tests**: 5 comprehensive scenarios
- ✅ **Load Tests**: Sustained + burst scenarios
- ✅ **Failure Tests**: 3 major failure modes

### Estimated Testing Effort

- **Unit tests**: 8-10 hours (during story development)
- **Integration tests**: 4-6 hours (after each story)
- **E2E tests**: 6-8 hours (after all stories)
- **Load tests**: 2-3 hours (final validation)
- **Total**: 20-27 hours

---

**Document Status**: ✅ Final  
**Ready for Execution**: Yes  
**Prerequisites**: Kind/Minikube installed, test data generated
