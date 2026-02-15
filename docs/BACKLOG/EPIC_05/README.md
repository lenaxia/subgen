# EPIC_05: Migration & Cutover

**Status:** Not Started  
**Estimated Effort:** 18-26 hours  
**Duration:** 3-4 days  
**Can Parallelize:** ❌ No (depends on EPIC_04)

---

## Overview

Migrate from the current Docker-based `subgen.py` deployment to the new Kubernetes-based hybrid Go/Python architecture. This epic ensures **zero data loss**, **feature parity**, and a **safe rollback plan**.

---

## Goals

1. Validate feature parity (new system = old system capabilities)
2. Create migration guide (Docker → Kubernetes)
3. Deploy to production cluster
4. Validate 24-hour production operation
5. Archive legacy code
6. Document rollback procedure

---

## Design References

- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md) - System architecture
- [04_K8S_DEPLOYMENT.md](../../DESIGN/04_K8S_DEPLOYMENT.md) - K8s deployment
- README-LLM.md - Legacy subgen.py architecture

---

## User Stories

### [STORY_01: Feature Parity Validation](./stories/STORY_01_feature_parity.md)
**Status:** Not Started  
**Effort:** 6-8 hours  
**Summary:** Checklist of all current features, test each in new system

### [STORY_02: Migration Guide](./stories/STORY_02_migration_guide.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Summary:** Docker → K8s instructions, environment variable mapping, webhook reconfiguration

### [STORY_03: Production Deployment](./stories/STORY_03_production_deployment.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Summary:** Deploy to production cluster, configure webhooks, 24-hour validation

### [STORY_04: Legacy Archival](./stories/STORY_04_legacy_archival.md)
**Status:** Not Started  
**Effort:** 4-6 hours  
**Summary:** Archive `subgen.py`, update README, document rollback procedure

---

## Acceptance Criteria

- [ ] All 4 stories completed
- [ ] Feature parity validated (100% of old features work)
- [ ] Migration guide complete and tested
- [ ] Production deployment successful
- [ ] 24-hour validation passed (zero crashes)
- [ ] Legacy code archived in `legacy/` directory
- [ ] Rollback procedure documented
- [ ] README.md updated for new architecture
- [ ] Work logs created for all stories

---

## Dependencies

**Requires:**
- EPIC_04 (K8s Deployment) - **MUST be complete**
- Docker images built and pushed to ghcr.io
- Production Kubernetes cluster available

**Blocks:**
- None (final epic)

**Parallelizable With:**
- None (sequential epic)

---

## Feature Parity Checklist

### Webhook Support

- [ ] **Plex webhooks**
  - [ ] `library.new` event (new media added)
  - [ ] `media.play` event (media played)
  - [ ] Queue next episode (`PLEX_QUEUE_NEXT_EPISODE`)
  - [ ] Queue season (`PLEX_QUEUE_SEASON`)
  - [ ] Queue series (`PLEX_QUEUE_SERIES`)
  - [ ] Metadata refresh after transcription

- [ ] **Jellyfin webhooks**
  - [ ] `ItemAdded` event
  - [ ] `PlaybackStart` event
  - [ ] Metadata refresh after transcription

- [ ] **Emby webhooks**
  - [ ] `library.new` event
  - [ ] `playback.start` event

- [ ] **Tautulli webhooks**
  - [ ] `added` event
  - [ ] `played` event

---

### Transcription Features

- [ ] **Whisper models**
  - [ ] tiny, base, small, medium, large, large-v3
  - [ ] Distil variants (distil-whisper)
  - [ ] CPU transcription
  - [ ] GPU transcription (CUDA)

- [ ] **Languages**
  - [ ] Auto-detection (104 languages supported)
  - [ ] Force language
  - [ ] Translate to English mode
  - [ ] Preferred audio track languages

- [ ] **Subtitle formats**
  - [ ] SRT for video files
  - [ ] LRC for audio files
  - [ ] Word-level highlighting (karaoke mode)
  - [ ] Custom regroup algorithm (stable-ts)

- [ ] **Skip conditions**
  - [ ] Skip if internal subtitles exist
  - [ ] Skip if external subtitles exist
  - [ ] Skip if target subtitles exist
  - [ ] Skip specific languages
  - [ ] Skip if audio track in list
  - [ ] Skip unknown language

---

### Queue & Processing

- [ ] **Priority queue**
  - [ ] Priority 0: Language detection (highest)
  - [ ] Priority 1: ASR requests (time-sensitive)
  - [ ] Priority 2: Standard transcription
  - [ ] Deduplication (by file path)

