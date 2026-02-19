# Work Log: Production Live Test Scenarios
## Date: February 19, 2026
## Epic: Production Readiness & Validation

## Executive Summary

Comprehensive list of production live test scenarios for Subgen system validation. Includes scenarios from previous testing, new scenarios for multi-worker distribution, and end-to-end validation tests. Organized by priority and complexity for systematic production deployment validation.

## Test Categories

### Category 1: Core Functionality (Priority: Critical)
### Category 2: Multi-Worker Distribution (Priority: High)  
### Category 3: Performance & Load (Priority: Medium)
### Category 4: Edge Cases & Failure Modes (Priority: Medium)
### Category 5: Integration & Observability (Priority: Low)

## Category 1: Core Functionality Tests

### Scenario 1.1: Basic File Monitoring & Transcription
**Objective**: Validate end-to-end file monitoring pipeline
**Test Steps**:
1. Place test media file in monitored directory (`/media`)
2. Verify orchestrator detects file creation event (< 1s)
3. Verify file stability check (6s wait, 3 checks)
4. Verify task queued with correct priority
5. Verify worker picks up task
6. Verify transcription completes
7. Verify subtitle file generated in correct location
8. Verify LRC/SRT file contains correct transcription

**Success Criteria**:
- File detected within 1 second
- Task queued after stability check (6s)
- Transcription completes within expected time
- Subtitle file created with correct content
- Logs show complete workflow trace

**Previous Test Results**: ✅ PASS (Epic 7 validation)

### Scenario 1.2: Skip Logic - File Existence
**Objective**: Validate duplicate prevention
**Test Steps**:
1. Place media file with existing `.srt` subtitle
2. Place media file with existing `.lrc` subtitle  
3. Place media file without subtitles
4. Trigger batch scan or wait for monitoring
5. Verify files with subtitles are skipped
6. Verify file without subtitles is queued

**Success Criteria**:
- Files with existing subtitles show "skipped" in logs
- Files without subtitles show "queued" in logs
- Skip reasons logged correctly (`subtitle_file_exists`, `lrc_file_exists`)

**Previous Test Results**: ✅ PASS (Epic 6 validation)

### Scenario 1.3: ASR Endpoint with Byte Content
**Objective**: Validate ASR endpoint works with new byte content architecture
**Test Steps**:
1. Send POST to `/asr` endpoint with audio bytes
2. Include format parameter (`srt`, `lrc`, `vtt`, etc.)
3. Verify request accepted (202 status)
4. Verify transcription completes
5. Verify response contains subtitle content in requested format
6. Verify no temp files created in `/media` (byte content architecture)

**Success Criteria**:
- Request accepted within 1 second
- Transcription completes within expected time
- Response contains correct format subtitle
- No permission errors for `/media` directory
- Logs show byte content processing

**Current Status**: ⚠️ PARTIAL (ASR result channel bug, but byte content architecture fixed in v0.2.4)

### Scenario 1.4: Language Detection Endpoint
**Objective**: Validate standalone language detection
**Test Steps**:
1. Send POST to `/detect-language` with audio file
2. Test with English, Spanish, French audio samples
3. Verify response contains language code, name, confidence
4. Verify confidence > 0.5 for clear samples

**Success Criteria**:
- Correct language detected for clear samples
- Confidence scores reasonable (> 0.5 for clear audio)
- Response time < 10 seconds

**Previous Test Results**: ✅ PASS (Epic 8 validation)

## Category 2: Multi-Worker Distribution Tests

### Scenario 2.1: Worker Discovery & Health Checking
**Objective**: Validate Kubernetes worker discovery
**Test Steps**:
1. Deploy 3 worker pods in Kubernetes
2. Deploy orchestrator with worker discovery enabled
3. Verify orchestrator discovers all 3 workers
4. Verify health checks run every 30 seconds
5. Kill one worker pod
6. Verify orchestrator marks worker as unhealthy
7. Restart killed worker
8. Verify orchestrator rediscoveres and marks as healthy

