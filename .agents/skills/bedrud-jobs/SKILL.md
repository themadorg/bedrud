---
name: bedrud-jobs
description: Async job queue, scheduler, room cleanup, recording service/store, chat upload storage.
license: Apache License
---

# Bedrud Job Queue & Background Tasks

Go module `bedrud`. Packages:

| Package | Path |
|---------|------|
| Queue | `server/internal/queue/` |
| Scheduler | `server/internal/scheduler/` |
| Services | `server/internal/services/` |
| Storage | `server/internal/storage/` |

Worker polls `jobs` table. Postgres: `FOR UPDATE SKIP LOCKED`. SQLite: two-step claim (safe with `SetMaxOpenConns(1)`).

---

## `internal/queue/` — Job Queue

### Files

| File | Purpose |
|------|---------|
| `job.go` | 8 payload structs |
| `queue.go` | `Enqueue`, `Handler`, `Worker`, opts, depth cap |
| `worker.go` | Claim, dispatch, retry, stale recovery, cleanup helpers |
| `handler_user_delete.go` | Hard-delete user + rooms |
| `handler_room_delete.go` | Archive (`Purge=false`) or cascade hard-delete (`Purge=true`) |
| `handler_room_suspend.go` | Suspend room |
| `handler_chat_upload.go` | Async S3 chat image upload |
| `handler_email.go` | SMTP transactional email + branding |
| `handler_dispatch_webhook.go` | HMAC-signed outbound webhooks |
| `handler_process_recording.go` | Download LK egress → store → complete + webhook |
| `handler_recording_delete.go` | Hard-delete recording DB row |
| `handler_stubs.go` | Index comment only — **no stub handlers remain** |
| `templates/*` | HTML + plaintext email templates |

### Enqueue API

```go
type Handler func(ctx context.Context, db *gorm.DB, job *models.Job) error

Enqueue(ctx, db, jobType, payload, opts...) error
// opts: WithPriority(p), WithMaxAttempts(n), WithRunAt(t)
// Default priority 0, maxAttempts 3, runAt=now
// Refuses enqueue when pending+active >= maxQueueDepth (10000)
GetMaxDepth() int64
```

### Worker

```go
WorkerOptions{Interval: 500ms, Concurrency: 1} // from QueueConfig
NewWorker(db, handlers map[string]Handler, opts) *Worker
Start(ctx)  // recoverStaleJobs then spawn Concurrency goroutines
Stop()      // close stopCh
```

**Claim loop:** each tick drains all available jobs (inner loop until claim returns nil).

| Behavior | Detail |
|----------|--------|
| Job timeout | 5 min per handler (`context.WithTimeout`) |
| Panic | recovered → `markError` |
| No handler | `markPermanentFail` (no retries) |
| Success | status `done` |
| Failure | exponential backoff or `failed` |
| Stale recovery | on `Start`: `active` + `updated_at` >10m → `pending` |

### Retry & Backoff

`markError`: if `attempts >= maxAttempts` → `failed`. Else:

```
run_at = now + min(2^attempts * 5s, 1h)
status = pending
```

Default maxAttempts=3 → backoffs after attempts 1,2: ~10s, ~20s (capped 1h).

### Cleanup helpers

| Fn | Purpose |
|----|---------|
| `CleanupJobs(db, cutoff)` | Delete `done` older than cutoff |
| `CleanupFailedJobs(db, cutoff)` | Delete `failed` older than cutoff |

Scheduler: daily 03:00 — done >7d, failed >30d.

### Job model (`models.Job`)

Statuses: `pending` → `active` → `done` | `failed`.

Fields: `ID`, `Type`, `Payload` (JSON text), `RunAt`, `Priority` (lower=higher), `Status`, `Attempts`, `MaxAttempts`, `LastError`, timestamps.

### Payloads & handlers

