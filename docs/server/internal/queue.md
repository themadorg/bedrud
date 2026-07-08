# Job Queue (`internal/queue`)

Internal async job queue. API endpoints return `202 Accepted` and enqueue work for background processing.

---

## Architecture

```
API handler → Enqueue(jobType, payload) → jobs table
                                              ↓
Worker (poll 500ms) → claim job → dispatch handler → done/failed/retry
```

Two DB backends with different claim algorithms:

| Backend | Claim strategy |
|---------|---------------|
| PostgreSQL | `SELECT ... FOR UPDATE SKIP LOCKED` |
| SQLite | Two-step claim with `SetMaxOpenConns(1)` |

---

## Core types (`queue.go`)

```go
type Handler func(ctx context.Context, db *gorm.DB, job *models.Job) error

type Worker struct {
    db       *gorm.DB
    handlers map[string]Handler
    opts     WorkerOptions
}
```

### `Enqueue(ctx, db, jobType, payload, opts...)`

Inserts a `Job` row with priority and retry options.

### Worker options

```go
type WorkerOptions struct {
    Interval    time.Duration  // default 500ms
    Concurrency int            // default 1
}
```

Configurable via `QueueConfig` in config.yaml.

---

## Worker (`worker.go`)

Poll loop:

1. Poll every `pollInterval` (default 500ms)
2. Claim pending jobs where `run_at <= now`
3. Dispatch to registered handler
4. On success → status `done`
5. On failure → retry with exponential backoff or mark `failed`

### Retry and backoff

- Default `maxAttempts`: 3 (1 original + 2 retries)
- Backoff: `2^attempts * 5s` (10s, 20s, 40s), capped at 1 hour
- Failed jobs kept for 30 days, done jobs for 7 days (scheduler cleanup at 03:00)

Worker drains all available jobs per tick (inner loop until nil).

---

## Job types and payloads (`job.go`)

| Type | Payload struct | Priority | Handler file |
|------|---------------|----------|--------------|
| `user_delete` | `UserDeletePayload` | 1 (HIGH) | `handler_user_delete.go` |
| `room_delete` | `RoomDeletePayload` | 1 (HIGH) | `handler_room_delete.go` |
| `room_suspend` | `RoomSuspendPayload` | 2 (MEDIUM) | `handler_room_suspend.go` |
| `chat_upload_s3` | `ChatUploadS3Payload` | 0 (DEFAULT) | `handler_chat_upload.go` |
| `send_email` | `SendEmailPayload` | 0 | `handler_email.go` |
| `dispatch_webhook` | `WebhookPayload` | 0 | `handler_dispatch_webhook.go` |
| `process_recording` | `ProcessRecordingPayload` | 0 | `handler_process_recording.go` |
| `recording_delete` | `RecordingDeletePayload` | 1 | `handler_recording_delete.go` |

### RoomDeletePayload

Includes `Purge` flag:
- `Purge=true` — hard-delete room + recording rows and files
- `Purge=false` — archive room (soft-delete, recordings preserved)

---

## Handler details

### `handler_user_delete.go`

1. Fetch user's rooms
2. `cleanupSvc.DeleteUserRooms` for each
3. Delete passkeys, preferences, user record

### `handler_room_delete.go`

1. Fetch room
2. `cleanupSvc.CascadeDeleteRoom` (close LK → broadcast → DB delete → upload cleanup)

### `handler_room_suspend.go`

1. Fetch room
2. `cleanupSvc.SuspendRoom` (close LK → mark inactive)

### `handler_chat_upload.go`

Decode base64 payload → `uploadStore.Store` → `uploadTracker.Record`

### `handler_email.go`

Render Cerberus HTML template from `queue/templates/` → send via SMTP

### `handler_dispatch_webhook.go`

POST to webhook URL with HMAC signature

### `handler_process_recording.go`

Process egress output → store in `RecordingStore` → update DB

### `handler_recording_delete.go`

Delete recording file + DB row

---

## Email templates (`internal/queue/templates/`)

Cerberus hybrid HTML + plain-text pairs:

| Template | HTML | Text | Purpose |
|----------|------|------|---------|
| `verify_email` | ✓ | ✓ | Email verification link |
| `welcome` | ✓ | ✓ | Post-verification welcome |
| `password_reset` | ✓ | ✓ | Password reset link |
| `password_changed` | ✓ | ✓ | Password change notification |
| `room_invite` | ✓ | ✓ | Room invitation |
| `generic` | ✓ | ✓ | Fallback template |
| `cerberus-hybrid.html` | ✓ | — | Base Cerberus layout |

Branding (colors, instance name, subject lines) from `config.email.templates` or `SystemSettings` DB overrides.

---

## Admin visibility

`GET /api/admin/queue` returns `QueueStats`: pending, active, done24h, failed24h, recent failures, throughput metrics.