**Success Criteria**:
- All workers discovered within 60 seconds
- Health status updates correctly
- Unhealthy workers removed from active pool
- Restarted workers rejoin pool

**Current Status**: ⚠️ PARTIAL (Tested with localhost discovery, not K8s)

### Scenario 2.2: Load Distribution Across Workers
**Objective**: Validate tasks distributed evenly
**Test Steps**:
1. Deploy 2 workers with different capacities (CPU vs GPU)
2. Queue 10 transcription tasks simultaneously
3. Monitor which worker processes each task
4. Verify tasks distributed across both workers
5. Verify no single worker overloaded
6. Check worker active task counts via metrics

**Success Criteria**:
- Tasks distributed to both workers
- GPU worker gets more compute-intensive tasks
- Active task counts balanced appropriately
- No worker exceeds capacity

**Test Metrics to Collect**:
- Tasks per worker distribution
- Worker utilization percentages
- Queue wait times
- Processing times by worker type

### Scenario 2.3: Worker Failure During Processing
**Objective**: Validate task recovery on worker failure
**Test Steps**:
1. Queue 5 transcription tasks
2. Kill worker while processing task #3
3. Verify orchestrator detects worker failure
4. Verify failed task re-queued
5. Verify another worker picks up re-queued task
6. Verify all 5 tasks eventually complete

**Success Criteria**:
- Worker failure detected within 30 seconds
- Failed task re-queued with same priority
- Task completed by another worker
- No data loss or corruption

### Scenario 2.4: Worker Scaling Under Load
**Objective**: Validate auto-scaling response
**Test Steps**:
1. Start with 1 worker
2. Queue 20 concurrent transcription tasks
3. Monitor queue backlog growth
4. Scale to 3 workers (manual or auto-scale)
5. Verify backlog reduces as new workers join
6. Verify new workers discovered and added to pool
7. Verify load redistributes to new workers

**Success Criteria**:
- New workers discovered within 60 seconds
- Tasks redistributed to new workers
- Queue backlog reduces after scaling
- No tasks stuck or orphaned

### Scenario 2.5: Mixed Worker Types (CPU vs GPU)
**Objective**: Validate heterogeneous worker pool
**Test Steps**:
1. Deploy 1 GPU worker, 2 CPU workers
2. Queue mix of task types (transcribe, detect-language)
3. Verify GPU worker gets transcription tasks
4. Verify CPU workers get language detection tasks
5. Queue high-priority transcription task
6. Verify GPU worker gets priority task even if busy

**Success Criteria**:
- Task type to worker type affinity working
- Priority tasks handled appropriately
- Worker utilization optimized by type

## Category 3: Performance & Load Tests

### Scenario 3.1: Concurrent Request Handling with Multi-Worker Scaling
**Objective**: Validate system under concurrent load with worker pool scaling
**Test Steps**:
1. Start with 2 workers (1 CPU, 1 GPU)
2. Send 30 concurrent ASR requests with varying audio durations (30s-5min)
3. Send 10 concurrent batch scan requests to different directories
4. Simulate 50 concurrent file monitoring events
5. Monitor worker utilization and task distribution
6. Scale to 4 workers (2 CPU, 2 GPU) during peak load
7. Measure response times at each concurrency level
8. Verify load redistributes to new workers

**Success Criteria**:
- All requests complete successfully
- Tasks distributed across all available workers
- GPU workers handle compute-intensive tasks
- Response times improve with worker scaling
- Error rate < 1% under peak load

**Load Levels to Test**:
- Baseline: 5 concurrent requests with 2 workers
- Medium: 20 concurrent requests with 2 workers  
- High: 50 concurrent requests with 4 workers
- Peak: 100 concurrent requests with 6 workers
- Stress: 200 concurrent requests with 8 workers

