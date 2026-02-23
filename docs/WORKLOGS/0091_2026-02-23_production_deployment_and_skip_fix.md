# Work Log: Production Deployment & Subtitle Skip Fix
## Date: February 23, 2026
## Epic: Production Readiness & Validation

---

## Summary

End-to-end production deployment of the subgen orchestrator + worker pipeline on a Talos/Flux
k8s cluster. Resolved several integration issues encountered during live testing and implemented
the subtitle skip logic correctly in the worker.

---

## Issues Found & Fixed

### 1. Orchestrator image was stale (v0.2.21)

The orchestrator helm release had not been bumped to include the `/webhook/plex` route alias fix
from v0.2.22. Plex sends webhooks directly to `/webhook/plex` but the Fiber app only had `/plex`
registered in older builds.

**Fix:** Bumped `orchestrator-helm-release.yaml` tag to `v0.2.22`, committed and pushed to
`talos-ops-prod`. Force-reconciled Flux via:
```
flux reconcile kustomization cluster-media-subgen --namespace flux-system --with-source
```

---

### 2. Worker pods had no NFS mount

The worker pods could not access `/omoikane` (the NFS share where media files live), causing
every gRPC transcription request to fail with:
```
rpc error: code = NotFound desc = File not found: /omoikane/...
```

**Fix:** Added NFS persistence block to `worker-helm-release.yaml`:
```yaml
persistence:
  omoikane:
    enabled: true
    type: nfs
    server: 192.168.0.120
    path: /volume1/omoikane
    globalMounts:
      - path: /omoikane
```

---

### 3. First live test hit a stale worker pod (no NFS)

The first `media.play` webhook after deployment was routed to the old worker pod (`j9vjj`,
`Completed` state, no NFS mount) which had not yet been evicted from service endpoints. It
returned `File not found` despite the file existing on NFS.

**Resolution:** No code change needed. Once the old pod fully completed and new pods (`78zdg`,
`z2pmq`) were healthy, subsequent requests routed correctly.

---

### 4. End-to-end success confirmed

After the above fixes, a live `media.play` event for Gotham Girls S01E01 completed successfully:

```
file_path: /omoikane/[TV Shows]/Gotham Girls/Season 1/Gotham Girls - S01E01 - The Vault SDTV.mkv
detected_lang: en
duration_sec: 84.365620428
subtitle_path: ...Gotham Girls - S01E01 - The Vault SDTV.subgen.medium.eng.srt
```

Plex metadata refresh was triggered automatically after subtitle write.

---

### 5. Subtitle skip not working (`SKIP_IF_TARGET_SUBTITLES_EXIST=true`)

Playing the same episode again caused the worker to re-transcribe it from scratch, ignoring the
existing `.srt` file. Root cause analysis:

- `SKIP_IF_TARGET_SUBTITLES_EXIST` is read by both orchestrator and worker configs
- The orchestrator's skip checker (`orchestrator/internal/skip/`) is fully implemented but the
  Plex webhook handler has the skip check **commented out** (`server.go:330-354`) because the
  orchestrator has no filesystem access — it cannot `stat()` the `.srt` file
- The worker has `skip_if_target_subtitles_exist` in `WorkerSettings.skip` config but the gRPC
  `Transcribe` handler in `service.py` never consulted it

**Architecture decision:** Keep skip logic in the **worker** — it already owns NFS access and the
transcription concern. The orchestrator should not need filesystem access.

**Fix:** Added skip check in `worker/src/grpc_server/service.py` `Transcribe()` handler,
immediately after the file-exists check, before loading the Whisper model:

```python
if self.config.skip.skip_if_target_subtitles_exist:
    base = os.path.splitext(request.file_path)[0]
    existing = glob.glob(f"{base}.subgen.*.*.srt") + glob.glob(f"{base}.subgen.*.*.lrc")
    if existing:
        logger.info(f"Skipping transcription, subtitle already exists: {existing[0]}")
        return transcription_pb2.TranscribeResponse(
            success=True,
            subtitle_path=existing[0],
            detected_language="",
        )
```

The glob pattern `<base>.subgen.*.*.srt` matches any language/model combination produced by
subgen, making it robust to language or model changes between runs.

**Tagged and released as `v0.2.23`.**

---

## Versions

| Component | Before | After |
|---|---|---|
| subgen-orchestrator | v0.2.21 | v0.2.22 |
| subgen-worker | v0.2.21-cpu | v0.2.23-cpu |

---

## Files Changed

### `talos-ops-prod` repo
- `kubernetes/apps/media/subgen/app/orchestrator-helm-release.yaml` — bumped to v0.2.22, added
  `WORKER_ADDRESS`, `PLEX_SERVER`, `LOG_LEVEL=debug` env vars
- `kubernetes/apps/media/subgen/app/worker-helm-release.yaml` — added NFS omoikane mount,
  `SKIP_IF_TARGET_SUBTITLES_EXIST=true`, bumped to v0.2.23-cpu

### `subgen` repo
- `worker/src/grpc_server/service.py` — added subtitle skip check in `Transcribe()` handler

---

## Remaining / Follow-up

- Verify skip works end-to-end once cluster recovers (API server was unreachable at end of
  session due to network issue)
- Set `LOG_LEVEL` back to `info` on orchestrator once confirmed stable
- The orchestrator's commented-out Plex skip check (`server.go:330-354`) can be cleaned up or
  removed — it is now superseded by the worker-side skip check
- Consider whether `SKIP_IF_TARGET_SUBTITLES_EXIST` should be removed from the orchestrator
  config entirely to avoid confusion
