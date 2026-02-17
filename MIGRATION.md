# Migration Guide: Legacy to Split Architecture

## Overview

Subgen has migrated from a **monolithic Python application** to a **split architecture** with:
- **Go Orchestrator:** Handles webhooks, file monitoring, queue management, and media server integration
- **Python Worker:** Performs Whisper transcription via gRPC

This change improves:
- **Scalability:** Run multiple workers for parallel transcription
- **Resource Efficiency:** Separate CPU-bound orchestration from GPU-intensive transcription
- **Maintainability:** Independent component updates and restarts
- **Performance:** Better queue management and concurrent processing

## For End Users

### Old Setup (Deprecated)

```yaml
services:
  subgen:
    image: mccloud/subgen:latest
    ports:
      - "9000:9000"
    environment:
      - TRANSCRIBE_DEVICE=cuda
      - WHISPER_MODEL=medium
      - PLEX_SERVER=http://plex:32400
      - PLEX_TOKEN=your_token
    volumes:
      - /path/to/media:/media
      - ./models:/subgen/models
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

### New Setup (Current)

```yaml
version: '3.8'

services:
  orchestrator:
    image: ghcr.io/lenaxia/subgen-orchestrator:latest
    container_name: subgen-orchestrator
    restart: unless-stopped
    ports:
      - "9000:9000"
    environment:
      # Worker connection
      - WORKER_ADDRESS=worker:50051
      
      # Media server settings (same as before)
      - PLEX_SERVER=http://plex:32400
      - PLEX_TOKEN=your_token
      - JELLYFIN_SERVER=http://jellyfin:8096
      - JELLYFIN_TOKEN=your_jellyfin_token
      
      # File monitoring (same as before)
      - TRANSCRIBE_FOLDERS=/media/tv|/media/movies
      - MONITOR=true
      
      # All other environment variables work the same
      - PROCESS_ADDED_MEDIA=true
      - SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE=eng
      # ... etc
    volumes:
      - /path/to/media:/media  # Same path mapping as before
      - ./config:/config
    depends_on:
      - worker

  worker:
    image: ghcr.io/lenaxia/subgen-worker:latest  # For GPU
    # image: ghcr.io/lenaxia/subgen-worker:cpu   # For CPU-only
    container_name: subgen-worker
    restart: unless-stopped
    environment:
      # Transcription settings (same as before)
      - TRANSCRIBE_DEVICE=cuda
      - WHISPER_MODEL=medium
      - CONCURRENT_TRANSCRIPTIONS=2
      - COMPUTE_TYPE=auto
      - WHISPER_THREADS=4
    volumes:
      - ./models:/models  # Model storage
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

## Configuration Changes

### What Stays the Same

✅ All webhook endpoints (`/plex`, `/jellyfin`, `/emby`, `/tautulli`)  
✅ All environment variables for media server configuration  
✅ All skip logic and subtitle generation settings  
✅ Path mapping requirements (orchestrator must see same paths as media server)  
✅ Bazarr integration (point to orchestrator at `http://orchestrator:9000`)  
✅ Port 9000 for webhook/API access

### What's Different

#### New Environment Variables

**Orchestrator:**
- `WORKER_ADDRESS` - **Required.** Address of worker gRPC server (default: `localhost:50051`)
- Example: `worker:50051` for Docker Compose, `localhost:50051` for standalone

**Worker:**
- No new variables - uses same transcription settings as before

#### Moved Configuration

| Setting | Old Location | New Location |
|---------|--------------|--------------|
| Webhook handling | Monolithic container | Orchestrator |
| File monitoring | Monolithic container | Orchestrator |
| Queue management | Monolithic container | Orchestrator |
| Whisper transcription | Monolithic container | Worker |
| Model storage | `/subgen/models` | `/models` |

## Migration Steps

### Step 1: Backup Current Configuration

```bash
# Save your current docker-compose.yml
cp docker-compose.yml docker-compose.yml.backup

# Note your current environment variables
docker inspect subgen | grep -A 50 Env
```

### Step 2: Update Docker Compose

Replace your existing `docker-compose.yml` with the new two-service setup shown above.

**Key Changes:**
1. Replace `mccloud/subgen` image with `ghcr.io/lenaxia/subgen-orchestrator` + `ghcr.io/lenaxia/subgen-worker`
2. Add `WORKER_ADDRESS=worker:50051` to orchestrator environment
3. Move GPU resources from orchestrator to worker
4. Update model volume path from `/subgen/models` to `/models`

### Step 3: Migrate Environment Variables

Copy all your existing environment variables to the **orchestrator** service, except:
- `TRANSCRIBE_DEVICE` → goes to **worker**
- `WHISPER_MODEL` → goes to **worker**
- `CONCURRENT_TRANSCRIPTIONS` → goes to **worker**
- `WHISPER_THREADS` → goes to **worker**
- `COMPUTE_TYPE` → goes to **worker**

### Step 4: Update Volume Paths

**Old:**
```yaml
volumes:
  - ./models:/subgen/models
```

**New:**
```yaml
# Orchestrator
volumes:
  - /path/to/media:/media
  - ./config:/config

# Worker
volumes:
  - ./models:/models
```

### Step 5: Start New Services

```bash
# Stop old container
docker-compose down

# Start new architecture
docker-compose up -d

# Check logs
docker-compose logs -f orchestrator
docker-compose logs -f worker
```

### Step 6: Verify Operation

