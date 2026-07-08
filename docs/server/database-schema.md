# Database Schema

GORM models, table relationships, indexes, and migration behavior for the Bedrud backend.

**Migration entry:** `server/internal/database/migrations.go`  
**Models:** `server/internal/models/`

**Public docs:** [Database & Models](https://bedrud.org/en/docs/backend/database) — [`backend/database.mdx`](../../apps/site/src/content/docs/en/backend/database.mdx). Full map: [public-docs.md](./public-docs.md).

---

## Overview

| Engine | Use case | Notes |
|--------|----------|-------|
| **SQLite** | Local dev | `SetMaxOpenConns(1)` — serialized writes; queue uses two-step claim |
| **PostgreSQL** | Production | `FOR UPDATE SKIP LOCKED` queue claiming; manual FK constraints |

Migrations use GORM `AutoMigrate` plus custom SQL for partial indexes and Postgres FKs. Skip with `BEDRUD_SKIP_MIGRATE=1`.

---

## Entity relationship diagram

```
users ─────────────────────────────────────────────┐
  │                                               │
  ├── passkeys (user_id)                          │
  ├── user_preferences (user_id, 1:1)             │
  ├── blocked_refresh_tokens (user_id)            │
  ├── verification_events (user_id)               │
  │                                               │
  └── room_participants (user_id) ◄── rooms ──────┤
           │                            │         │
           └── room_permissions ────────┘         │
                                                   │
invite_tokens                                      │
chat_uploads ──► rooms (FK, Postgres CASCADE)     │
jobs (queue)                                       │
webhooks                                           │
system_settings (singleton, id=1)                  │
recordings ──► rooms (planned, wired in code)      │
```

---

## Tables

### `users`

| Column | Type | Notes |
|--------|------|-------|
| `id` | varchar(36) PK | UUID |
| `email` | varchar(255) | Unique with `provider` (`idx_email_provider`) |
| `name` | varchar(255) | Display name |
| `provider` | varchar(20) | `local`, `passkey`, `guest`, or OAuth provider |
| `avatar_url` | varchar(255) | |
| `password` | varchar(255) | Bcrypt hash; empty for OAuth/passkey/guest |
| `refresh_token` | text | Current refresh token hash |
| `accesses` | text[] / text | PostgreSQL array or comma-joined; levels: `superadmin`, `admin`, `moderator`, `user`, `guest` |
| `is_active` | bool | `false` = banned; loaded into in-memory ban set at startup |
| `email_verified_at` | timestamp | NULL = unverified |
| `password_changed_at` | timestamp | Invalidates tokens issued before change |
| `created_at`, `updated_at` | timestamp | |

### `blocked_refresh_tokens`

Tracks revoked refresh tokens (logout, force-logout, password change).

### `rooms`

| Column | Type | Notes |
|--------|------|-------|
| `id` | varchar(36) PK | |
| `name` | varchar(255) | URL slug; validated `^[a-z0-9]+(-[a-z0-9]+)*$`, 3–63 chars |
| `created_by` | varchar(36) | Creator user ID |
| `admin_id` | varchar(36) | Room admin (usually creator) |
| `is_active` | bool | `false` = suspended/idle |
| `is_public` | bool | Listed in public room list |
| `max_participants` | int | Default 20 |
| `expires_at` | timestamp | Optional expiry |
| `last_activity_at` | timestamp | Scheduler stale detection |
| `deleted_at` | timestamp | Soft archive; NULL = active |
| `mode` | varchar(20) | e.g. `standard` |
| `settings_*` | embedded | `allow_chat`, `allow_video`, `allow_audio`, `require_approval`, `e2ee`, `is_persistent`, `recordings_allowed` |

**Indexes:**

- `idx_rooms_name` — regular (non-unique) index on `name`
- `idx_rooms_active_name` — **partial unique** on `name` WHERE `is_active = true` (SQLite: `= 1`)

Archived rooms keep their names; new active rooms can reuse names after archive.

### `room_participants`

Composite unique `(room_id, user_id)`.

| Notable columns | Purpose |
|-----------------|---------|
| `is_active` | Currently in room |
| `is_approved` | Waiting room approval |
| `is_moderator` | Room-scoped mod (not global RBAC) |
| `is_muted`, `is_video_off`, `is_chat_blocked`, `is_banned` | Moderation state |
| `is_on_stage` | Stage mode |
| `left_at` | Last leave time |

### `room_permissions`

Per-participant permission overrides. Postgres FK `(room_id, user_id)` → `room_participants` ON DELETE CASCADE.

### `passkeys`

WebAuthn credentials: `credential_id`, `public_key`, `sign_count`, `user_id`, `name`.

### `system_settings`

Singleton row (`id = 1`). Stores admin-editable instance config: OAuth secrets, SMTP branding, CORS, LiveKit, quotas, email subject overrides. See [Settings System](./settings-system.md).

### `invite_tokens`

Registration invite codes: token hash, expiry, max uses, created by.

### `user_preferences`

Per-user JSON preferences (theme, locale, etc.) — 1:1 with user.

### `chat_uploads`

Tracks chat image uploads per room: storage backend (`disk`, `inline`, `s3`), size, path/key. Postgres FK `room_id` → `rooms` ON DELETE CASCADE.

### `jobs`

Internal async queue. See [Queue Deep Dive](./queue-deep-dive.md).

| Column | Notes |
|--------|-------|
| `type` | Handler key: `user_delete`, `room_delete`, etc. |
| `payload` | JSON string |
| `run_at` | Scheduled execution time |
| `priority` | Lower = higher priority |
| `status` | `pending`, `active`, `done`, `failed` |
| `attempts`, `max_attempts` | Retry tracking |
| `last_error` | Last failure message |

### `verification_events`

Audit log for email verification: `sent`, `verified`, `resent`, `admin_verified`, etc.

### `webhooks`

Outbound webhook endpoints: URL, secret, enabled events, active flag.

### `recordings` (planned)

Model and migrations exist; HTTP routes and queue handlers are not wired in `server.go`. See [Planned Features](./planned-features.md).

---

## Migration highlights

### Room name uniqueness evolution

Older versions had a global unique index on `rooms.name`. Migration:

1. Detects if `idx_rooms_name` is unique
2. Drops it and recreates as non-unique
3. Adds partial unique `idx_rooms_active_name` for active rooms only

Repository layer also checks name availability before create.

### Postgres-only foreign keys

SQLite cannot add composite FKs via `ALTER TABLE`. Postgres gets:

- `room_permissions` → `room_participants` CASCADE
- `chat_uploads` → `rooms` CASCADE
- `recordings` → `rooms` CASCADE (when enabled)

### AutoMigrate order

```
User → BlockedRefreshToken → Room → (index fix) → RoomParticipant →
RoomPermissions → Passkey → SystemSettings → InviteToken →
UserPreferences → ChatUpload → Job → VerificationEvent → Webhook → Recording
```

---

## Repository layer

Data access is isolated in `internal/repository/` — handlers should not call `database.GetDB()` directly.

| Repository | Primary model |
|------------|---------------|
| `UserRepository` | User, sessions, bulk ops |
| `RoomRepository` | Room, participants, permissions, events |
| `PasskeyRepository` | Passkey |
| `SettingsRepository` | SystemSettings + config merge |
| `InviteTokenRepository` | InviteToken |
| `UserPreferencesRepository` | UserPreferences |
| `WebhookRepository` | Webhook |
| `RecordingRepository` | Recording |
| `VerificationEventRepository` | VerificationEvent |

See [internal/repository.md](./internal/repository.md).

---

## Environment variables

| Variable | Effect |
|----------|--------|
| `BEDRUD_SKIP_MIGRATE=1` | Skip all migrations on startup |
| `DATABASE_*` | See [configuration.md](./configuration.md) |

---

## Related docs

- [internal/database.md](./internal/database.md) — connection init, SQLite vs Postgres
- [internal/models.md](./internal/models.md) — per-model field reference
- [Room Lifecycle](./room-lifecycle.md) — how room state maps to columns