### Scenario 3.2: Large File Processing with Memory Constraints
**Objective**: Validate memory management with large files across worker pool
**Test Steps**:
1. Deploy workers with memory limits (2GB, 4GB, 8GB)
2. Process 2-hour movie file (4GB video) on 8GB worker
3. Process 5-hour audiobook (2GB audio) on 4GB worker
4. Process 10-hour podcast series (multiple files) across workers
5. Monitor memory usage per worker during processing
6. Test memory oversubscription (try to process 3GB file on 2GB worker)
7. Verify task reassignment when worker memory exhausted
8. Measure processing time vs file size vs worker memory

**Success Criteria**:
- Large files processed successfully on appropriately sized workers
- Memory usage stays within limits
- Memory oversubscription handled gracefully (task fails, not worker crashes)
- Processing time scales with worker capacity
- No memory leaks across multiple large file processing

### Scenario 3.3: Sustained Load Endurance with Worker Failover
**Objective**: Validate system stability over time with worker failures
**Test Steps**:
1. Run continuous load (2 requests/minute) for 72 hours
2. Mix of request types (ASR, batch, monitoring, language detection)
3. Randomly restart 1 worker every 6 hours
4. Scale worker count up/down every 12 hours (2→4→2→3)
5. Monitor system metrics continuously
6. Check for memory leaks, connection leaks, file descriptor leaks
7. Verify all requests successful despite worker churn
8. Analyze performance consistency over time

**Success Criteria**:
- >99.5% success rate over 72 hours
- Memory usage stable (±15%) despite worker churn
- No resource leaks (connections, file handles, goroutines)
- Performance consistent (±20%) despite scaling events
- Automatic recovery from worker failures within 60 seconds

### Scenario 3.4: Mixed Workload Performance
**Objective**: Validate performance with realistic mixed workloads
**Test Steps**:
1. Simulate typical production pattern: 70% ASR, 20% batch, 10% language detection
2. Run for 8 hours with varying load (low morning, high evening)
3. Include periodic large file processing (every 2 hours)
4. Monitor worker specialization (GPU for ASR, CPU for language detection)
5. Measure end-to-end latency for each request type
6. Verify no request type starves others
7. Test priority handling (urgent ASR requests vs background batch)

**Success Criteria**:
- All request types processed within SLA
- Worker specialization effective (GPU utilization > 60% for ASR)
- No request starvation or priority inversion
- Mixed workload throughput matches sum of individual capacities
- Resource utilization balanced across worker types

## Category 4: Edge Cases & Failure Modes

### Scenario 4.1: Invalid Input Handling Across All APIs
**Objective**: Validate graceful error handling for all endpoints
**Test Steps**:
1. **ASR Endpoint**:
   - Invalid audio format (non-audio file)
   - Empty byte content
   - Corrupted audio (header mismatch)
   - Audio too large (>100MB limit)
   - Missing required parameters
   - Invalid format parameter (non-existent format)
   
2. **Batch Endpoint**:
   - Non-existent directory
   - Directory without read permissions
   - Invalid recursive parameter (non-boolean)
   - Missing directory parameter
   
3. **Language Detection**:
   - Non-audio file
   - Empty file
   - Corrupted audio
   
4. **Queue Endpoints**:
   - Invalid limit parameter (negative, too large)
   - Invalid offset parameter
   
5. **Health/Ready Endpoints**:
   - Test during startup (should return 503)
   - Test during shutdown (should return 503)

**Success Criteria**:
- All invalid requests return appropriate HTTP codes (400, 422, 503)
- Descriptive, actionable error messages
- No system crashes or instability
- Logs contain structured error details
- Error responses include request ID for correlation

### Scenario 4.2: Filesystem Edge Cases with Multi-Worker Coordination
**Objective**: Validate handling of filesystem issues across distributed workers
**Test Steps**:
1. **Permission Issues**:
   - Worker tries to write to read-only shared volume
   - Different workers have different permissions to same directory
   - Permission changes during file processing
   
2. **File State Changes**:
   - File deleted while being processed by worker
   - File modified (size/content) during processing
   - File moved/renamed during processing
   - File becomes symlink during processing
   