- [ ] **Processing modes**
  - [ ] Process added media (`PROCESS_ADDED_MEDIA`)
  - [ ] Process media on play (`PROCESS_MEDIA_ON_PLAY`)
  - [ ] Batch processing (`/batch` endpoint)
  - [ ] Monitor folders (`MONITOR` + `TRANSCRIBE_FOLDERS`)

- [ ] **Concurrent workers**
  - [ ] Configurable worker count (`CONCURRENT_TRANSCRIPTIONS`)

---

### Media Server Integration

- [ ] **Plex**
  - [ ] Fetch file path via API (`get_plex_file_name`)
  - [ ] Refresh metadata after transcription

- [ ] **Jellyfin**
  - [ ] Fetch file path via API (`get_jellyfin_file_name`)
  - [ ] Refresh metadata after transcription

---

### Advanced Features

- [ ] **Path mapping**
  - [ ] `USE_PATH_MAPPING`
  - [ ] `PATH_MAPPING_FROM` / `PATH_MAPPING_TO`

- [ ] **Model management**
  - [ ] Lazy loading
  - [ ] Delayed cleanup (30s default)
  - [ ] VRAM clearing (`CLEAR_VRAM_ON_COMPLETE`)

- [ ] **ASR endpoint** (Bazarr integration)
  - [ ] `/asr` endpoint (blocking, returns SRT)
  - [ ] Audio hash-based deduplication
  - [ ] Configurable timeout (`ASR_TIMEOUT`)

- [ ] **Language detection endpoint**
  - [ ] `/detect-language` endpoint
  - [ ] Sample length/offset configuration

---

## Environment Variable Mapping

### Old (subgen.py) → New (Orchestrator + Worker)

| Old Variable | New Variable (Orchestrator) | New Variable (Worker) | Notes |
|--------------|-----------------------------|-----------------------|-------|
| `WEBHOOK_PORT` | `WEBHOOK_PORT` | - | Orchestrator only |
| `PLEX_SERVER` | `PLEX_SERVER` | - | Orchestrator only |
| `PLEX_TOKEN` | `PLEX_TOKEN` | - | Orchestrator only (secret) |
| `JELLYFIN_SERVER` | `JELLYFIN_SERVER` | - | Orchestrator only |
| `JELLYFIN_TOKEN` | `JELLYFIN_TOKEN` | - | Orchestrator only (secret) |
| `WHISPER_MODEL` | - | `WHISPER_MODEL` | Worker only |
| `WHISPER_THREADS` | - | `WHISPER_THREADS` | Worker only |
| `TRANSCRIBE_DEVICE` | - | `TRANSCRIBE_DEVICE` | Worker only |
| `COMPUTE_TYPE` | - | `COMPUTE_TYPE` | Worker only |
| `MODEL_PATH` | - | `MODEL_PATH` | Worker only |
| `CONCURRENT_TRANSCRIPTIONS` | `WORKER_COUNT` | - | Orchestrator (Phase 2 only) |
| `PROCESS_ADDED_MEDIA` | `PROCESS_ADDED_MEDIA` | - | Orchestrator only |
| `PROCESS_MEDIA_ON_PLAY` | `PROCESS_MEDIA_ON_PLAY` | - | Orchestrator only |
| `TRANSCRIBE_OR_TRANSLATE` | `TRANSCRIBE_OR_TRANSLATE` | - | Orchestrator only |
| All skip conditions | Orchestrator | - | Skip logic in orchestrator |
| Subtitle formatting | - | Worker | Formatting in worker |

**New Variables:**
- `WORKER_DISCOVERY` (orchestrator): `localhost` or `kubernetes`
- `PYTHON_WORKER_ADDRESS` (orchestrator): `localhost:50051` (Phase 1)
- `GRPC_PORT` (worker): `50051`

---

## Migration Procedure

### Phase 1: Preparation

1. **Backup current configuration**
   ```bash
   docker-compose config > docker-compose.backup.yml
   cp subgen.env subgen.env.backup
   ```

2. **Document current webhook URLs**
   - Plex: Settings → Webhooks → URL
   - Jellyfin: Dashboard → Plugins → Webhook → URL

3. **Test new system in parallel**
   - Deploy to K8s with different webhook URL
   - Test with non-critical media
   - Validate subtitle quality matches old system

---

### Phase 2: Cutover

1. **Update webhook URLs**
   - Plex: `http://<k8s-ip>:9000/plex`
   - Jellyfin: `http://<k8s-ip>:9000/jellyfin`
   - Emby: `http://<k8s-ip>:9000/emby`
   - Tautulli: `http://<k8s-ip>:9000/tautulli`

2. **Stop old Docker container**
   ```bash
   docker-compose down
   ```

