# Legacy Subgen (Python-only)

This directory contains the original monolithic Python-based Subgen implementation.

## ⚠️ Deprecated

This code is **deprecated** and kept for reference only. Please use the new architecture:

- **Orchestrator:** `/orchestrator` - Go-based HTTP/webhook server, queue management, and orchestration
- **Worker:** `/worker` - Python-based Whisper transcription service (gRPC)

## What's in this folder

- `subgen.py` - Original monolithic Python implementation
- `launcher.py` - Launcher script for the legacy version
- `Dockerfile` - Legacy GPU Docker image
- `Dockerfile.cpu` - Legacy CPU Docker image
- `entrypoint.sh` - Docker entrypoint for legacy images
- `requirements.txt` - Legacy Python dependencies

## Migration

If you're using the legacy version, please migrate to the new architecture:

1. **Replace single container** with orchestrator + worker architecture
2. **Update environment variables** to new format (see main README)
3. **Use separate Docker images:**
   - `ghcr.io/lenaxia/subgen-orchestrator:latest` - Orchestrator
   - `ghcr.io/lenaxia/subgen-worker:latest` - Worker (GPU)
   - `ghcr.io/lenaxia/subgen-worker:cpu` - Worker (CPU-only)

## Legacy CalVer Workflow

The `calver.yml` workflow updates version in `subgen.py` - this is also legacy and will be removed in future releases.