3. **Storage Issues**:
   - Disk full during subtitle writing
   - Disk I/O errors during read/write
   - Network storage (NFS) disconnection
   - Storage latency spikes causing timeouts
   
4. **Concurrent Access**:
   - Multiple workers try to process same file
   - Worker tries to read file being written by another process
   - File locked by external application

**Success Criteria**:
- Graceful error handling with appropriate logging
- Failed tasks marked appropriately in queue
- No data corruption from partial writes
- Automatic retry for transient errors
- Clear error messages indicating filesystem issue

### Scenario 4.3: Network Partition & Recovery with Multi-Worker
**Objective**: Validate network failure handling in distributed system
**Test Steps**:
1. **Partial Network Partition**:
   - Orchestrator loses connection to 1 worker (of 3)
   - Worker loses connection to shared storage
   - Workers can't communicate with each other
   
2. **Complete Network Partition**:
   - Orchestrator isolated from all workers
   - Workers isolated from each other
   - Split-brain scenario (workers think orchestrator dead)
   
3. **Network Degradation**:
   - High latency (500ms+) between components
   - Packet loss (5%+) causing retransmissions
   - Bandwidth saturation
   
4. **DNS/Service Discovery Issues**:
   - DNS resolution failures
   - Service registry inconsistencies
   - IP address changes during runtime

**Success Criteria**:
- Workers marked unhealthy within 30 seconds of partition
- Tasks reassigned when worker unreachable
- No duplicate processing when network restored
- Automatic reconnection when network restored
- System returns to normal operation without manual intervention

### Scenario 4.4: Resource Exhaustion with Worker Pool
**Objective**: Validate behavior under resource constraints across worker pool
**Test Steps**:
1. **Memory Exhaustion**:
   - Worker hits memory limit during large file processing
   - Multiple workers hit memory limits simultaneously
   - Memory leak causes gradual exhaustion
   
2. **CPU Exhaustion**:
   - Worker at 100% CPU for extended period
   - CPU throttling (cgroups) impact on performance
   - CPU affinity issues with mixed workloads
   
3. **Disk I/O Exhaustion**:
   - High disk I/O causing timeouts
   - Disk queue saturation
   - Concurrent file operations causing lock contention
   
4. **Network Resource Exhaustion**:
   - Connection pool exhaustion
   - Bandwidth saturation affecting control plane
   - Port exhaustion from too many connections

**Success Criteria**:
- Graceful degradation not crashes
- Resource exhaustion logged with clear metrics
- System remains operable for lighter functions
- Automatic recovery when resources freed
- Alerting triggers before critical exhaustion

### Scenario 4.5: Configuration Edge Cases
**Objective**: Validate handling of configuration issues
**Test Steps**:
1. **Invalid Configuration**:
   - Invalid worker discovery configuration
   - Invalid queue configuration (negative sizes)
   - Invalid timeout values (too short/long)
   - Invalid directory paths
   
2. **Configuration Changes**:
   - Configuration reload during operation
   - Configuration file corruption
   - Environment variable conflicts
   - Configuration precedence issues
   
3. **Missing Dependencies**:
   - Missing model files
   - Missing external binaries (ffmpeg, etc.)
   - Missing shared libraries
   
4. **Security Configuration**:
   - Invalid TLS certificates
   - Expired credentials
   - Insufficient permissions from config

**Success Criteria**:
- Invalid configuration rejected at startup
- Configuration changes applied correctly
- Missing dependencies handled gracefully
- Security issues logged appropriately
- Clear error messages for configuration problems

## Category 5: Integration & Observability

### Scenario 5.1: API Endpoint Validation with Multi-Worker Context
**Objective**: Validate all HTTP endpoints provide accurate multi-worker context
**Test Steps**:
1. **Health & Readiness**:
   - `/health` - returns orchestrator status, uptime, version
   - `/ready` - returns readiness based on minimum worker count
   - Verify endpoints reflect worker pool health
   
