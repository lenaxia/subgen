# STORY_05: Load Balancing Testing & Validation

**Epic:** EPIC_09  
**Status:** Not Started  
**Assignee:** TBD  
**Effort:** 3-4 hours

---

## User Story

As a **DevOps engineer**,  
I want to **validate that both load balancing strategies work correctly**,  
So that **I can choose the best strategy for my workload**.

---

## Acceptance Criteria

- [ ] Round Robin strategy distributes tasks evenly
- [ ] Least Loaded strategy selects least busy worker
- [ ] Load distribution metrics accurate
- [ ] Mixed workload testing complete
- [ ] Performance comparison documented
- [ ] Strategy selection guide created
- [ ] Test scripts created for automation

---

## Test Scenarios

### Scenario 1: Round Robin with Even Workload

**Setup:**
- 3 workers (all idle)
- 9 tasks (equal duration)
- Strategy: `LOAD_BALANCE_STRATEGY=round_robin`

**Expected Result:**
- Worker 0: 3 tasks
- Worker 1: 3 tasks
- Worker 2: 3 tasks

**Test Script:**

```bash
#!/bin/bash
# test-round-robin-even.sh

ORCHESTRATOR_IP="192.168.1.100"

echo "Testing Round Robin with even workload..."

# Queue 9 tasks
for i in {1..9}; do
  curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
    -H "Content-Type: application/json" \
    -d "{\"path\":\"/media/test/sample${i}.mp4\"}" \
    -s -o /dev/null
  echo "Queued task $i"
  sleep 0.5
done

# Wait for all tasks to start
sleep 5

# Check distribution
echo ""
echo "Task distribution:"
curl -s http://${ORCHESTRATOR_IP}:9090/metrics | grep "subgen_worker_active_jobs"

# Expected output:
# subgen_worker_active_jobs{worker="subgen-worker-0"} 3
# subgen_worker_active_jobs{worker="subgen-worker-1"} 3
# subgen_worker_active_jobs{worker="subgen-worker-2"} 3
```

---

### Scenario 2: Round Robin with Uneven Start

**Setup:**
- 3 workers
- Worker 0: 2 tasks already running
- Worker 1: 1 task already running
- Worker 2: idle
- Queue 6 new tasks

**Expected Result:**
Round robin doesn't consider current load:
- Worker 0: 2 + 2 = 4 tasks
- Worker 1: 1 + 2 = 3 tasks
- Worker 2: 0 + 2 = 2 tasks

**Test Script:**

```bash
#!/bin/bash
# test-round-robin-uneven.sh

ORCHESTRATOR_IP="192.168.1.100"

# Pre-load workers
echo "Pre-loading workers..."
curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
  -d '{"path":"/media/test/long1.mp4"}' -s -o /dev/null &
curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
  -d '{"path":"/media/test/long2.mp4"}' -s -o /dev/null &
curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
  -d '{"path":"/media/test/long3.mp4"}' -s -o /dev/null &

sleep 3

echo "Current load:"
curl -s http://${ORCHESTRATOR_IP}:9090/metrics | grep "subgen_worker_active_jobs"

# Queue 6 more tasks
echo ""
echo "Queueing 6 additional tasks..."
for i in {1..6}; do
  curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
    -d "{\"path\":\"/media/test/sample${i}.mp4\"}" -s -o /dev/null
done

sleep 2

echo "Final load:"
curl -s http://${ORCHESTRATOR_IP}:9090/metrics | grep "subgen_worker_active_jobs"
```

---

### Scenario 3: Least Loaded with Even Workload

**Setup:**
- 3 workers (all idle)
- 9 tasks (equal duration)
- Strategy: `LOAD_BALANCE_STRATEGY=least_loaded`

**Expected Result:**
Similar to Round Robin (all workers equal load):
- Worker 0: 3 tasks
- Worker 1: 3 tasks
- Worker 2: 3 tasks

**Test Script:**

```bash
#!/bin/bash
# test-least-loaded-even.sh

# Same as test-round-robin-even.sh but with least_loaded strategy
# Result should be similar since all workers start with equal load
```

---

### Scenario 4: Least Loaded with Uneven Start

**Setup:**
- 3 workers
- Worker 0: 2 tasks (busy)
- Worker 1: 1 task (less busy)
- Worker 2: 0 tasks (idle)
- Queue 6 new tasks

**Expected Result:**
Least loaded strategy considers current load:
- Worker 0: 2 + 1 = 3 tasks (got 1 new task)
- Worker 1: 1 + 2 = 3 tasks (got 2 new tasks)
- Worker 2: 0 + 3 = 3 tasks (got 3 new tasks)

**Test Script:**