| Type | Struct | Typical priority | Handler | Behavior |
|------|--------|------------------|---------|----------|
| `user_delete` | `UserDeletePayload{UserID, Email, RoomIDs}` | 1 | `NewUserDeleteHandler` | Fetch rooms → `DeleteUserRooms` → passkeys → prefs → user |
| `room_delete` | `RoomDeletePayload{RoomID, SystemEvent, SystemMessage, DeletedIdentity, Purge}` | 1 | `NewRoomDeleteHandler` | `Purge=true` → `CascadeDeleteRoom`; `false` → `ArchiveRoom` |
| `room_suspend` | `RoomSuspendPayload{RoomID}` | 2 | `NewRoomSuspendHandler` | `SuspendRoom` |
| `chat_upload_s3` | `ChatUploadS3Payload{Data(base64), RoomID, MimeType, UserID}` | 0 | `NewChatUploadS3Handler` | Decode → `Store` → `tracker.Record(..., "s3")` |
| `send_email` | `SendEmailPayload{To, Subject, TemplateName, TemplateData}` | 0 | `NewSendEmailHandler` | Render template + branding → SMTP (or log-skip if no SMTP) |
| `dispatch_webhook` | `WebhookPayload{URL, Event, Body, Secret}` | 0 | `NewDispatchWebhookHandler` | HMAC POST, soft-fail (always nil) |
| `process_recording` | `ProcessRecordingPayload{RoomID, RoomName, EgressID, FileURL, FileSize, RecordingType, DurationMs, StartedAt}` | 0 | `NewProcessRecordingHandler` | Download → store → complete + enqueue webhooks |
| `recording_delete` | `RecordingDeletePayload{RecordingID, RoomID, RoomName}` | 1 | `NewRecordingDeleteHandler` | Hard-delete recording DB row |

**Enqueue sources:**

| Type | Who enqueues |
|------|----------------|
| `user_delete` | `handlers/users.go` (delete / bulk) |
| `room_delete` | `handlers/room.go` — user end meeting `Purge=false`; admin close / bulk `Purge=true` |
| `room_suspend` | admin suspend / bulk |
| `chat_upload_s3` | `UploadChatImage` when S3 backend + large file |
| `send_email` | auth (welcome, verify, reset, password_changed), admin |
| `dispatch_webhook` | room lifecycle + `process_recording` for `recording.completed` |
| `process_recording` | `LiveKitWebhookHandler.handleEgressEnded` |
| `recording_delete` | `RecordingHandler` bulk delete |

### Handler details

#### `process_recording` (SHIPPED)

`NewProcessRecordingHandler(recordingRepo, webhookRepo, lkHost, lkInternalHost, lkAPIKey, lkAPISecret, recStore)`

1. Idempotency: only if status is `processing` (else skip)
2. Temp file download from egress `FileURL` (5m timeout)
3. Embedded LK: `resolveDownloadURL` appends short-lived JWT (`RoomAdmin` + room, 5m TTL). Cloud/pre-signed URLs used as-is
4. Square backoff retries: 3 attempts (1s/4s/9s)
5. Storage key: `recordings/{createdBy}/{roomID}/{recordingID}-{started}-{completed}.mp4`
6. `recStore.Store` → `recordingRepo.UpdateCompleted`
7. Enqueue `dispatch_webhook` for active `recording.completed` subscribers

#### `recording_delete` (SHIPPED)

`NewRecordingDeleteHandler(recordingRepo)` — fetch by ID, `DeleteRecording`. Missing row is success. **Does not call `RecordingStore.Delete`** (file cleanup is retention/cascade paths).

#### `send_email` (SHIPPED)

Templates (HTML+txt): `welcome`, `room_invite`, `password_reset`, `password_changed`, `verify_email`, `generic`. Branding from `EmailConfig.Templates` overlaid by `SystemSettings`. SMTPS/STARTTLS via `utils.SendSMTP`. 30s send timeout. No SMTP → log body, return nil.

#### `dispatch_webhook` (SHIPPED)

Envelope: `{event, timestamp, data}`. Headers: `X-Bedrud-Signature` (`sha256=` HMAC-SHA256), `X-Bedrud-Event`, `X-Bedrud-Timestamp`. 10s HTTP timeout. Soft-fail all errors (no retry).

### Config

```go
type QueueConfig struct {
    PollInterval ConfigInt // ms, default 500. Env: QUEUE_POLL_INTERVAL
    MaxAttempts  int       // default 3. Env: QUEUE_MAX_ATTEMPTS
    Concurrency  int       // default 1. Env: QUEUE_CONCURRENCY
}
```

### Bootstrap wiring (`server/internal/server/server.go`)

Registered today:

```go
"user_delete", "room_delete", "room_suspend",
"chat_upload_s3", "send_email", "dispatch_webhook"
```

`process_recording` / `recording_delete` handlers and `NewRecordingStore` / egress / `RecordingService` are **implemented** but currently commented out in bootstrap with leftover TODOs — jobs can be enqueued by webhook/admin paths while worker map omits them until wiring is restored. Stale active jobs with no handler → permanent fail.

---