2. **Queue & Status Endpoints**:
   - `/queue/status` - returns queue metrics across all workers
   - `/queue/processing` - returns active tasks per worker
   - `/queue/history` - returns completed tasks with worker assignment
   - Verify data includes worker-specific metrics
   
3. **Worker Management Endpoints**:
   - `/workers` - returns list of all workers with health status
   - `/workers/{id}/stats` - returns detailed worker statistics
   - `/workers/{id}/tasks` - returns tasks assigned to specific worker
   
4. **Functional Endpoints**:
   - `/asr` - verify response includes processing worker ID
   - `/batch` - verify batch progress includes worker assignments
   - `/detect-language` - verify language detection worker metrics
   
5. **Performance Endpoints**:
   - `/metrics` - Prometheus metrics with worker labels
   - `/debug/pprof` - performance profiling per component
   - `/debug/vars` - runtime variables

**Success Criteria**:
- All endpoints return correct HTTP codes
- JSON responses include multi-worker context
- Metrics accurately reflect distributed state
- Endpoints perform within expected time (< 100ms)
- Data consistency across related endpoints

### Scenario 5.2: Comprehensive Logging & Monitoring
**Objective**: Validate observability stack for distributed system
**Test Steps**:
1. **Structured Logging**:
   - Verify logs contain structured JSON with consistent fields
   - Test log levels (debug, info, warn, error) for different events
   - Verify correlation IDs flow across components
   - Test log aggregation (Loki/Fluentd) collects all logs
   
2. **Metrics Collection**:
   - Verify Prometheus metrics exposed by all components
   - Test metric labels include worker_id, task_type, etc.
   - Verify histogram metrics for latency distributions
   - Test counter metrics for request counts, errors
   - Verify gauge metrics for queue depth, active workers
   
3. **Distributed Tracing**:
   - Verify trace propagation across orchestrator→worker
   - Test trace includes timing for each processing stage
   - Verify trace context includes in logs (correlation)
   - Test sampling rates appropriate for production
   
4. **Alerting & Notifications**:
   - Test alert rules fire on error conditions
   - Verify alert includes relevant context (which worker, what failed)
   - Test alert suppression to avoid noise
   - Verify notification channels work (Slack, PagerDuty, email)
   
5. **Dashboard & Visualization**:
   - Verify Grafana dashboards show multi-worker metrics
   - Test dashboard refresh rates and query performance
   - Verify key metrics visible at a glance
   - Test drill-down from system view to worker view

**Success Criteria**:
- All components emit structured logs
- Metrics cover all critical business and technical aspects
- Tracing shows complete request flow across components
- Alerts fire on true issues with minimal false positives
- Dashboards provide actionable insights

### Scenario 5.3: Configuration Management & Validation
**Objective**: Validate configuration handling for distributed deployment
**Test Steps**:
1. **Configuration Sources**:
   - Test configuration from file (YAML, JSON)
   - Test environment variable overrides
   - Test Kubernetes ConfigMap/Secret integration
   - Test configuration precedence (env > file > defaults)
   
2. **Dynamic Configuration**:
   - Test configuration reload without restart
   - Verify changed configuration applied correctly
   - Test partial configuration updates
   - Verify configuration validation on reload
   
3. **Multi-Component Configuration**:
   - Test shared configuration (Redis, storage, etc.)
   - Verify component-specific configuration isolation
   - Test configuration consistency across replicas
   - Verify configuration versioning and rollback
   
4. **Security Configuration**:
   - Test TLS configuration (certificates, cipher suites)
   - Verify authentication/authorization configuration
   - Test secret management (Vault, Kubernetes Secrets)
   - Verify security configuration validation
   
5. **Performance Configuration**:
   - Test worker pool configuration (min/max workers)
   - Verify queue configuration (size, priorities)
   - Test timeout configuration (connection, processing)
   - Verify resource limit configuration (CPU, memory)

**Success Criteria**:
- Configuration loaded correctly from all sources
- Dynamic configuration changes applied safely
- Configuration validation catches errors early
- Security configuration properly enforced
- Performance configuration optimizes resource usage