```bash
#!/bin/bash
# test-least-loaded-uneven.sh

ORCHESTRATOR_IP="192.168.1.100"

# Pre-load workers unevenly
echo "Pre-loading workers unevenly..."
curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
  -d '{"path":"/media/test/long1.mp4"}' -s -o /dev/null &
curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
  -d '{"path":"/media/test/long2.mp4"}' -s -o /dev/null &
curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
  -d '{"path":"/media/test/long3.mp4"}' -s -o /dev/null &

sleep 3

echo "Initial load:"
curl -s http://${ORCHESTRATOR_IP}:9090/metrics | grep "subgen_worker_active_jobs"
# Expected:
# subgen_worker_active_jobs{worker="subgen-worker-0"} 2
# subgen_worker_active_jobs{worker="subgen-worker-1"} 1
# subgen_worker_active_jobs{worker="subgen-worker-2"} 0

# Queue 6 more tasks
echo ""
echo "Queueing 6 tasks (should prefer idle/less busy workers)..."
for i in {1..6}; do
  curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
    -d "{\"path\":\"/media/test/sample${i}.mp4\"}" -s -o /dev/null
  sleep 0.5  # Small delay to let metrics update
done

sleep 2

echo "Final load (should be balanced):"
curl -s http://${ORCHESTRATOR_IP}:9090/metrics | grep "subgen_worker_active_jobs"
# Expected (all balanced):
# subgen_worker_active_jobs{worker="subgen-worker-0"} 3
# subgen_worker_active_jobs{worker="subgen-worker-1"} 3
# subgen_worker_active_jobs{worker="subgen-worker-2"} 3
```

---

### Scenario 5: Worker Failure During Processing

**Setup:**
- 3 workers
- Queue 10 tasks
- Kill worker-1 while processing

**Expected Result:**
- No new tasks go to worker-1 after health check fails
- Tasks redistribute to worker-0 and worker-2
- In-progress task on worker-1 fails gracefully

**Test Script:**

```bash
#!/bin/bash
# test-worker-failure.sh

ORCHESTRATOR_IP="192.168.1.100"

# Queue 10 tasks
echo "Queueing 10 tasks..."
for i in {1..10}; do
  curl -X POST http://${ORCHESTRATOR_IP}:9000/batch \
    -d "{\"path\":\"/media/test/long${i}.mp4\"}" -s -o /dev/null &
done

sleep 3

echo "Initial distribution:"
curl -s http://${ORCHESTRATOR_IP}:9090/metrics | grep "subgen_worker_active_jobs"

# Kill worker-1
echo ""
echo "Killing worker-1..."
kubectl delete pod subgen-worker-1 -n media

# Wait for health check to detect failure
sleep 35

echo "Distribution after worker-1 killed:"
curl -s http://${ORCHESTRATOR_IP}:9090/metrics | grep "subgen_worker_active_jobs"
# Expected:
# subgen_worker_active_jobs{worker="subgen-worker-0"} 5
# subgen_worker_active_jobs{worker="subgen-worker-1"} 0  (or metric absent)
# subgen_worker_active_jobs{worker="subgen-worker-2"} 5

echo "Healthy workers:"
curl -s http://${ORCHESTRATOR_IP}:9090/metrics | grep "subgen_worker_healthy"
# Expected:
# subgen_worker_healthy{worker="subgen-worker-0"} 1
# subgen_worker_healthy{worker="subgen-worker-1"} 0
# subgen_worker_healthy{worker="subgen-worker-2"} 1
```

---

## Performance Comparison

### Test Setup

- 3 workers (identical resources)
- 30 tasks (10 each: small, medium, large)
- Run with both strategies
- Measure: total completion time, task distribution

### Metrics to Collect

```bash
# Total tasks processed
subgen_tasks_completed_total{strategy="round_robin"}
subgen_tasks_completed_total{strategy="least_loaded"}

# Average task duration
histogram_quantile(0.5, 
  rate(subgen_task_duration_seconds_bucket{strategy="round_robin"}[5m])
)

# Worker utilization
avg_over_time(subgen_worker_active_jobs[5m])
```

### Expected Results

**Round Robin:**
- Pros: Simple, predictable, no overhead
- Cons: Doesn't adapt to worker performance differences
- Best for: Homogeneous workers, similar task durations

**Least Loaded:**
- Pros: Adapts to worker speed, better utilization
- Cons: Slight overhead checking active jobs
- Best for: Heterogeneous workers, varied task durations

---

## Strategy Selection Guide

### Create Documentation

File: `docs/DEPLOYMENT/load-balancing-strategies.md`

```markdown
# Load Balancing Strategies

## Round Robin

**How it works:** Distributes tasks in circular order (worker-0, worker-1, worker-2, repeat)

**Use when:**
- All workers have identical hardware
- Tasks have similar duration
- Simplicity is preferred
- Minimal overhead required

**Configuration:**
```yaml
env:
  LOAD_BALANCE_STRATEGY: "round_robin"
