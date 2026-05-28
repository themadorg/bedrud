---
name: bedrud-jobs
description: Async job queue, scheduler, cleanup services, chat upload storage.
license: Apache License
---

# Bedrud Job Queue & Background Tasks

Go module `bedrud`. `internal/queue/` + `internal/scheduler/` + `internal/services/` + `internal/storage/`.

---

## `internal/queue/` — Job Queue

Internal job queue for async task processing. Worker polls `jobs` table. Two DB backends: PostgreSQL uses `SKIP LOCKED`; SQLite uses two-step with serialized writes via `SetMaxOpenConns(1)`.

### Files

| File | Purpose |
|------|---------|
| `job.go` | 7 payload structs |
| `queue.go` | `Enqueue(ctx, db, jobType, payload, opts...)`, `Handler` type, `Worker` struct with `Start(ctx)`/`Stop()` |
| `worker.go` | Claim loop: poll 500ms, dispatch to handler, retry exponential backoff |
| `handler_user_delete.go` | Fetch user's rooms → `cleanupSvc.DeleteUserRooms` → delete passkeys → prefs → user |
| `handler_room_delete.go` | Fetch room → `cleanupSvc.CascadeDeleteRoom` |
| `handler_room_suspend.go` | Fetch room → `cleanupSvc.SuspendRoom` |
| `handler_chat_upload.go` | Decode base64 → `uploadStore.Store` → `uploadTracker.Record` |
| `handler_email.go` | Full SMTP sender with SMTPS/STARTTLS. Embedded templates: welcome, room_invite, password_reset, password_changed, verify_email, generic |
| `handler_dispatch_webhook.go` | HMAC-SHA256 POST, 10s timeout, soft-fail, no retry |
> **TODO oncoming feature:** Recording functionality is planned for a future release.
| `handler_process_recording.go` | Download file from LK → store → complete → webhook |

### Worker Options

`WorkerOptions{Interval: 500ms, Concurrency: 1}` (configurable via `QueueConfig`).

### Retry & Backoff

On failure: if `attempts >= maxAttempts` → `failed`. Else `run_at = now + (2^attempts * 5s)`, status stays `pending`. Default `MaxAttempts=3`. Backoff: 10s, 20s, 40s.

### Payloads

| Type | Struct | Priority |
|------|--------|----------|
| `user_delete` | `UserDeletePayload{UserID, Email, RoomIDs}` | 1 (HIGH) |
| `room_delete` | `RoomDeletePayload{RoomID, SystemEvent, SystemMessage, DeletedIdentity}` | 1 (HIGH) |
| `room_suspend` | `RoomSuspendPayload{RoomID}` | 2 (MEDIUM) |
| `chat_upload_s3` | `ChatUploadS3Payload{Data(base64), RoomID, MimeType, UserID}` | 0 (DEFAULT) |
| `send_email` | `SendEmailPayload{To, Subject, TemplateName, TemplateData}` | 0 (DEFAULT) |
| `dispatch_webhook` | `WebhookPayload{URL, Event, Body, Secret}` | 0 (DEFAULT) |
> **TODO oncoming feature:** Recording functionality is planned for a future release.
| `process_recording` | `ProcessRecordingPayload{RoomID, RoomName, EgressID, FileURL, ...}` | 0 (DEFAULT) |

### Config

```go
type QueueConfig struct {
    PollInterval ConfigInt // ms, default 500. Env: QUEUE_POLL_INTERVAL
    MaxAttempts  int       // default 3. Env: QUEUE_MAX_ATTEMPTS
    Concurrency  int       // default 1. Env: QUEUE_CONCURRENCY
}
```

---

## `internal/scheduler/scheduler.go` — gocron Tasks

`Initialize(db, roomRepo, userRepo, lkCfg, serverCfg)` — 5 params.
`Stop()` — Graceful stop.

| Interval | Task |
|----------|------|
| Every 1 min | `CleanupExpiredRooms` — bulk mark expired inactive (excludes persistent) |
| Every 1 min | `checkIdleRooms` — query LK participant counts → mark idle if 0 participants + >5min old. Skips persistent. Reactivates if participants join during check |
| Weekly (03:00) | `DeleteGuestUsers` — stale guests >7d |
| Daily (03:30) | `DeleteUnverifiedAccounts` — unverified local/passkey users (configurable TTL, default 48h) |
| Hourly | `CleanupBlockedTokens` — expired blocked refresh tokens |
| Hourly | `PruneRevokedTokens` — in-memory revoked token set cleanup |
| Daily (03:00) | Queue done >7d / failed >30d cleanup |
| Daily (09:00) | TLS cert expiry check → auto-renew self-signed certs when <30d |

---

## `internal/services/room_cleanup.go` — RoomCleanupService

Cross-cutting cascade delete/suspend for rooms and users.

`RoomCleanupService` struct: `roomRepo`, `livekit.RoomService` client, `apiKey`, `apiSecret`, `uploadTracker`.

| Fn | Purpose |
|----|---------|
| `NewRoomCleanupService(roomRepo, lkClient, apiKey, apiSecret, uploadTracker)` | Constructor |
| `CascadeDeleteRoom(ctx, room, reason, deletedIdentity)` | Close LK room → broadcast end msg → AdminDeleteRoom → chat upload tracker cleanup |
| `SuspendRoom(ctx, room)` | Close LK room → mark room inactive |
| `DeleteUserRooms(ctx, user, rooms)` | Iterate rooms, CascadeDeleteRoom each |

---

## `internal/storage/chat_upload.go` — Chat Upload Storage

`ChatUploadStore` interface: `Store(data []byte) (*ChatAttachment, error)`.
`ObjectDeleter` interface: `Delete(key string) error` — S3 impl: `S3Deleter`.

`ChatUploadTracker` methods: `Record(roomID, fileHash, ext)`, `DeleteByRoom(roomID)` (disk cleanup + DB row delete).

Backends: `disk`, `inline` (base64), `hybrid`, `s3` (raw AWS SigV4).
Factory: `NewChatUploadStore(cfg)` — by `cfg.Backend`.
Validation: MIME png/jpeg/gif/webp. SHA256 content hash filename.

```go
ChatAttachment {
    URL    string `json:"url"`
    Mime   string `json:"mime"`
    Size   int64  `json:"size"`
    Width  int    `json:"w"`
    Height int    `json:"h"`
}
```