### Scenario 5.4: External Integration Points
**Objective**: Validate integration with external systems
**Test Steps**:
1. **Media Server Integrations**:
   - Test Plex webhook integration
   - Test Jellyfin/Emby integration
   - Test Tautulli integration
   - Verify path mapping works correctly
   
2. **Storage Integrations**:
   - Test local filesystem integration
   - Test NFS/SMB network storage
   - Test cloud storage (S3, GCS, Azure Blob)
   - Verify storage performance and reliability
   
3. **Message Queue Integration**:
   - Test Redis queue integration
   - Verify message persistence and delivery
   - Test queue failure and recovery
   - Verify message ordering and priority
   
4. **Model Service Integration**:
   - Test Whisper model loading
   - Verify model version compatibility
   - Test model fallback and recovery
   - Verify model performance metrics

**Success Criteria**:
- All external integrations work reliably
- Integration failures handled gracefully
- Performance acceptable for production use
- Monitoring covers integration points
- Documentation exists for integration configuration

## Test Environment Requirements

### Hardware Requirements
- **Orchestrator**: 2 CPU, 2GB RAM minimum
- **Worker (CPU)**: 4 CPU, 4GB RAM minimum  
- **Worker (GPU)**: 4 CPU, 8GB RAM, NVIDIA GPU
- **Storage**: 50GB for media files, models
- **Network**: 1Gbps between components

### Software Requirements
- **Kubernetes**: v1.25+ with CNI networking
- **Docker**: 20.10+ with nvidia-docker for GPU
- **Monitoring**: Prometheus, Grafana, Loki
- **Load Testing**: k6, vegeta, or custom scripts

### Test Data Requirements
- **Audio Samples**: 10+ files, various languages, durations 30s-5min
- **Video Samples**: 5+ files, various codecs, durations 1-10min
- **Large Files**: 2+ files >1GB for stress testing
- **Corrupted Files**: 2+ files with invalid formats

## Test Execution Plan

### Phase 1: Core Validation & Multi-Worker Foundation (Week 1-2)
1. Deploy basic 1+1 setup (1 orchestrator, 1 worker)
2. Run Category 1 tests (core functionality)
3. Scale to 1+3 setup (mixed CPU/GPU workers)
4. Run Category 2 tests (multi-worker distribution)
5. Fix any issues found
6. Document baseline performance

### Phase 2: Performance & Load Testing (Week 3-4)
1. Run Category 3.1 (concurrent request handling with scaling)
2. Run Category 3.2 (large file processing with memory constraints)
3. Run Category 3.3 (sustained load endurance with worker failover)
4. Run Category 3.4 (mixed workload performance)
5. Establish performance baselines for all scenarios
6. Identify and document bottlenecks
7. Optimize configuration based on findings

### Phase 3: Edge Case & Failure Mode Testing (Week 5-6)
1. Run Category 4.1 (invalid input handling across all APIs)
2. Run Category 4.2 (filesystem edge cases with multi-worker coordination)
3. Run Category 4.3 (network partition & recovery with multi-worker)
4. Run Category 4.4 (resource exhaustion with worker pool)
5. Run Category 4.5 (configuration edge cases)
6. Validate error handling and recovery mechanisms
7. Document failure modes and create runbooks
8. Implement monitoring alerts for critical failures

### Phase 4: Integration & Observability Testing (Week 7-8)
1. Run Category 5.1 (API endpoint validation with multi-worker context)
2. Run Category 5.2 (comprehensive logging & monitoring)
3. Run Category 5.3 (configuration management & validation)
4. Run Category 5.4 (external integration points)
5. Validate monitoring stack (Prometheus, Grafana, Loki)
6. Document operational procedures and troubleshooting guides
7. Finalize dashboards and alerting rules
8. Conduct operational readiness review