```

## Least Loaded

**How it works:** Selects worker with fewest active jobs

**Use when:**
- Workers have different hardware (CPU vs GPU)
- Tasks have varied duration (short vs long files)
- Maximum throughput required
- Workers may process at different speeds

**Configuration:**
```yaml
env:
  LOAD_BALANCE_STRATEGY: "least_loaded"
```

## Performance Comparison

| Scenario | Round Robin | Least Loaded |
|----------|-------------|--------------|
| Even workload | Similar | Similar |
| Uneven workload | Unbalanced | Balanced |
| Worker speed diff | Slower worker lags | Compensates |
| Overhead | Minimal | Low |

## Recommendation

**Default:** Use `least_loaded` for production

**Reason:** Better handles real-world scenarios (varied file sizes, worker restarts)

**Exception:** Use `round_robin` if all workers identical and tasks similar
```

---

## Automated Test Suite

Create `test/load-balancing/run-all-tests.sh`:

```bash
#!/bin/bash
set -e

ORCHESTRATOR_IP="${ORCHESTRATOR_IP:-192.168.1.100}"

echo "=========================================="
echo "Load Balancing Test Suite"
echo "=========================================="

# Test 1: Round Robin Even
echo ""
echo "Test 1: Round Robin with even workload"
./test-round-robin-even.sh

# Wait for tasks to complete
sleep 60

# Test 2: Round Robin Uneven
echo ""
echo "Test 2: Round Robin with uneven start"
./test-round-robin-uneven.sh

# Wait for tasks to complete
sleep 60

# Switch to least_loaded strategy
echo ""
echo "Switching to least_loaded strategy..."
kubectl set env deployment/subgen-orchestrator \
  LOAD_BALANCE_STRATEGY=least_loaded \
  -n media

sleep 10

# Test 3: Least Loaded Even
echo ""
echo "Test 3: Least Loaded with even workload"
./test-least-loaded-even.sh

# Wait for tasks to complete
sleep 60

# Test 4: Least Loaded Uneven
echo ""
echo "Test 4: Least Loaded with uneven start"
./test-least-loaded-uneven.sh

# Wait for tasks to complete
sleep 60

# Test 5: Worker Failure
echo ""
echo "Test 5: Worker failure handling"
./test-worker-failure.sh

echo ""
echo "=========================================="
echo "All tests complete!"
echo "=========================================="
```

---

## Validation Checklist

### Round Robin Strategy

- [ ] Distributes 9 tasks evenly across 3 workers (3 each)
- [ ] Doesn't consider current worker load
- [ ] Cycles through workers in order
- [ ] Metrics show round_robin selection count

### Least Loaded Strategy

- [ ] Selects idle worker first
- [ ] Distributes to least busy workers
- [ ] Balances load over time
- [ ] Adapts to worker speed differences
- [ ] Metrics show least_loaded selection count

### Worker Failure Handling

- [ ] Unhealthy worker receives no new tasks
- [ ] Tasks redistribute to healthy workers
- [ ] Health metrics updated (<60 seconds)
- [ ] No tasks lost

### Performance

- [ ] Both strategies complete 30 tasks successfully
- [ ] Least Loaded shows better balance with uneven workload
- [ ] Round Robin has slightly less overhead
- [ ] Worker metrics accurate and updated

---

## Files to Create

1. `test/load-balancing/test-round-robin-even.sh`
2. `test/load-balancing/test-round-robin-uneven.sh`
3. `test/load-balancing/test-least-loaded-even.sh`
4. `test/load-balancing/test-least-loaded-uneven.sh`
5. `test/load-balancing/test-worker-failure.sh`
6. `test/load-balancing/run-all-tests.sh`
7. `docs/DEPLOYMENT/load-balancing-strategies.md`

---

## Definition of Done

- [ ] All test scripts created and executable
- [ ] Round Robin tests passing (even + uneven workload)
- [ ] Least Loaded tests passing (even + uneven workload)
- [ ] Worker failure test passing
- [ ] Performance comparison documented
- [ ] Strategy selection guide created
- [ ] Automated test suite runs successfully
- [ ] Results documented in work log
- [ ] Metrics validated for accuracy
- [ ] Recommendation provided for default strategy

---

## Expected Outcomes

### Test Results Summary

```
Test 1: Round Robin Even
  ✅ Tasks distributed evenly (3/3/3)
  
Test 2: Round Robin Uneven
  ⚠️ Unbalanced distribution (4/3/2) - expected

Test 3: Least Loaded Even
  ✅ Tasks distributed evenly (3/3/3)
  
Test 4: Least Loaded Uneven
  ✅ Balanced over time (3/3/3) - good!

Test 5: Worker Failure
  ✅ No tasks to unhealthy worker
  ✅ Tasks redistributed (5/0/5)
```

### Recommendation

Based on testing, **recommend `least_loaded` as default** for production deployments.

---

**Story Owner:** TBD  
**Created:** 2026-02-17
