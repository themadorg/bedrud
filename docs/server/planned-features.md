# Planned Features (In Code, Not Fully Wired)

Several subsystems have model, handler, and queue code written but are marked `TODO oncoming feature` in `internal/server/server.go` — they are not active in the default production bootstrap.

---

## Recordings

### What exists

| Component | Status |
|-----------|--------|
| `models/recording.go` | Model defined |
| `repository/recording_repository.go` | Full CRUD |
| `storage/recording_store.go` | Disk storage |
| `services/recording_service.go` | Egress start/stop/process |
| `handlers/recording_handler.go` | HTTP handlers written |
| `queue/handler_process_recording.go` | Implemented |
| `queue/handler_recording_delete.go` | Implemented |
| `middleware/recordings_enabled.go` | Gate middleware |
| `SystemSettings.RecordingsEnabled` | Admin toggle in DB |

### What is commented out in `server.go`

```go
// recordingStore = storage.NewRecordingStore(...)
// egressClient = lkutil.NewEgressClient(...)
// recordingService = services.NewRecordingService(...)
// recordingHandler = handlers.NewRecordingHandler(...)
// queue handlers: process_recording, recording_delete
// API routes: /api/room/:id/recording/*, /api/recordings/*
// admin recording routes
// Static serving of recording files
```

### Scheduler (partially active)

- Stale recording cleanup (failed/pending > 7 days) — **runs** via `recordingRepo.DeleteStaleRecordings`
- Retention cleanup on archived rooms — **disabled** until `recStore` is passed to scheduler

---

## How to tell if a feature is live

Check `internal/server/server.go` route registration block. If routes are commented with `TODO oncoming feature`, the feature is not exposed over HTTP even if handler files exist.

---

## Other TODOs in codebase

| Location | Note |
|----------|------|
| `handlers/cooldown.go` | Redis-backed cooldown for multi-instance |
| `queue/handler_stubs.go` | Legacy stub file; real handlers exist in dedicated files |
| `config/config.go` | `Recording` section marked TODO |
| `models/settings.go` | Recording fields exist but feature gated at bootstrap |