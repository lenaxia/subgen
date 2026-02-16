# Subgen Hybrid Architecture - Docker Deployment

## Overview

This directory contains the Docker Compose configuration for running Subgen's hybrid Go + Python architecture.

## Architecture Components

### Orchestrator (Go)
- **Image**: `subgen-orchestrator`
- **Ports**: 
  - `9000`: Webhook endpoint (Plex/Jellyfin/Tautulli/Bazarr)
  - `9090`: Prometheus metrics
- **Responsibilities**:
  - Receives webhooks from media servers
  - Manages transcription queue with deduplication
  - Discovers and coordinates Python workers
  - Handles media server API interactions
  - Uploads completed subtitles

### Worker (Python)
- **Image**: `subgen-worker`
- **Port**: `50051` (gRPC, internal only)
- **Responsibilities**:
  - Whisper AI transcription only
  - Audio extraction and processing
  - Subtitle generation (SRT/LRC)
  - Model lifecycle management

## Quick Start

### 1. Configure Environment

Copy the example environment file:
```bash
cp .env.example .env
```

Edit `.env` and configure:
- Media server credentials (Plex or Jellyfin)
- Volume mappings for your media files
- Whisper model settings
- Processing preferences

### 2. Build Images

```bash
docker compose -f docker-compose.hybrid.yml build
```

**Note**: First build takes 5-10 minutes due to PyAV compilation.

### 3. Start Services

```bash
docker compose -f docker-compose.hybrid.yml up -d
```

### 4. Verify Health

```bash
# Check service status
docker compose -f docker-compose.hybrid.yml ps

# View logs
docker compose -f docker-compose.hybrid.yml logs -f

# Test webhook endpoint
curl http://localhost:9000/health

# Test metrics endpoint
curl http://localhost:9090/metrics
```

## Configuration

### Required Environment Variables

#### Media Server Configuration
```bash
# For Plex
MEDIA_SERVER_TYPE=plex
PLEX_URL=http://your-plex-server:32400
PLEX_TOKEN=your-plex-token

# For Jellyfin
MEDIA_SERVER_TYPE=jellyfin
JELLYFIN_URL=http://your-jellyfin-server:8096
JELLYFIN_API_KEY=your-api-key
```

#### Volume Mappings
```bash
TV=/path/to/tv/shows
MOVIES=/path/to/movies
MUSIC=/path/to/music
MODEL_PATH=./data/models  # Whisper model storage
```

### Optional Configuration

#### Whisper Settings
```bash
WHISPER_MODEL=base           # tiny, base, small, medium, large
WHISPER_DEVICE=cpu           # cpu or cuda
COMPUTE_TYPE=auto            # int8, float16, float32
```

#### Processing Settings
```bash
CONCURRENT_TRANSCRIPTIONS=2  # Max parallel transcriptions
PROC_ADDED_MEDIA=true       # Process on library addition
PROC_MEDIA_ON_PLAY=false    # Process on playback
SKIP_IF_INTERNAL_SUB_LANG=eng  # Skip if internal subs exist
```

#### Path Mapping (for remote workers)
```bash
USE_PATH_MAPPING=false
PATH_MAPPING_FROM=/tv
PATH_MAPPING_TO=/mnt/media/tv
```

## Scaling Workers

### Multiple Workers (Manual)

Scale to 3 workers:
```bash
docker compose -f docker-compose.hybrid.yml up -d --scale worker=3
```

**Important**: When scaling manually, you must configure worker addresses:
```bash
# In .env or environment
WORKER_ADDRESSES=worker-1:50051,worker-2:50051,worker-3:50051
```

### Kubernetes (Auto-Discovery)

For production deployments with automatic worker discovery, see:
- `deploy/values.yaml` - Kubernetes Helm values
- `docs/DESIGN/02_KUBERNETES_DEPLOYMENT.md` - K8s deployment guide

## Webhook Configuration

### Plex + Tautulli

In Tautulli, configure webhook notification agent:
- **Webhook URL**: `http://subgen-orchestrator:9000/tautulli`
- **Method**: POST
- **Triggers**: Playback Start, Library New
- **Data**: JSON payload (see Tautulli webhook docs)

### Jellyfin

In Jellyfin webhook plugin settings:
- **Webhook URL**: `http://subgen-orchestrator:9000/jellyfin`
- **Events**: Playback Start, Library Item Added
- **Content Type**: application/json

### Bazarr

In Bazarr settings:
- **Custom Post-Processing**: `http://subgen-orchestrator:9000/bazarr`

## Monitoring

### Prometheus Metrics

Available at `http://localhost:9090/metrics`:

```
# Queue metrics
subgen_queue_size
subgen_queue_processing
subgen_queue_completed_total
subgen_queue_failed_total

# Worker metrics
subgen_workers_discovered
subgen_workers_healthy

# Processing metrics
subgen_transcription_duration_seconds
subgen_transcription_errors_total
```

### Health Checks