1. **Check orchestrator:** `curl http://localhost:9000/health`
2. **Check worker connection:** Look for "Worker connected" in orchestrator logs
3. **Test webhook:** Send a test webhook from your media server
4. **Verify transcription:** Check that subtitles are generated as expected

## Network Configuration

### Docker Compose (Recommended)

The `depends_on` clause ensures proper startup order. Services communicate via Docker network:
- Orchestrator calls worker at `worker:50051`
- No additional network configuration needed

### Separate Hosts

If running orchestrator and worker on different machines:

**Orchestrator:**
```yaml
environment:
  - WORKER_ADDRESS=192.168.1.100:50051  # Worker's IP
```

**Worker:**
```yaml
ports:
  - "50051:50051"  # Expose gRPC port
```

### Docker Bridge Network

If not using Docker Compose:
```bash
# Create network
docker network create subgen-network

# Run worker
docker run -d \
  --name subgen-worker \
  --network subgen-network \
  ghcr.io/lenaxia/subgen-worker:latest

# Run orchestrator
docker run -d \
  --name subgen-orchestrator \
  --network subgen-network \
  -e WORKER_ADDRESS=subgen-worker:50051 \
  ghcr.io/lenaxia/subgen-orchestrator:latest
```

## Scaling Workers

One benefit of the new architecture is easy worker scaling:

```yaml
services:
  orchestrator:
    image: ghcr.io/lenaxia/subgen-orchestrator:latest
    environment:
      - WORKER_ADDRESSES=worker1:50051,worker2:50051,worker3:50051
    # ... rest of config

  worker1:
    image: ghcr.io/lenaxia/subgen-worker:latest
    # ... config

  worker2:
    image: ghcr.io/lenaxia/subgen-worker:latest
    # ... config

  worker3:
    image: ghcr.io/lenaxia/subgen-worker:latest
    # ... config
```

> Note: Multi-worker support requires load balancing configuration (coming in future release)

## Troubleshooting

### "Worker not available" Error

**Problem:** Orchestrator can't connect to worker.

**Solutions:**
1. Check worker is running: `docker ps | grep worker`
2. Verify worker address: `docker exec orchestrator env | grep WORKER_ADDRESS`
3. Check network connectivity: `docker exec orchestrator ping worker`
4. Check worker logs: `docker logs subgen-worker`

### "Path not found" Error

**Problem:** Orchestrator can't find media files.

**Solution:** Ensure orchestrator has same volume mounts as your media server:
```yaml
orchestrator:
  volumes:
    - /same/path/as/plex:/media
```

### Models Re-downloading

**Problem:** Worker downloads models every restart.

**Solution:** Ensure models volume is mounted:
```yaml
worker:
  volumes:
    - ./models:/models  # Persistent storage
```

### GPU Not Detected

**Problem:** Worker not using GPU.

**Solutions:**
1. Ensure nvidia-docker2 is installed: `docker run --rm --gpus all nvidia/cuda:12.3.2-base-ubuntu22.04 nvidia-smi`
2. Check GPU resources in compose file:
```yaml
worker:
  deploy:
    resources:
      reservations:
        devices:
          - driver: nvidia
            count: all
            capabilities: [gpu]
```
3. Use GPU image: `ghcr.io/lenaxia/subgen-worker:latest` (not `:cpu`)
4. Set device: `TRANSCRIBE_DEVICE=cuda`

## Rollback Procedure

If you need to rollback to the legacy version:

```bash
# Stop new services
docker-compose down

# Restore backup
cp docker-compose.yml.backup docker-compose.yml

# Start legacy container
docker-compose up -d
```

Legacy images will continue to receive security updates but no new features.

## FAQ

### Q: Can I still use standalone Python without Docker?

**A:** Yes, but you need to run both components:
1. Build orchestrator: `cd orchestrator && go build -o bin/orchestrator ./cmd/orchestrator`
2. Run worker: `cd worker && python -m worker.server`
3. Run orchestrator: `./orchestrator/bin/orchestrator`

### Q: Do I need to reconfigure my media server webhooks?

**A:** No, webhook endpoints remain the same (`http://orchestrator:9000/plex`, etc.)

### Q: Will this work with my existing Bazarr setup?

**A:** Yes, point Bazarr to the orchestrator address: `http://orchestrator:9000`

### Q: Can I mix old and new images?

**A:** No, you must use either the legacy monolithic image OR the new split architecture. Mixing won't work.

### Q: What happens to my existing subtitle files?

**A:** Nothing - they remain unchanged. New subtitles will be generated with the same format and naming.

### Q: Do I need to re-download Whisper models?

**A:** No, if you mount the models directory from your old setup:
```yaml
worker:
  volumes:
    - ./old-models:/models  # Reuse existing models
```

## Getting Help

- **Issues:** https://github.com/lenaxia/subgen/issues
- **Discussions:** https://github.com/lenaxia/subgen/discussions
- **Legacy Code:** See `legacy/` directory for old implementation

## Summary

| Aspect | Legacy | New Architecture |
|--------|--------|------------------|
| Images | 1 monolithic | 2 separate (orchestrator + worker) |
| Languages | Python only | Go + Python |
| Scaling | Single container | Multiple workers possible |
| Resource Isolation | Combined | Separate (CPU orchestrator, GPU worker) |
| Restart Impact | Full service down | Component-level restarts |
| Configuration | Same container | Split across services |
| Webhooks | Same endpoints | Same endpoints |
| Env Variables | Mostly same | Added WORKER_ADDRESS |
| Volume Paths | `/subgen/models` | `/models` |

**Bottom Line:** More complex setup, but much better performance and scalability.