## `internal/scheduler/scheduler.go` — gocron Tasks

```go
Initialize(db, roomRepo, userRepo, recordingRepo, lkCfg, serverCfg, recStore, recCfg)
Stop()
```

| Interval | Task |
|----------|------|
| Every 1 min | `CleanupExpiredRooms` — bulk mark expired inactive (excludes persistent) |
| Every 1 min | `checkIdleRooms` — LK participant counts → idle if 0 + >5m old. Skip persistent. Reactivate if join during check. Deactivate participants |
| Weekly 03:00 | `DeleteGuestUsers` — guests older than 7d |
| Daily 03:30 | `DeleteUnverifiedAccounts` — unverified local/passkey (TTL default 48h, `auth.unverifiedAccountTTLHours`) |
| Hourly | `CleanupBlockedTokens` |
| Hourly | `auth.PruneRevokedTokens` |
| Daily 03:00 | Queue: done >7d / failed >30d |
| Daily 03:00 | **Stale recordings** — `recordingRepo.DeleteStaleRecordings(now-7d)` when `recordingRepo != nil` |
| Daily 09:00 | TLS cert expiry — renew self-signed if ≤`CertWarnDays` (30) or expired (only when non-ACME TLS) |
| Every `CleanupIntervalHours` (default 24) | **Recording retention** — if `recCfg.RetentionHours > 0` && `recStore != nil` |

### Recording retention job

1. `FindExpiredOnArchivedRooms(cutoff)` where cutoff = now − `RetentionHours`
2. Per recording: `ExtractStorageKey` → `recStore.Delete` → `DeleteRecording`
3. `FindArchivedRoomsNoRecordings` → `HardDeleteRoom` for empty archived rooms

If `retentionHours=0` or no store: retention disabled (logged).

**Note:** bootstrap currently passes `recStore=nil, recCfg=nil` so retention is off until store is wired.

---

## `internal/services/recording_service.go` — RecordingService (SHIPPED)

LiveKit Egress orchestration. Gates: system → room exists → room allow → max per room (non-persistent) → no active recording.

```go
NewRecordingService(settingsRepo, recordingRepo, roomRepo, egressClient, apiKey, apiSecret)
```

| Fn | Purpose |
|----|---------|
| `StartRecording(ctx, roomID, createdBy)` | Create `pending` row → `StartRoomCompositeEgress` (MP4, **AudioOnly=true**) → optimistic lock to `started` with egress ID. On egress fail: delete row. On DB update fail after start: best-effort `StopEgress` |
| `StopRecording(ctx, roomID)` | `StopEgress` only — **does not** set processing/completed. Webhook `egress_ended` owns lifecycle. On stop fail: mark failed |
| `ListRecordings(roomID, page, limit)` | Paginated room list |
| `ListAdminRecordings(...)` | Admin filters: roomID, status, date range |
| `GetRecording(id)` | Single row |

**Errors:** `ErrRecordingsDisabled`, `ErrRecordingsNotAllowed`, `ErrActiveRecordingExists`, `ErrRoomNotFound`, `ErrNoActiveRecording`, `ErrEgressClientNotReady`, `ErrMaxRecordingsPerRoom`.

**Lifecycle:**

```
pending → (egress start) → started
  → (egress_ended webhook) → processing → enqueue process_recording
  → (handler) → completed | failed
  → (bulk delete) → deleting → recording_delete job
```

Egress auth: `lkutil.AuthContext` with `RoomRecord` + `RoomJoin` + room-scoped grant.

---

## `internal/services/room_cleanup.go` — RoomCleanupService

```go
NewRoomCleanupService(roomRepo, recordingRepo, lkClient, egressClient, apiKey, apiSecret, uploadTracker)
```

| Fn | Purpose |
|----|---------|
| `CascadeDeleteRoom(ctx, room, CascadeDeleteOptions)` | Stop active egress → system msg → LK DeleteRoom → chat upload cleanup → `recordingRepo.DeleteByRoom` → `HardDeleteRoom` |
| `ArchiveRoom(ctx, room)` | Stop egress → system msg → LK DeleteRoom → chat uploads → deactivate participants → `SoftDeleteRoom` (**recordings preserved**) |
| `SuspendRoom(ctx, room)` | Stop egress → system msg → LK DeleteRoom → chat uploads → delete recording rows → `SetRoomIdle` + deactivate participants |
| `DeleteUserRooms(ctx, rooms, deletedUserID)` | Cascade each with owner-removed message |
| `BulkSuspendRooms` / `BulkCloseRooms` | Batch helpers; per-room error map |

