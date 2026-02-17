# Quick Reference: Webhook Curl Commands

## Plex Webhook

### library.new (New Content)
```bash
curl -X POST http://localhost:9000/plex \
  -H "User-Agent: PlexMediaServer/1.40.0" \
  -F 'payload={"event":"library.new","Metadata":{"ratingKey":"12345","type":"episode","title":"Test Episode"}}'
```

### media.play (Playback Started)
```bash
curl -X POST http://localhost:9000/plex \
  -H "User-Agent: PlexMediaServer/1.40.0" \
  -F 'payload={"event":"media.play","Metadata":{"ratingKey":"67890","type":"episode","title":"Played Episode"}}'
```

---

## Jellyfin Webhook

### ItemAdded (New Content)
```bash
curl -X POST http://localhost:9000/jellyfin \
  -H "User-Agent: Jellyfin-Server/10.8.13" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "NotificationType=ItemAdded&ItemId=abc123&ItemType=Episode&ItemName=Test%20Episode"
```

### PlaybackStart (Playback Started)
```bash
curl -X POST http://localhost:9000/jellyfin \
  -H "User-Agent: Jellyfin-Server/10.8.13" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "NotificationType=PlaybackStart&ItemId=xyz789&ItemType=Episode&ItemName=Played%20Episode"
```

---

## Emby Webhook

### library.new (New Content)
```bash
curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'data={"Event":"library.new","Item":{"Name":"Test Episode","Path":"/path/to/file.mkv","Type":"Episode"}}'
```

### playback.start (Playback Started)
```bash
curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'data={"Event":"playback.start","Item":{"Name":"Test Episode","Path":"/path/to/file.mkv","Type":"Episode"}}'
```

### Test Notification
```bash
curl -X POST http://localhost:9000/emby \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d 'data={"Event":"system.notificationtest","Server":{"Name":"Test Emby Server"}}'
```

---

## Tautulli Webhook

### added (New Content)
```bash
curl -X POST http://localhost:9000/tautulli \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "source: Tautulli" \
  -d "event=added&file=/path/to/file.mkv&title=Test%20Episode"
```

### played (Playback Started)
```bash
curl -X POST http://localhost:9000/tautulli \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "source: Tautulli" \
  -d "event=played&file=/path/to/file.mkv&title=Test%20Movie"
```

---

## Monitoring Endpoints

### Check Orchestrator Status
```bash
curl http://localhost:9000/status | jq .
```

### Check Queue Status
```bash
curl http://localhost:9000/queue/status | jq .
```

### Check Processing Tasks
```bash
curl http://localhost:9000/queue/processing | jq .
```

### Check Queue History
```bash
curl http://localhost:9000/queue/history | jq .
```