Both services have built-in health checks:
```bash
# Orchestrator
docker compose -f docker-compose.hybrid.yml exec orchestrator /usr/local/bin/orchestrator --health

# Worker gRPC health
docker compose -f docker-compose.hybrid.yml exec worker python -c "import grpc; from pb import transcription_pb2_grpc, transcription_pb2; channel = grpc.insecure_channel('localhost:50051'); stub = transcription_pb2_grpc.TranscriptionServiceStub(channel); print(stub.HealthCheck(transcription_pb2.HealthCheckRequest()))"
```

## Troubleshooting

### Worker Not Discovered

Check orchestrator logs:
```bash
docker compose -f docker-compose.hybrid.yml logs orchestrator | grep -i worker
```

Ensure `WORKER_ADDRESSES` is set correctly in environment.

### Transcription Fails

Check worker logs for errors:
```bash
docker compose -f docker-compose.hybrid.yml logs worker
```

Common issues:
- Model not downloaded (first run downloads model)
- Insufficient memory (need RAM >= model size)
- FFmpeg errors (check media file is accessible)

### Permission Errors

Both containers run as non-root users (UID 568 for orchestrator, UID 1000 for worker).
Ensure volume permissions allow these UIDs to read media files and write subtitles.

### GPU Support

For NVIDIA GPU acceleration, uncomment the deploy section in worker service:
```yaml
deploy:
  resources:
    reservations:
      devices:
        - driver: nvidia
          count: 1
          capabilities: [gpu]
```

Also set:
```bash
WHISPER_DEVICE=cuda
```

## Development

### Local Testing

Run orchestrator unit tests:
```bash
cd orchestrator
go test ./...
```

Run worker unit tests:
```bash
cd worker
python -m pytest
```

### Rebuilding After Code Changes

```bash
# Rebuild specific service
docker compose -f docker-compose.hybrid.yml build orchestrator
docker compose -f docker-compose.hybrid.yml build worker

# Restart services
docker compose -f docker-compose.hybrid.yml up -d
```

### Viewing Real-Time Logs

```bash
# All services
docker compose -f docker-compose.hybrid.yml logs -f

# Specific service
docker compose -f docker-compose.hybrid.yml logs -f orchestrator
docker compose -f docker-compose.hybrid.yml logs -f worker
```

## Performance Tuning

### Concurrent Transcriptions

Adjust based on available CPU/GPU:
```bash
# Conservative (single-core or slow CPU)
CONCURRENT_TRANSCRIPTIONS=1

# Moderate (quad-core CPU)
CONCURRENT_TRANSCRIPTIONS=2

# Aggressive (8+ cores or GPU)
CONCURRENT_TRANSCRIPTIONS=4
```

### Model Selection

Model size vs. speed trade-off:
- `tiny` - Fastest, lowest quality (~1GB RAM)
- `base` - Good balance (~1GB RAM)
- `small` - Better quality (~2GB RAM)
- `medium` - High quality (~5GB RAM)
- `large` - Best quality (~10GB RAM)

### Worker Scaling

Scale workers based on queue depth:
- 1 worker: < 10 transcriptions/hour
- 2-3 workers: 10-30 transcriptions/hour
- 4+ workers: > 30 transcriptions/hour

## Migration from Monolithic Subgen

### Configuration Mapping

Old monolithic environment variables map to new hybrid architecture:

| Old Variable | New Orchestrator Variable | New Worker Variable |
|-------------|--------------------------|---------------------|
| `WHISPER_MODEL` | - | `WHISPER_MODEL` |
| `WHISPER_THREADS` | - | `GRPC_MAX_WORKERS` |
| `TRANSCRIBE_DEVICE` | - | `WHISPER_DEVICE` |
| `PLEXSERVER` | `PLEX_URL` | - |
| `PLEXTOKEN` | `PLEX_TOKEN` | - |
| `JELLYFINSERVER` | `JELLYFIN_URL` | - |
| `JELLYFINTOKEN` | `JELLYFIN_API_KEY` | - |
| `WEBHOOKPORT` | `WEBHOOK_PORT` | - |
| `CONCURRENT_TRANSCRIPTIONS` | `CONCURRENT_TRANSCRIPTIONS` | - |
| `MODEL_PATH` | - | `MODEL_PATH` |

### Volume Mapping

Volumes remain the same - both architectures need access to:
- Media files (read-only)
- Subtitle output locations (read-write)
- Model cache directory (read-write on worker)

## Stopping Services

```bash
# Stop services (keep containers)
docker compose -f docker-compose.hybrid.yml stop

# Stop and remove containers
docker compose -f docker-compose.hybrid.yml down

# Stop, remove containers, and delete volumes
docker compose -f docker-compose.hybrid.yml down -v
```

## Additional Resources

- **Architecture Design**: `docs/DESIGN/00_HYBRID_ARCHITECTURE.md`
- **gRPC Protocol**: `docs/DESIGN/01_GRPC_PROTOCOL.md`
- **LLM Development Guide**: `README-LLM.md`
- **Epic Planning**: `docs/BACKLOG/`
- **Work Logs**: `docs/WORKLOGS/`

## Support

For issues, questions, or contributions:
1. Check existing issues in the repository
2. Review work logs for implementation details
3. Consult design documents for architectural decisions
4. Create a new issue with detailed logs and configuration
