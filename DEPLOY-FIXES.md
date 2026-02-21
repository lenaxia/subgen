# Deploying Language Detection and ASR Options Fixes

## Summary of Fixes

### 1. Language Detection Bug Fix
**Problem**: Language detection endpoint was broken due to orchestrator writing uploaded files to `/tmp` (not accessible to workers in separate pods).

**Root Cause**: Orchestrator's `DetectLanguage` method only accepted `filePath`, not `audioContent`, causing temp file race conditions.

**Solution**: Updated orchestrator to pass audio content directly via gRPC (consistent with ASR endpoint).

**Files Modified**:
- `orchestrator/internal/grpc_client/client.go` - Updated `DetectLanguage` to accept `audioContent []byte`
- `orchestrator/internal/webhooks/server.go` - Updated `GRPCClientInterface`
- `orchestrator/internal/webhooks/detect_language.go` - Rewrote handler to read audio content directly
- `orchestrator/internal/webhooks/detect_language_test.go` - Updated tests

### 2. ASR Advanced Options Fix
**Problem**: Advanced Whisper options (`word_level_highlight`, `custom_regroup`, etc.) weren't being passed from ASR endpoint to worker.

**Solution**: Added parsing of all ASR options from form data and passed them to gRPC `TranscribeOptions`.

**Files Modified**:
- `orchestrator/internal/webhooks/server.go` - Added parsing of ASR options
- `orchestrator/internal/grpc_client/client.go` - Added logic to pass ASROptions

## Current State (v0.2.10)
- ✅ Basic ASR transcription works
- ✅ Multi-worker deployment works (2 workers)
- ✅ NFS mount access works
- ✅ Batch scanning works (with `recursive=true`)
- ✅ Queue management works
- ❌ **Language detection broken** (returns empty results)
- ❌ **Advanced options not propagated** to workers

## Deployment Instructions

### Option 1: Rebuild and Deploy New Image
```bash
# 1. Build new orchestrator image
cd /home/ubuntu/workspace/subgen
docker build -t ghcr.io/lenaxia/subgen-orchestrator:v0.2.11-fixed -f orchestrator/Dockerfile .

# 2. Push to registry (requires authentication)
docker push ghcr.io/lenaxia/subgen-orchestrator:v0.2.11-fixed

# 3. Update Kubernetes deployment
kubectl set image deployment/subgen-orchestrator orchestrator=ghcr.io/lenaxia/subgen-orchestrator:v0.2.11-fixed -n default
```

### Option 2: Patch Existing Deployment
Create patch file `orchestrator-patch.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: subgen-orchestrator
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: orchestrator
        image: YOUR_NEW_IMAGE_TAG_HERE
        imagePullPolicy: Always
```

Apply patch:
```bash
kubectl patch deployment subgen-orchestrator --patch-file orchestrator-patch.yaml -n default
```

### Option 3: Manual Binary Update (Development)
```bash
# 1. Build binary locally
cd orchestrator
go build -mod=vendor -o orchestrator ./cmd/orchestrator

# 2. Copy to pod (temporary)
kubectl cp orchestrator default/$(kubectl get pods -l app=subgen-orchestrator -o name | head -1 | cut -d/ -f2):/usr/local/bin/orchestrator -c orchestrator

# 3. Restart pod
kubectl rollout restart deployment/subgen-orchestrator -n default
```

## Testing After Deployment

### 1. Test Language Detection Fix
```bash
curl -X POST "http://<orchestrator-ip>:9000/detect-language?offset=0&length=30" \
  -F "audio_file=@test_audio.wav" \
  -H "Content-Type: multipart/form-data"

# Expected response (after fix):
# {"language":"English","code":"en","confidence":0.95}
```

### 2. Test ASR Advanced Options
```bash
curl -X POST "http://<orchestrator-ip>:9000/asr" \
  -F "audio_file=@test_audio.wav" \
  -F "word_level_highlight=true" \
  -F "custom_regroup=paragraph" \
  -F "max_line_width=42" \
  -F "max_line_count=2" \
  -H "Content-Type: multipart/form-data"

# Expected: Task created with advanced options passed to worker
```

### 3. Test Batch Scanning
```bash
curl -X POST "http://<orchestrator-ip>:9000/batch" \
  -F "path=/media/test" \
  -F "recursive=true" \
  -F "limit=10" \
  -H "Content-Type: multipart/form-data"
```

## API Documentation Updates

### Language Detection Endpoint
- **URL**: `POST /detect-language`
- **Parameters**:
  - `audio_file` (multipart): Audio file to analyze
  - `offset` (query): Start offset in seconds (default: 0)
  - `length` (query): Length to analyze in seconds (default: 30)
- **Response**: `{"language":"English","code":"en","confidence":0.95}`

### ASR Endpoint with Advanced Options
- **URL**: `POST /asr`
- **Parameters**:
  - `audio_file` (multipart): Audio file to transcribe
  - `word_level_highlight` (form): Enable word-level timestamps (true/false)
  - `custom_regroup` (form): Regrouping strategy (paragraph/sentence/none)
  - `max_line_width` (form): Maximum characters per line
  - `max_line_count` (form): Maximum lines per segment
  - `output` (query): Output format (json/srt/lrc)

### Batch Scanning
- **URL**: `POST /batch`
- **Parameters**:
  - `path` (form): Directory path to scan
  - `recursive` (form): Scan subdirectories (true/false, **required**)
  - `limit` (form): Maximum files to process (default: 100)

## Notes
1. **Language detection now works** without temp file race conditions
2. **Advanced Whisper options** are now properly passed to workers
3. **Batch scanning requires `recursive=true`** for nested directories
4. **Output format parameter** must be query param (not form data)
5. **No progress reporting** implemented yet (only queue status available)