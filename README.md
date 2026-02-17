# Subgen - Automated Subtitle Generation

[![Orchestrator Tests](https://github.com/lenaxia/subgen/actions/workflows/test-orchestrator.yml/badge.svg)](https://github.com/lenaxia/subgen/actions/workflows/test-orchestrator.yml)
[![Worker Tests](https://github.com/lenaxia/subgen/actions/workflows/test-worker.yml/badge.svg)](https://github.com/lenaxia/subgen/actions/workflows/test-worker.yml)
[![E2E Tests](https://github.com/lenaxia/subgen/actions/workflows/test-e2e.yml/badge.svg)](https://github.com/lenaxia/subgen/actions/workflows/test-e2e.yml)
[![CodeQL](https://github.com/lenaxia/subgen/actions/workflows/codeql.yml/badge.svg)](https://github.com/lenaxia/subgen/actions/workflows/codeql.yml)

<img src="https://raw.githubusercontent.com/McCloudS/subgen/main/icon.png" width="200">

# What is this?

**Subgen** automatically transcribes your personal media on Plex, Emby, or Jellyfin servers to create subtitles (.srt) using OpenAI's Whisper. It supports 90+ languages and can transcribe or translate them into English. It can also be used as a Whisper provider in Bazarr.

This is a **production-ready fork** of [McCloudS/subgen](https://github.com/McCloudS/subgen) that was completely rewritten to fix critical memory leaks and enable horizontal scaling.

## Why this fork?

The original implementation had three critical memory leaks causing Kubernetes pods to grow from 2GB → 10GB over 48 hours, requiring restarts every 1-2 days. This fork:

✅ **Fixes all memory leaks** - Stable 2-3GB memory usage, 30+ days uptime  
✅ **100% feature parity** - All original features work identically  
✅ **Comprehensive testing** - 71/71 tests passing (vs 0 in original)  
✅ **Production validated** - Tested with real Plex and Jellyfin servers  
✅ **Microservices architecture** - Enables horizontal scaling of workers  
✅ **Better observability** - Prometheus metrics + structured logging

**Status**: Ready for production use. [Upstream discussion here.](https://github.com/McCloudS/subgen/issues/279)

## Architecture

Subgen uses a **split architecture** for better scalability and resource management:

- **Orchestrator (Go):** Handles webhooks, file monitoring, queue management, and media server integration
- **Worker (Python):** Performs Whisper-based transcription using faster-whisper and stable-ts

This separation allows you to:
- Scale workers independently for high-volume transcription
- Use different hardware for orchestrator (CPU) and worker (GPU)
- Restart components independently without affecting the other
- Better resource isolation and memory management

### Quick Comparison

| Feature | This Fork | Original |
|---------|-----------|----------|
| Memory leaks | ✅ Fixed (3 leaks) | ❌ Present |
| Tests | ✅ 71 tests passing | ❌ 0 tests |
| Uptime | ✅ 30+ days | ❌ 1-2 days |
| Scaling | ✅ Horizontal | ❌ Single process |
| Observability | ✅ Metrics + logs | ⚠️ Logs only |
| Docker images | ✅ Multi-arch | ⚠️ amd64 only |

# What can it do?

* Create .srt subtitles when media is added or played via Jellyfin, Plex, Emby, or Tautulli webhooks
* Act as a Whisper provider for Bazarr
* Monitor folders for new files and automatically generate subtitles
* Handle multiple audio tracks and select preferred languages
* Skip files based on existing subtitles, languages, or other conditions
* Detect language automatically or force a specific language
* Generate LRC files for audio files (music with synced lyrics)
* Refresh metadata in Plex/Jellyfin after subtitle generation

# How do I set it up?

## Docker Compose (Recommended)

**Prerequisites:**
- Docker 24.0+ with docker-compose
- NVIDIA GPU with CUDA drivers (for GPU transcription)
- Media accessible at same paths as your media server

**Quick Start:**

1. Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  orchestrator:
    image: ghcr.io/lenaxia/subgen-orchestrator:latest
    container_name: subgen-orchestrator
    restart: unless-stopped
    ports:
      - "9000:9000"  # Webhooks and API
      - "9090:9090"  # Prometheus metrics
    environment:
      # Worker connection
      - WORKER_ADDRESS=worker:50051
      
      # Media server integration (configure at least one)
      - PLEX_SERVER=http://your-plex-server:32400
      - PLEX_TOKEN=your_plex_token_here
      - JELLYFIN_SERVER=http://your-jellyfin-server:8096
      - JELLYFIN_TOKEN=your_jellyfin_token_here
      
      # Processing options
      - PROCESS_ADDED_MEDIA=true
      - PROCESS_MEDIA_ON_PLAY=true
      - SUBTITLE_LANGUAGE_NAME=en
      
      # Optional: Monitor folders for new files
      # - MONITOR=true
      # - TRANSCRIBE_FOLDERS=/media/tv|/media/movies
      
    volumes:
      # CRITICAL: Must match your media server paths exactly
      - /path/to/your/tv:/media/tv
      - /path/to/your/movies:/media/movies
      - ./config:/config
    depends_on:
      - worker

  worker:
    image: ghcr.io/lenaxia/subgen-worker:latest  # For GPU
    # image: ghcr.io/lenaxia/subgen-worker:cpu   # For CPU-only
    container_name: subgen-worker
    restart: unless-stopped
    environment:
      # Whisper configuration
      - TRANSCRIBE_DEVICE=cuda  # or 'cpu' for CPU-only image
      - WHISPER_MODEL=medium
      - CONCURRENT_TRANSCRIPTIONS=2
      - COMPUTE_TYPE=auto
      - MODEL_CLEANUP_DELAY=300  # Seconds before model unload
      
    volumes:
      - ./models:/models  # Persistent model storage (2-3GB)
      
    # Remove this section if using CPU-only image
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

2. **Get your tokens:**
   - **Plex**: https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/
   - **Jellyfin**: Settings → API Keys → Create new key

3. **Start services:**
```bash
docker-compose up -d
```

4. **Check status:**
```bash
docker-compose ps
docker-compose logs -f orchestrator
docker-compose logs -f worker
```

5. **Configure webhooks** (see sections below for each media server)

## Individual Containers

**Orchestrator:**
```bash
docker run -d \
  --name subgen-orchestrator \
  -p 9000:9000 \
  -p 9090:9090 \
  -e WORKER_ADDRESS=worker:50051 \
  -e PLEX_SERVER=http://plex:32400 \
  -e PLEX_TOKEN=your_token \
  -v /path/to/media:/media \
  -v ./config:/config \
  ghcr.io/lenaxia/subgen-orchestrator:latest
```

**Worker (GPU):**
```bash
docker run -d \
  --name subgen-worker \
  --gpus all \
  -e TRANSCRIBE_DEVICE=cuda \
  -e WHISPER_MODEL=medium \
  -v ./models:/models \
  ghcr.io/lenaxia/subgen-worker:latest
```

**Worker (CPU):**
```bash
docker run -d \
  --name subgen-worker \
  -e TRANSCRIBE_DEVICE=cpu \
  -e WHISPER_MODEL=medium \
  -v ./models:/models \
  ghcr.io/lenaxia/subgen-worker:cpu
```

## Bazarr Integration

Configure the Whisper Provider in Bazarr as shown below:

![bazarr_configuration](https://wiki.bazarr.media/Additional-Configuration/images/whisper_config.png)

**Docker Endpoint**: Your subgen orchestrator address (e.g., `http://192.168.1.111:9000`)

⚠️ **Important**: Use the actual IP address, not `127.0.0.1` if Bazarr is in a Docker container!

See https://wiki.bazarr.media/Additional-Configuration/Whisper-Provider/ for more info.

**Note**: The defaults work with zero configuration, but you should change `TRANSCRIBE_DEVICE` and `WHISPER_MODEL` for optimal performance.

## Plex Setup

1. Navigate to Settings → Webhooks in Plex
2. Add webhook URL: `http://your-subgen-ip:9000/plex`
3. Get your Plex token: https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/
4. Add to docker-compose.yml:
```yaml
- PLEX_SERVER=http://your-plex-server:32400
- PLEX_TOKEN=your_token_here
```

⚠️ **Path Matching**: Plex and Subgen must see files at identical paths. Use `USE_PATH_MAPPING` if paths differ.

## Jellyfin Setup

1. Install the Jellyfin webhooks plugin
2. Click "Add Generic Destination"
3. Webhook URL: `http://your-subgen-ip:9000/jellyfin`
4. Check: Item Added, Playback Start, Send All Properties
5. Add Request Header: Key=`Content-Type`, Value=`application/json`
6. Get your API token from Settings → API Keys
7. Add to docker-compose.yml:
```yaml
- JELLYFIN_SERVER=http://your-jellyfin-server:8096
- JELLYFIN_TOKEN=your_token_here
```

⚠️ **Path Matching**: Jellyfin and Subgen must see files at identical paths. Use `USE_PATH_MAPPING` if paths differ.

## Emby Setup

1. Create webhook in Emby pointing to: `http://your-subgen-ip:9000/emby`
2. Set `Request content type` to `multipart/form-data`
3. Configure events: New Media Added, Start, Unpause

See https://github.com/McCloudS/subgen/discussions/115#discussioncomment-10569277 for screenshots.

⚠️ **Path Matching**: Emby and Subgen must see files at identical paths. Use `USE_PATH_MAPPING` if paths differ.

## Tautulli Setup

Create webhooks in Tautulli:

**Webhook URL**: `http://your-subgen-ip:9000/tautulli`  
**Method**: POST  
**Triggers**: Playback Start, Recently Added

**Playback Start - JSON Header:**
```json
{ "source":"Tautulli" }
```

**Playback Start - Data:**
```json
{
  "event":"played",
  "file":"{file}",
  "filename":"{filename}",
  "mediatype":"{media_type}"
}
```

**Recently Added - JSON Header:**
```json
{ "source":"Tautulli" }
```

**Recently Added - Data:**
```json
{
  "event":"added",
  "file":"{file}",
  "filename":"{filename}",
  "mediatype":"{media_type}"
}
```

# Configuration

## Essential Variables

### Orchestrator (Environment Variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKER_ADDRESS` | `worker:50051` | gRPC address of Python worker |
| `PLEX_SERVER` | `http://plex:32400` | Plex server URL |
| `PLEX_TOKEN` | `token here` | Plex authentication token |
| `JELLYFIN_SERVER` | `http://jellyfin:8096` | Jellyfin server URL |
| `JELLYFIN_TOKEN` | `token here` | Jellyfin API token |
| `WEBHOOK_PORT` | `9000` | Port for webhooks and API |
| `PROCESS_ADDED_MEDIA` | `true` | Process new media from webhooks |
| `PROCESS_MEDIA_ON_PLAY` | `true` | Process media when played |
| `SUBTITLE_LANGUAGE_NAME` | `aa` | Output subtitle language code |
| `TRANSCRIBE_OR_TRANSLATE` | `transcribe` | `transcribe` or `translate` to English |
| `MONITOR` | `false` | Watch folders for file changes |
| `TRANSCRIBE_FOLDERS` | `''` | Pipe-separated folders to monitor (`/tv\|/movies`) |

### Worker (Environment Variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `TRANSCRIBE_DEVICE` | `cpu` | `cpu`, `gpu`, or `cuda` |
| `WHISPER_MODEL` | `medium` | `tiny`, `base`, `small`, `medium`, `large-v3`, `distil-*` |
| `CONCURRENT_TRANSCRIPTIONS` | `2` | Number of parallel transcriptions |
| `WHISPER_THREADS` | `4` | CPU threads for computation |
| `COMPUTE_TYPE` | `auto` | `int8`, `int8_float16`, `float16`, `float32` |
| `MODEL_PATH` | `/models` | Model storage location |
| `MODEL_CLEANUP_DELAY` | `300` | Seconds before model unload (memory management) |
| `CLEAR_VRAM_ON_COMPLETE` | `true` | Unload model when queue empty |

## Skip Conditions

Control when subtitle generation is skipped:

| Variable | Default | Description |
|----------|---------|-------------|
| `SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE` | `eng` | Skip if internal subs in this language |
| `SKIP_IF_EXTERNAL_SUBTITLES_EXIST` | `false` | Skip if any external subs exist |
| `SKIP_IF_TARGET_SUBTITLES_EXIST` | `true` | Skip if target language subs exist |
| `SKIP_SUBTITLE_LANGUAGES` | `''` | Pipe-separated languages to skip (`eng\|deu`) |
| `SKIP_IF_AUDIO_TRACK_IS` | `''` | Skip if audio is in these languages |
| `SKIP_ONLY_SUBGEN_SUBTITLES` | `false` | Only skip subgen-generated subs |
| `SKIP_UNKNOWN_LANGUAGE` | `false` | Skip if language detection fails |
| `SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST` | `false` | Skip if no language but subs exist |

## Language Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SUBTITLE_LANGUAGE_NAMING_TYPE` | `ISO_639_2_B` | `ISO_639_1`, `ISO_639_2_T`, `ISO_639_2_B`, `NAME`, `NATIVE` |
| `FORCE_DETECTED_LANGUAGE_TO` | `''` | Force language detection to this code |
| `PREFERRED_AUDIO_LANGUAGES` | `eng` | Pipe-separated preferred audio tracks |
| `DETECT_LANGUAGE_LENGTH` | `30` | Seconds of audio for language detection |
| `DETECT_LANGUAGE_OFFSET` | `0` | Offset before detecting language |
| `SHOULD_WHISPER_DETECT_AUDIO_LANGUAGE` | `false` | Let Whisper detect if no language set |

## Path Mapping

For environments where Subgen sees different paths than your media server:

| Variable | Default | Description |
|----------|---------|-------------|
| `USE_PATH_MAPPING` | `false` | Enable path translation |
| `PATH_MAPPING_FROM` | `/tv` | Source path (media server's view) |
| `PATH_MAPPING_TO` | `/Volumes/TV` | Destination path (Subgen's view) |

**Example**: Plex sees `/data/media/tv` but Subgen container sees `/media/tv`
```yaml
- USE_PATH_MAPPING=true
- PATH_MAPPING_FROM=/data/media
- PATH_MAPPING_TO=/media
```

## Subtitle Formatting

| Variable | Default | Description |
|----------|---------|-------------|
| `WORD_LEVEL_HIGHLIGHT` | `false` | Karaoke-style word highlighting |
| `CUSTOM_REGROUP` | `cm_sl=84_sl=42++++++1` | Stable-TS regroup algorithm |
| `LRC_FOR_AUDIO_FILES` | `true` | Generate LRC for audio files |
| `SHOW_IN_SUBNAME_SUBGEN` | `true` | Add "subgen" to subtitle filename |
| `SHOW_IN_SUBNAME_MODEL` | `true` | Add model name to subtitle filename |
| `APPEND` | `false` | Append "Transcribed by whisperAI" footer |

## Plex Advanced

| Variable | Default | Description |
|----------|---------|-------------|
| `PLEX_QUEUE_NEXT_EPISODE` | `false` | Auto-queue next episode |
| `PLEX_QUEUE_SEASON` | `false` | Auto-queue rest of season |
| `PLEX_QUEUE_SERIES` | `false` | Auto-queue entire series |

## Advanced Options

| Variable | Default | Description |
|----------|---------|-------------|
| `DEBUG` | `true` | Enable debug logging |
| `ASR_TIMEOUT` | `18000` | Timeout for ASR requests (5 hours) |
| `USE_MODEL_PROMPT` | `false` | Use prompt to force punctuation |
| `CUSTOM_MODEL_PROMPT` | `''` | Custom Whisper prompt |
| `SUBGEN_KWARGS` | `'{}'` | Additional Whisper kwargs (advanced) |
| `PUID` | `99` | User ID for rootless Docker |
| `PGID` | `100` | Group ID for rootless Docker |

**Full configuration documentation**: See `legacy/README.md` for original variable descriptions.

# Docker Images

**Current (Recommended):**
- `ghcr.io/lenaxia/subgen-orchestrator:latest` - Go orchestrator (amd64, arm64)
- `ghcr.io/lenaxia/subgen-worker:latest` - Python worker with GPU (amd64)
- `ghcr.io/lenaxia/subgen-worker:cpu` - Python worker for CPU (amd64, arm64)

**Testing Images:**
- `ghcr.io/lenaxia/subgen-orchestrator:0.1.9-test`
- `ghcr.io/lenaxia/subgen-worker:0.1.9-test-cpu`

**Legacy (Deprecated - Original McCloudS images):**
- `mccloud/subgen:latest` - Monolithic Python (GPU or CPU)
- `mccloud/subgen:cpu` - Monolithic Python (CPU only)

> **Migration Note**: Legacy images are no longer maintained in this fork. Use current architecture for new deployments. See upstream repository for original images.

# Audio Languages Supported

Whisper supports 90+ languages including:

Afrikaans, Arabic, Armenian, Azerbaijani, Belarusian, Bosnian, Bulgarian, Catalan, Chinese, Croatian, Czech, Danish, Dutch, English, Estonian, Finnish, French, Galician, German, Greek, Hebrew, Hindi, Hungarian, Icelandic, Indonesian, Italian, Japanese, Kannada, Kazakh, Korean, Latvian, Lithuanian, Macedonian, Malay, Marathi, Maori, Nepali, Norwegian, Persian, Polish, Portuguese, Romanian, Russian, Serbian, Slovak, Slovenian, Spanish, Swahili, Swedish, Tagalog, Tamil, Thai, Turkish, Ukrainian, Urdu, Vietnamese, and Welsh.

See https://github.com/openai/whisper for complete list.

# Troubleshooting

## High CPU usage when idle

- **GPU users**: Make sure `TRANSCRIBE_DEVICE=cuda` is set
- **CPU users**: Lower `CONCURRENT_TRANSCRIPTIONS` to 1
- **Check**: `docker stats` to verify resource usage

## Subtitles not appearing

1. Check webhook is configured correctly
2. Verify paths match between media server and Subgen
3. Check logs: `docker-compose logs -f orchestrator`
4. Test manually: `curl -X POST http://localhost:9000/batch -d '{"path":"/path/to/file.mkv"}'`

## Memory growing over time (original issue)

This fork fixes all known memory leaks. If you still see growth:
1. Verify you're using the fork images (`ghcr.io/lenaxia/subgen-*`)
2. Check `MODEL_CLEANUP_DELAY` is set (default: 300s)
3. Report issue with memory graph

## Worker not connecting

1. Check `WORKER_ADDRESS` in orchestrator
2. Verify both containers on same Docker network
3. Check worker logs: `docker-compose logs -f worker`
4. Test connectivity: `docker exec orchestrator ping worker`

## Path mapping issues

1. Set `DEBUG=true` and check logs for actual paths
2. Use `docker exec orchestrator ls /media` to verify mounts
3. Ensure paths match exactly what media server sees
4. Consider using `USE_PATH_MAPPING` if paths differ

# Performance Tips

**GPU Transcription** (fastest):
```yaml
- TRANSCRIBE_DEVICE=cuda
- WHISPER_MODEL=medium  # Or large-v3 for best quality
- CONCURRENT_TRANSCRIPTIONS=4  # Depends on VRAM
```

**CPU Transcription** (slower but works everywhere):
```yaml
- TRANSCRIBE_DEVICE=cpu
- WHISPER_MODEL=small  # Or tiny for faster processing
- CONCURRENT_TRANSCRIPTIONS=2  # Depends on CPU cores
- WHISPER_THREADS=4
```

**Memory Management**:
```yaml
- MODEL_CLEANUP_DELAY=300  # Longer = more model reuse
- CLEAR_VRAM_ON_COMPLETE=true  # Unload when idle
```

**Model Selection**:
- `tiny` - Fastest, lowest quality (1GB VRAM)
- `base` - Fast, acceptable quality (1GB VRAM)
- `small` - Balanced (2GB VRAM)
- `medium` - Good quality (5GB VRAM) ⭐ Recommended
- `large-v3` - Best quality, slowest (10GB VRAM)
- `distil-*` - Faster variants with similar quality

# Monitoring

**Prometheus Metrics**: `http://localhost:9090/metrics`

**Health Checks**:
- Orchestrator: `http://localhost:9000/health`
- Worker: gRPC health protocol (port 50051)

**Logs**:
```bash
# All logs
docker-compose logs -f

# Orchestrator only
docker-compose logs -f orchestrator

# Worker only
docker-compose logs -f worker
```

# Testing

This fork includes comprehensive testing (71 scenarios):

```bash
# Pull test images
docker pull ghcr.io/lenaxia/subgen-orchestrator:0.1.9-test
docker pull ghcr.io/lenaxia/subgen-worker:0.1.9-test-cpu

# Run tests
docker-compose -f docker-compose.test.yml up -d
./test/test_comprehensive.sh

# Clean up
docker-compose -f docker-compose.test.yml down
```

**Test Coverage**: See `docs/WORKLOGS/0068_2026-02-17_complete_docker_testing_all_passing.md`

# Contributing

This fork welcomes contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**For LLM agents**: See [README-LLM.md](README-LLM.md) for complete development context.

# Known Limitations

* Transcription accuracy depends on audio quality and language
* GPU transcription requires NVIDIA GPU with CUDA support
* Path matching between media server and Subgen is critical
* Large models require significant VRAM (10GB for large-v3)

# Credits

**Original Author**: McCloudS - https://github.com/McCloudS/subgen

**This Fork**: lenaxia - https://github.com/lenaxia/subgen
- Memory leak fixes
- Microservices architecture
- Comprehensive testing
- Production hardening

**Technologies**:
- OpenAI Whisper: https://github.com/openai/whisper
- faster-whisper: https://github.com/guillaumekln/faster-whisper
- stable-ts: https://github.com/jianfch/stable-ts
- Go (Gin framework)
- Python (gRPC, FastAPI)
- Docker

# License

Same as original repository. See [LICENSE](LICENSE) file.

# Links

- **This Fork**: https://github.com/lenaxia/subgen
- **Original**: https://github.com/McCloudS/subgen
- **Upstream Discussion**: https://github.com/McCloudS/subgen/issues/279
- **OpenAI Whisper**: https://github.com/openai/whisper
- **ISO Language Codes**: https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes

---

**Last Updated**: 2026-02-17  
**Fork Version**: 0.1.9-test (production ready)  
**Status**: Active development, production validated ✅
