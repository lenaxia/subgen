# Subgen Worker Autoscaling Plan

## Current Status
- **v0.2.15 tag pushed** - Release workflow triggered (includes 120s language detection timeout fix)
- **Current deployment**: 2 worker pods (StatefulSet, OrderedReady) - Will migrate to Deployment
- **No autoscaling**: Fixed replica count

## Autoscaling Implementation

### Files Created:
1. **`deploy-working.yaml`** - Updated with Deployment instead of StatefulSet
2. **`hpa-worker.yaml`** - HorizontalPodAutoscaler configuration (targets Deployment)
3. **`test-autoscaling.sh`** - Test script with migration support

### Configuration Details:

#### HPA Settings:
- **Min replicas**: 1
- **Max replicas**: 10
- **Scale triggers**: CPU >70%, Memory >80%
- **Scale up**: Max 2 pods/minute, 1 minute cooldown
- **Scale down**: Max 1 pod/minute, 5 minute cooldown

#### Pod Disruption Budget:
- **Max unavailable**: 1 pod during disruptions
- **Ensures**: At least N-1 workers available during maintenance

## Deployment Steps

### Phase 1: Deploy v0.2.15
```bash
# Wait for GitHub Actions to complete
# Then update images:
kubectl set image deployment/subgen-orchestrator -n default orchestrator=ghcr.io/lenaxia/subgen-orchestrator:v0.2.15
kubectl set image deployment/subgen-worker -n default worker=ghcr.io/lenaxia/subgen-worker:v0.2.15-cpu
```

### Phase 2: Migrate to Deployment & Enable Autoscaling
```bash
# Migrate from StatefulSet to Deployment
kubectl apply -f deploy-working.yaml
kubectl rollout status deployment/subgen-worker

# Enable autoscaling
kubectl apply -f hpa-worker.yaml
```

### Phase 3: Test Scaling
```bash
# Monitor HPA
watch kubectl get hpa subgen-worker

# Monitor pods
watch kubectl get pods -l app=subgen,component=worker

# Generate load (if needed)
curl -X POST 'http://localhost:9000/batch?directory=/media&recursive=true&format=json'
```

## Scaling Behavior

### Scale Up (when busy):
1. CPU >70% OR Memory >80% for 1 minute
2. Adds up to 2 new workers
3. New workers load Whisper model (~30 seconds)
4. Orchestrator automatically discovers new workers

### Scale Down (when idle):
1. CPU <70% AND Memory <80% for 5 minutes
2. Removes up to 1 worker at a time
3. Ensures at least 1 worker remains
4. Active transcriptions complete before termination

## Considerations

### Worker Startup Time:
- **First-time model load**: ~30 seconds (downloads Whisper-small)
- **Subsequent starts**: ~10 seconds (cached model)
- **Readiness probe**: 30 second initial delay

### Resource Requirements:
- **Per worker**: 500m CPU request, 2Gi memory request
- **Cluster needs**: Ensure enough capacity for max 10 workers
- **Node affinity**: Consider spreading workers across nodes

### Monitoring:
```bash
# Check HPA status
kubectl describe hpa subgen-worker

# Check resource usage
kubectl top pods -l app=subgen,component=worker

# Check scaling events
kubectl get events --field-selector involvedObject.name=subgen-worker
```

## Future Enhancements

### Custom Metrics (Phase 2):
1. Expose queue length from orchestrator
2. Scale based on pending tasks
3. More intelligent than CPU/Memory alone

### KEDA Integration (Phase 3):
1. Event-driven scaling
2. Scale based on queue depth
3. Support for multiple metric sources

### Predictive Scaling:
1. Learn transcription patterns
2. Scale before expected load
3. Consider time-of-day patterns

## Troubleshooting

### Common Issues:
1. **Metrics server missing**: Install with `kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml`
2. **HPA not scaling**: Check resource requests/limits in StatefulSet
3. **Slow scaling**: Switch to `podManagementPolicy: Parallel`
4. **Thrashing**: Adjust stabilization windows in HPA behavior

### Verification:
```bash
# Verify HPA is working
kubectl get hpa subgen-worker -o yaml | grep -A5 -B5 "currentMetrics"

# Check scaling history
kubectl describe hpa subgen-worker | grep -A10 "Events:"

# Test with load
for i in {1..20}; do
  curl -X POST "http://localhost:9000/detect-language?offset=0&length=30" \
    -F "audio_file=@test_sample.mp3" \
    -H "Content-Type: multipart/form-data" &
done
```

## Next Steps

1. **Wait for v0.2.15 release** to complete
2. **Deploy updated images**
3. **Apply HPA configuration**
4. **Test scaling behavior**
5. **Monitor and adjust thresholds** as needed

## Rollback
```bash
# Remove HPA
kubectl delete hpa subgen-worker

# Revert to fixed replicas
kubectl scale statefulset subgen-worker --replicas=2
```