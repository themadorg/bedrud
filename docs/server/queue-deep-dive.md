# Queue Deep Dive

The Bedrud job queue: design, claiming algorithm, retries, job types, and handler implementation.

**Package:** `server/internal/queue/`  
**Model:** `server/internal/models/job.go`  
**Started in:** `server/internal/server/server.go`

**Public docs:** [Advanced Topics](https://bedrud.org/en/docs/backend/advanced) (queue overview) · [Code Structure](https://bedrud.org/en/docs/backend/structure) — [`backend/advanced.mdx`](../../apps/site/src/content/docs/en/backend/advanced.mdx). Full map: [public-docs.md](./public-docs.md).

---

## Design principles

- **Database-backed** — jobs table, no Redis/RabbitMQ
- **At-least-once delivery** — handlers must be idempotent where possible
- **Poll-based worker** — default 500ms interval, configurable concurrency
- **Depth cap** — 10,000 pending+active jobs max (safety net beyond bulk endpoint caps)
- **Separate failure modes** — retriable errors vs permanent failures vs advisory no-ops

---

## Job model

```go
type Job struct {
    ID          string    // UUID
    Type        string    // handler key
    Payload     string    // JSON
    RunAt       time.Time // scheduled execution
    Priority    int       // lower = higher priority
    Status      JobStatus // pending | active | done | failed
    Attempts    int
    MaxAttempts int       // default 3
    LastError   string
}
```

### Status lifecycle

```
pending ──claim──► active ──success──► done
                      │
                      ├── retryable error ──► pending (with backoff)
                      └── max attempts / permanent ──► failed
```

---

## Enqueue API

```go
queue.Enqueue(ctx, db, "room_delete", payload,
    queue.WithPriority(0),
    queue.WithMaxAttempts(3),
    queue.WithRunAt(time.Now().Add(delay)),
)
```

**Depth check:** Refuses enqueue if `COUNT(pending + active) >= 10000`.

**Bulk endpoints** additionally cap at 500 IDs per request.

---

## Worker architecture

```
Worker.Start()
  └─ N goroutines (Concurrency from config)
       └─ loop every Interval (default 500ms)
            ├─ claimNextJob()
            └─ handleJob() with 5-minute timeout + panic recovery
```

### Claim algorithm

| Database | Strategy |
|----------|----------|
| **PostgreSQL** | Single `UPDATE ... RETURNING` with subquery `FOR UPDATE SKIP LOCKED` |
| **SQLite** | Two-step UPDATE + SELECT; safe because `SetMaxOpenConns(1)` |

Ordering: `priority ASC, run_at ASC` — highest priority (lowest number) first.

### Retry backoff

On handler error (if `attempts < max_attempts`):

- Status → `pending`
- `run_at` += exponential backoff (capped)
- `last_error` stored

On unknown job type: **permanent fail** immediately (no retry — would never succeed).

On panic: caught, logged, marked as retriable error.

---

## Registered handlers (production)

From `server.go`:

| Job type | Handler | Purpose |
|----------|---------|---------|
| `user_delete` | `NewUserDeleteHandler` | Cascade delete user rooms, passkeys, prefs, uploads |
| `room_delete` | `NewRoomDeleteHandler` | `CascadeDeleteRoom` + remove DB row |
| `room_suspend` | `NewRoomSuspendHandler` | Delete LK room, set `is_active=false` |
| `chat_upload_s3` | `NewChatUploadS3Handler` | Async S3 upload for chat images |
| `send_email` | `NewSendEmailHandler` | SMTP transactional email |
| `dispatch_webhook` | `NewDispatchWebhookHandler` | Outbound HTTP webhook (advisory) |

### Planned (code exists, not registered)

| Job type | File | Status |
|----------|------|--------|
| `process_recording` | `handler_process_recording.go` | Commented out in server.go |
| `recording_delete` | `handler_recording_delete.go` | Commented out in server.go |

---

## Handler details

### `user_delete`

Payload: `{ "userId": "..." }`

1. Load user and owned rooms
2. For each room: `CascadeDeleteRoom`
3. Delete passkeys, preferences
4. Delete user row
5. Remove from ban cache

### `room_delete`

Payload: `{ "roomId": "...", "systemEvent": "...", ... }`

1. Load room
2. `CascadeDeleteRoom` (LK + uploads + participants)
3. Hard delete room row (or archive depending on options)

### `room_suspend`

Payload: `{ "roomId": "..." }`

1. Delete LiveKit room
2. Set `is_active=false`, notify participants via system message

### `chat_upload_s3`

Moves upload from temp/disk to S3, updates `chat_uploads` record.

### `send_email`

Payload (`SendEmailPayload`):

```json
{
  "to": "user@example.com",
  "templateName": "verify_email",
  "subject": "optional override",
  "templateData": { "verifyUrl": "...", "name": "..." }
}
```

Templates embedded in `queue/templates/*.html` and `*.txt`. If SMTP not configured: logs and returns success (no-op).

Subject resolution order: DB override → config → payload → hardcoded default.

### `dispatch_webhook`

Payload (`WebhookPayload`):

```json
{
  "url": "https://...",
  "secret": "hmac-secret",
  "event": "room.created",
  "body": { "roomId": "..." }
}
```

**Design:** Single attempt, all failures return `nil` (success). Webhooks are advisory — must not block room operations.

Envelope:

```json
{
  "event": "room.created",
  "timestamp": "2026-06-16T12:00:00Z",
  "data": { ... }
}
```

Headers: `X-Bedrud-Signature` (HMAC-SHA256), `X-Bedrud-Event`, `X-Bedrud-Timestamp`.

---

## Configuration

From `config.yaml` `queue` section:

| Field | Env var | Default |
|-------|---------|---------|
| `pollInterval` | `QUEUE_POLL_INTERVAL` | 500 (ms) |
| `maxAttempts` | `QUEUE_MAX_ATTEMPTS` | 3 |
| `concurrency` | `QUEUE_CONCURRENCY` | 1 |

SQLite note: concurrency > 1 may require wrapping SQLite claim in `BEGIN IMMEDIATE` transaction.

---

## Monitoring

`GET /api/admin/queue/stats` — pending/active/done/failed counts via `AdminQueueHandler`.

Admin overview dashboard includes queue pending count in KPIs.

---

## Scheduler cleanup

`internal/scheduler/` periodically deletes old `done`/`failed` job rows to prevent unbounded table growth.

---

## Adding a new job type

1. Define payload struct in handler file
2. Implement `func(ctx, db, job) error` handler
3. Register in `server.go` worker map
4. Call `queue.Enqueue` from handler with appropriate options
5. Return `202 Accepted` from HTTP handler
6. Add tests in `handler_*_test.go`

Recipe: [developer-guide.md](./developer-guide.md)

---

## Related docs

- [internal/queue.md](./internal/queue.md) — package overview
- [email-webhooks.md](./email-webhooks.md) — email and webhook specifics
- [room-lifecycle.md](./room-lifecycle.md) — what triggers room jobs
- [architecture.md](./architecture.md) — async vs sync patterns