3. **Monitor new system**
   ```bash
   kubectl logs -n media -l app.kubernetes.io/name=subgen -c orchestrator -f
   kubectl logs -n media -l app.kubernetes.io/name=subgen -c worker -f
   ```

4. **Validate first transcription**
   - Trigger webhook manually
   - Verify subtitle file created
   - Verify media server metadata refreshed

---

### Phase 3: Validation

1. **24-hour soak test**
   - Monitor for crashes
   - Monitor memory usage
   - Monitor queue size

2. **Performance validation**
   - Compare transcription times (old vs new)
   - Verify subtitle quality unchanged
   - Verify all features working

3. **Rollback readiness**
   - Keep old Docker setup available
   - Document rollback steps
   - Test rollback procedure

---

## Rollback Procedure

**If new system fails, rollback to Docker:**

```bash
# 1. Update webhooks back to old URLs
# Plex: http://<docker-host>:9000/plex

# 2. Start old Docker container
docker-compose up -d

# 3. Verify old system working
docker logs -f subgen

# 4. Scale down K8s deployment (optional)
kubectl scale deployment subgen --replicas=0 -n media
```

---

## Legacy Archival

### Files to Archive

Move to `legacy/` directory:
- `subgen.py` (2,144 lines) → `legacy/subgen.py`
- `launcher.py` → `legacy/launcher.py`
- `language_code.py` → `legacy/language_code.py` (keep copy in `worker/utils/`)
- `docker-compose.yml` → `legacy/docker-compose.yml`
- `Dockerfile` → `legacy/Dockerfile.gpu`
- `Dockerfile.cpu` → `legacy/Dockerfile.cpu`

### Create Legacy README

**File:** `legacy/README_LEGACY.md`

```markdown
# Subgen Legacy (Docker-based)

This directory contains the original Docker-based implementation of Subgen.

**Status:** Archived as of 2026-02-XX  
**Replacement:** Kubernetes-based hybrid Go/Python architecture

## Rollback Instructions

If you need to rollback to the legacy system:

1. Copy `docker-compose.yml` to repository root
2. Copy `subgen.py` to repository root
3. Run `docker-compose up -d`
4. Update webhooks to point to Docker host

## Why Archived?

The new architecture provides:
- No memory leaks (tested with 1000+ transcriptions)
- Horizontal scaling (Phase 1 → Phase 2)
- Better observability (Prometheus metrics)
- Modular, testable codebase
- Kubernetes-native deployment

## Legacy Features

All features from the legacy system are preserved in the new architecture.
See migration guide: `docs/BACKLOG/EPIC_05/stories/STORY_02_migration_guide.md`
```

---

## Timeline

**Day 1:** STORY_01 (Feature Parity Validation) - 6-8 hours  
**Day 2:** STORY_02 (Migration Guide) - 4-6 hours  
**Day 3:** STORY_03 (Production Deployment) - 4-6 hours  
**Day 4:** STORY_04 (Legacy Archival) + 24-hour validation

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Missing features | **CRITICAL** | Comprehensive checklist, test every feature |
| Production downtime | High | Test in parallel, rollback plan ready |
| Configuration errors | High | Document all env var mappings |
| Subtitle quality regression | High | Side-by-side comparison tests |
| User confusion | Medium | Update README with clear instructions |

---

## Definition of Done

- [ ] All 4 stories completed with ✅ status
- [ ] Feature parity validated (100% checklist complete)
- [ ] Migration guide complete and tested
- [ ] Production deployment successful
- [ ] 24-hour validation passed
- [ ] Legacy code archived
- [ ] README.md updated
- [ ] Rollback procedure documented and tested
- [ ] Work logs created for each story
- [ ] Users can successfully migrate with documentation alone

---

## Success Metrics

- **Downtime:** < 5 minutes (webhook URL update)
- **Rollbacks:** Zero (perfect migration)
- **Feature Parity:** 100% (all features work)
- **Performance:** Transcription time ± 10% of old system
- **Quality:** Subtitle quality unchanged (same Whisper models)
- **Stability:** Zero crashes in 24-hour validation

---

## Next Steps

After successful migration:
1. Monitor production for 1 week
2. Gather user feedback
3. Consider Phase 2 scaling (if load increases)
4. Continue improving based on production metrics

---

## References

- README-LLM.md - Development workflow
- [00_HYBRID_ARCHITECTURE.md](../../DESIGN/00_HYBRID_ARCHITECTURE.md)
- [04_K8S_DEPLOYMENT.md](../../DESIGN/04_K8S_DEPLOYMENT.md)
- Legacy `subgen.py` - Original implementation

---

**Epic Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