### Phase 5: End-to-End Validation & Sign-off (Week 9-10)
1. Run integrated scenarios combining all categories
2. Simulate 72-hour production-like workload
3. Conduct chaos engineering tests (random failures)
4. Perform security validation and penetration testing
5. Final performance benchmarking
6. Documentation review and completion
7. Stakeholder demonstration and sign-off
8. Production deployment readiness assessment

## Success Metrics

### Quantitative Metrics
#### Performance Metrics
- **Success Rate**: > 99.5% for all requests under normal load
- **Latency**: 
  - P95 < 30s for ASR requests (1min audio)
  - P95 < 5s for health/ready checks
  - P95 < 10s for language detection
  - P95 < 60s for batch directory scans (100 files)
  
- **Throughput**:
  - > 20 concurrent transcriptions sustained with 4 workers
  - > 50 concurrent mixed requests with 6 workers
  - > 100 RPS for monitoring/health endpoints
  
- **Scalability**:
  - Linear scaling up to 8 workers (80% efficiency)
  - Worker discovery < 60 seconds
  - Task redistribution < 30 seconds after scaling
  
- **Resource Usage**:
  - CPU < 80% under sustained load
  - Memory stable (±15%) over 72 hours
  - No resource leaks (connections, file handles, goroutines)

#### Multi-Worker Specific Metrics
- **Load Distribution**: Tasks evenly distributed (±25% across workers)
- **Worker Utilization**: 60-80% optimal range for each worker
- **Failure Recovery**: < 30s detection, < 60s task reassignment
- **Scaling Efficiency**: > 70% additional throughput per added worker

### Qualitative Metrics
- **Observability**: All issues detectable within 5 minutes via logs/metrics
- **Recoverability**: Automatic recovery from 95% of failure scenarios
- **Usability**: Clear, actionable error messages with troubleshooting guidance
- **Documentation**: Complete runbooks covering all operational scenarios
- **Monitoring**: Comprehensive dashboards showing system health at a glance
- **Alerting**: Minimal false positives, all critical issues alerted within 5 minutes
- **Operational Readiness**: Team can troubleshoot and resolve issues without escalation

## Risk Assessment

### High Risk Areas
1. **Memory leaks** under sustained load with multiple workers
2. **Worker discovery & coordination** failures in distributed Kubernetes environment
3. **Network partition** handling causing split-brain scenarios
4. **Large file processing** stability across worker pool with memory constraints
5. **Load distribution imbalances** causing worker starvation or overload
6. **Configuration consistency** across distributed components

### Medium Risk Areas
1. **Mixed workload performance** with CPU/GPU worker specialization
2. **Filesystem coordination** issues with shared storage across workers
3. **External integration failures** (media servers, storage systems)
4. **Monitoring gaps** in distributed tracing and metrics aggregation
5. **Security configuration** consistency across components

### Low Risk Areas
1. **Basic API functionality** (already validated in previous testing)
2. **Single-worker scenarios** (thoroughly tested in development)
3. **Simple error handling** for common invalid inputs

### Mitigation Strategies
#### Technical Mitigations
1. **Memory testing** with 72-hour endurance tests and memory limit enforcement
2. **Discovery validation** with multiple Kubernetes scenarios (pod restarts, node failures)
3. **Network failure simulation** using chaos engineering tools (kube-monkey, chaos-mesh)
4. **Large file processing** with progressive memory allocation and task checkpointing
5. **Load distribution monitoring** with real-time metrics and automatic rebalancing
6. **Configuration management** with versioning, validation, and rollback capabilities

#### Process Mitigations
1. **Progressive deployment** starting with low traffic, increasing gradually
2. **Feature flags** for new functionality to enable quick rollback
3. **Comprehensive monitoring** with dashboards visible to entire team
4. **Regular chaos testing** in staging environment to validate recovery procedures
5. **Documented runbooks** for all failure scenarios with step-by-step resolution
6. **Capacity planning** based on performance testing results with 30% headroom