```go
CascadeDeleteOptions{SystemEvent, SystemMessage, DeletedIdentity}
```

`cleanupRecordings` stops active egress via egress client (best-effort). Recording **file** deletion on cascade relies on DB row delete + separate retention; individual file wipe for cascade is not full S3/disk walk unless retention/store paths run.

---

## `internal/storage/recording_store.go` — RecordingStore (SHIPPED)

```go
type RecordingAttachment struct {
    URL  string `json:"url"`
    Size int64  `json:"size"`
}

type RecordingStore interface {
    Store(ctx, key string, src io.Reader, size int64) (*RecordingAttachment, error)
    Delete(ctx, key string) error
}

NewRecordingStore(cfg *RecordingConfig, s3Cfg *ChatUploadS3Config) RecordingStore
ExtractStorageKey(fileURL string) string  // disk or S3 URL → storage key
```

| Backend | When | URL shape |
|---------|------|-----------|
| Disk | default / incomplete S3 | `/recordings/{key}` under `storageDir` (default `./data/recordings`) |
| S3 | endpoint+bucket+accessKey set | `{publicBaseURL}/{key}` or `{endpoint}/{bucket}/{key}` |

Disk: path-traversal guard, temp+rename atomic write, optional max size, prune empty parent dirs on delete. S3: raw AWS SigV4 (shared helpers with chat upload); content-type `video/mp4` or `video/webm`.

### Recording config

```go
type RecordingConfig struct {
    MaxFileSizeMB        int // 0=unlimited; yaml default 2048
    StorageDir           string // default ./data/recordings
    MaxRecordingsPerRoom int // 0=unlimited; non-persistent rooms only
    RetentionHours       int // 0=forever; default 720 (30d). Env: RECORDING_RETENTION_HOURS
    CleanupIntervalHours int // 0→24 when retention on. Env: RECORDING_CLEANUP_INTERVAL_HOURS
}
```

---

## `internal/storage/chat_upload.go` — Chat Upload Storage

```go
type ChatAttachment struct {
    Kind, URL, Mime string
    Size int64
    Width, Height int  // json: w, h
}

ChatUploadStore interface { Store(data []byte) (*ChatAttachment, error) }
ObjectDeleter interface { DeleteObject(key string) error }
NewChatUploadStore(cfg *ChatUploadConfig) ChatUploadStore
NewS3Deleter(cfg *ChatUploadS3Config) ObjectDeleter
SniffMime / ContentHash / imageDimensions (incl. WebP)
```

| Backend | Rule |
|---------|------|
| `disk` / default | `hybridStore`: size < `InlineMaxBytes` (default 500KB) → data URI; else disk |
| `inline` | always base64 data URI |
| `s3` | small → inline; else SigV4 PUT; incomplete S3 → disk fallback |

MIME allowlist: png/jpeg/gif/webp. Filename = SHA256 + ext. Disk paths: `/uploads/chat/{hash}{ext}`.

### ChatUploadTracker

```go
NewChatUploadTracker(db, chatDir, deleter)
Record(roomID, userID, fileHash, ext, fileSize, backend) error
DeleteByRoom(roomID) error  // cross-room refcount on file_hash; S3/disk/inline
GetUserUploadBytes(userID) (int64, error)
GetTotalUploadBytes() (int64, error)  // 60s cache
```

---

## `internal/storage/avatar.go` — User avatars

Not queue-driven. Disk under `./data/uploads/avatars`. Max 2MB, max dim 1024. Same MIME allowlist. `SaveUserAvatar` / `DeleteUserAvatarFiles` / `ResolveAvatarFile`.

---

## Recording end-to-end (quick map)

```
RecordingService.StartRecording
  → models.Recording pending/started + LK RoomCompositeEgress (audio-only MP4)
LiveKit webhook egress_ended
  → status processing + Enqueue(process_recording)
process_recording handler
  → download (JWT if embedded LK) → RecordingStore.Store → completed
  → optional dispatch_webhook(recording.completed)
User ends meeting → room_delete Purge=false → ArchiveRoom (recordings kept)
Admin close → room_delete Purge=true → CascadeDeleteRoom
Scheduler retention → delete expired completed on archived rooms + empty rooms
Scheduler stale → DeleteStaleRecordings (7d pending/started/failed)
Bulk admin delete → MarkDeleting + Enqueue(recording_delete)
```