#### Organizational Mitigations
1. **Cross-team training** on system architecture and troubleshooting
2. **On-call rotation** with escalation procedures
3. **Post-mortem process** for analyzing and learning from incidents
4. **Regular review** of monitoring and alerting effectiveness
5. **Capacity review meetings** to anticipate scaling needs

## Previous Test Results Reference

### Epic 6 (Skip Logic) - ✅ Core Working
- File existence checking working
- External subtitle scanning working
- Embedded subtitle detection partially tested
- Language-based filtering not tested

### Epic 7 (File Monitoring) - ✅ Fully Working
- Startup scan working
- File detection working (< 1s latency)
- Stability checking working (6s wait)
- End-to-end transcription working

### Epic 8 (Advanced Features) - ⚠️ Mostly Working
- Batch endpoint working
- Queue status endpoints working
- Health endpoints working
- ASR endpoint partially working (result channel bug)
- Multi-format output partially tested

## Gaps Identified from Previous Testing

### Test Coverage Gaps
1. **Multi-worker distribution** - Not tested
2. **Load testing** - Not performed
3. **Failure recovery** - Limited testing
4. **Kubernetes integration** - Basic testing only
5. **Mixed worker types** - Not tested

### Functional Gaps
1. **ASR result channel** - Bug needs fixing
2. **Language-based skip logic** - Needs real media testing
3. **Plex integration** - Not tested
4. **Path mapping** - Not tested

## Recommendations for Production Deployment

### Pre-Deployment Checklist
- [ ] All Category 1 tests passing
- [ ] Multi-worker distribution validated (Category 2)
- [ ] Performance baselines established (Category 3)
- [ ] Critical edge cases handled (Category 4)
- [ ] Monitoring operational (Category 5)

### Deployment Strategy
1. **Canary deployment**: Start with 10% traffic
2. **Blue-green**: Maintain old version for rollback
3. **Feature flags**: Control new functionality rollout
4. **Progressive exposure**: Increase load gradually

### Monitoring During Deployment
- **Business metrics**: Success rate, latency
- **System metrics**: CPU, memory, disk, network
- **Application metrics**: Queue depth, worker health
- **User metrics**: Error rates, satisfaction

## Conclusion

This comprehensive test plan provides complete coverage for Subgen production readiness validation. The expanded scenarios now include:

### Key Additions:
1. **Performance scenarios with multi-worker scaling** - Concurrent requests, large files, sustained load with worker failover
2. **Edge cases for distributed systems** - Invalid inputs, filesystem coordination, network partitions, resource exhaustion
3. **Integration & observability** - API endpoints, comprehensive logging, monitoring, configuration management

### Priority Order:
1. **Fix ASR result channel bug** - Critical blocker for core functionality
2. **Validate multi-worker distribution** (Category 2) - Foundation for scaling
3. **Perform comprehensive load testing** (Category 3) - Performance baselines
4. **Complete edge case validation** (Category 4) - Resilience testing
5. **Finalize monitoring & integration** (Category 5) - Operational readiness

### Expected Timeline: 8-10 weeks for complete validation
- **Phase 1-2**: 4 weeks (core + multi-worker + performance)
- **Phase 3-4**: 4 weeks (edge cases + integration)
- **Phase 5**: 2 weeks (end-to-end validation)

### Resource Requirements:
- **Test Environment**: Kubernetes cluster with 8+ nodes (mixed CPU/GPU)
- **Test Data**: Diverse media files, load testing scripts, monitoring stack
- **Team**: 2-3 engineers for test execution and analysis
- **Tools**: k6/vegeta for load testing, chaos-mesh for failure injection, full observability stack

### Success Criteria:
- **Pre-production**: All Category 1-3 tests passing with documented performance baselines
- **Production readiness**: All Category 1-5 tests passing with operational runbooks
- **Go/No-Go Decision**: Based on quantitative metrics meeting targets and qualitative assessment of operational readiness

This test plan provides a systematic, phased approach to validating Subgen in production-like conditions, ensuring reliability, performance, and operability at scale with multi-worker distribution.