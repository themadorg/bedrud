---
name: bedrud-server
description: Full Go backend code map. Load for any server/API/auth/DB/model/handler work.
license: Apache License
---

# Bedrud Server — Full Backend Map

Go module `bedrud`. Root: `server/`. Fiber HTTP + GORM + embedded LiveKit + job queue.

**Leaf skills** (deeper detail when needed):

| Skill | Scope |
|-------|--------|
| `bedrud-http` | Entrypoints, bootstrap, handlers, routes, lkutil |
| `bedrud-auth` | AuthService, JWT, middleware, rate limits |
| `bedrud-data` | Models, repos, migrations, testutil |
| `bedrud-jobs` | Queue, scheduler, cleanup, recording service/store, chat storage |
| `bedrud-realtime` | Embedded LiveKit binary lifecycle |
| `bedrud-ops-cli` | Cobra CLI, installer, usercli/roomcli, utils |
| `bedrud-email-cerberus` | Email templates + send_email branding |

---

## Entrypoints

### `cmd/bedrud/main.go` — Production CLI

Thin wrapper → `cli.Execute(version)` (Cobra in `internal/cli/`). Legacy flags still work (`--run`, `--livekit`, `--version`).

| Command | Calls | Purpose |
|---------|-------|---------|
| `run` / `server` | `server.Run(configPath, version)` | Full app (API + optional embedded LK + SPA) |
| `livekit` / `--livekit` | `livekit.RunLiveKit(configPath)` | Standalone LK binary |
| `install` / `uninstall` | `install.LinuxInstall` / `LinuxUninstall` | Linux installer (systemd/OpenRC/SysV) |
| `user *` | `usercli.*` | create/delete/promote/demote/list/info/password/enable/disable |
| `room *` | `roomcli.*` | list/info/close/suspend/reactivate/kick |
| `config` / `settings` / `invite-token` / `cert` / `db` | respective CLI | Ops tooling |
| `version` | print | Version string |

Config: `--config`, `BEDRUD_CONFIG`, or `CONFIG_PATH`. Management cmds default `/etc/bedrud/config.yaml`.

### `cmd/server/main.go` — Dev API (Air)

No Cobra. Wires subsystems + routes + Swagger/Scalar + SPA. Minor drift vs prod (`GET /admin/stats` still registered here; prod uses overview). Prefer `internal/server/server.go` as source of truth.

Health: `GET /api/health` · Ready: `GET /api/ready`. Docs: `/api/swagger/*`, `/api/scalar`. Disable: `DISABLE_API_DOCS`.

### `ui.go` — Frontend embed

`//go:embed all:frontend` → `UI embed.FS`. `make build` copies `apps/web/build/*` → `server/frontend/`.

---

## Package inventory (`server/internal/`)

| Package | Role |
|---------|------|
| `server/` | Prod bootstrap `Run(configPath, version)` |
| `handlers/` | HTTP handlers (auth, room, users, admin, recording, webhook, cert, queue) |
| `auth/` | AuthService, JWT, session store, challenge store, email canonicalize |
| `middleware/` | Protected, RBAC, CSRF bearer-for-mutations, guest reject, email-verify, rate limit, recordings gate |
| `models/` | GORM models + admin/overview DTOs |
| `repository/` | Data access (user, room, passkey, settings, invite, prefs, verification, webhook, recording) |
| `database/` | GORM init (SQLite/Postgres), AutoMigrate + PG FKs |
| `queue/` | Job enqueue/worker + 8 handlers + email templates |
| `scheduler/` | gocron: idle rooms, guests, unverified, tokens, queue cleanup, cert, recording retention |
| `services/` | `RoomCleanupService`, `RecordingService` |
| `storage/` | Chat upload, avatar, recording store |
| `livekit/` | Embedded LK binary + temp YAML + process mgmt |
| `lkutil/` | RoomService/Egress clients, AuthContext, system messages |
| `cli/` | Cobra command surface |
| `clioutput/` | `--json` envelopes |
| `usercli/` / `roomcli/` | CLI domain ops |
| `install/` | LinuxInstall / LinuxUninstall |
| `utils/` | TLS certs, SMTP, keys, safeio, net |
| `testutil/` | In-memory SQLite + LiveKit mocks |
| `templates/` | Legacy HTML (login/index) — SPA usually embedded |

---

## Bootstrap (`internal/server/server.go`)

`Run(configPath, version) error` sequence:

1. Load + validate config (JWT/session secrets)
2. Start embedded LiveKit when internal (localhost InternalHost) — or log external
3. Session store · DB · migrations
4. Repos (`room`, `user`, `recording`, …)
5. Scheduler (`recordingRepo` live; `recStore`/`recCfg` currently `nil` until store wired)
6. Auth `Init` + hydrate ban set from inactive users
7. Fiber: recover, helmet, CORS (credentials forbid `*`)
8. LK reverse-proxy `/livekit` when internal
9. Upload tracker, LK client, cleanup service
10. Queue worker (6 handlers registered today; recording handlers commented)
11. Handlers + `/api` routes + static uploads + SPA
12. TLS (ACME / manual / plain) + optional HTTP on `httpPort`
13. Graceful shutdown SIGINT/SIGTERM

**Recording bootstrap gap (important):** handler, service, repo, store, queue types, and webhook processing are **implemented**, but prod `server.Run` still has commented blocks for:

- `NewRecordingStore` / egress client / `RecordingService` / `RecordingHandler`
- Queue map entries `process_recording`, `recording_delete`
- Room-scoped routes under `/api/rooms/:id/...` and admin recording routes
- Static recording file serving

Wire same pattern as integration tests when enabling. Jobs enqueued without a registered handler → permanent fail.

---

## Path convention

All API under Fiber group `/api` (`@BasePath /api`).

| Area | Prefix | Notes |
|------|--------|-------|
| Room lifecycle + moderation | **`/api/room/...` (singular)** | create, join, list, `:roomId/*` |
| Recording | **`/api/rooms/:id/...` (plural)** | SHIPPED code; routes still commented in bootstrap |
| Auth | `/api/auth/...` | register, login, passkeys, verify, preferences, OAuth |
| Admin | `/api/admin/...` | superadmin group |
| LiveKit webhook | `POST /api/livekit/webhook` | LK JWT, not app JWT |
| Static | `/uploads/chat/*` (auth), `/uploads/avatars/*` (public) | |
| Embedded LK proxy | `/livekit/*` | internal only |

Admin group middleware: `Protected` → `RequireEmailVerified` → `RequireAccess(superadmin)`.

---

## Route map (prod)

```
/api/health  /api/ready
/api/auth/*     register, login, guest-login, refresh, logout
                me, me/avatar, password, verify*, forgot/reset
                passkey/*, preferences, settings (public)
                :provider/login|callback
/api/room/*     create, join, guest-join, refresh-token, list, archived
                :roomId/{kick,mute,ban,video,promote,demote,chat,deafen,
                         undeafen,ask,spotlight,screenshare,presence,
                         participant,stage,settings,chat/upload}
                DELETE :roomId  → 202 room_delete
/api/rooms/:id/recording*   (IMPLEMENTED; bootstrap routes commented)
/api/admin/*    users*, rooms*, settings*, invite-tokens, webhooks*
                overview, queue, cert-info, online-count, livekit/stats
                recordings* (IMPLEMENTED; bootstrap commented)
/api/livekit/webhook
/api/cert
/uploads/chat/*  /uploads/avatars/*
/livekit/*      (internal LK reverse proxy)
```

### HTTP 501 stubs only

| Handler | Route |
|---------|-------|
| `BringToStage` | `POST /api/room/:roomId/stage/:identity/bring` |
| `RemoveFromStage` | `POST /api/room/:roomId/stage/:identity/remove` |
| `AdminGenerateToken` | `POST /api/admin/rooms/:roomId/token` |

Repo stage helpers (`BringToStage`/`RemoveFromStage` on participants) exist; HTTP layer returns 501.

Do **not** call email, webhook, or recording “stubs” — they are implemented at handler/service/queue layer.

---

## Handlers (`internal/handlers/`)

| File | Responsibility |
|------|----------------|
| `auth.go` | OAuth begin/callback (goth) |
| `auth_handler.go` | Local auth, passkeys, profile/avatar, verify, password reset |
| `room.go` | Room lifecycle, moderation, chat upload, admin room ops |
| `recording_handler.go` | Start/stop/list/get/wait/clear + admin list/bulk delete → `RecordingService` |
| `users.go` | Admin users, bulk, force-logout, verify, async delete |
| `admin_handler.go` | Settings, invite tokens, webhooks, test email, validate |
| `preferences_handler.go` | User JSON prefs ≤4KB |
| `admin_overview.go` | Aggregated KPIs / health |
| `admin_queue.go` | Queue stats + email diagnostics |
| `cert_handler.go` | Cert PEM download + admin cert-info |
| `livekit_webhook.go` | `participant_left`, `room_finished`, `egress_started`, `egress_ended` |
| `cooldown.go` / `errors.go` / `room_auth.go` / `models.go` | Shared helpers + DTOs |

### Auth routes (summary)

| Method | Path | Notes |
|--------|------|-------|
| POST | `/auth/register`, `/login`, `/guest-login`, `/refresh` | Rate limited |
| POST | `/auth/logout` | Protected |
| GET/PUT | `/auth/me` | Protected + email verified |
| POST/DELETE | `/auth/me/avatar` | Avatar upload |
| PUT | `/auth/password` | Change password |
| POST/GET | `/auth/verify`, `/verify/status`, `/verify/resend` | Email verification |
| POST | `/auth/forgot-password`, `/reset-password` | Reset flow |
| POST | `/auth/passkey/*` | Register/login/signup begin+finish |
| GET/PUT | `/auth/preferences` | Prefs |
| GET | `/auth/settings` | Public flags (incl. recordingsEnabled) |
| GET | `/auth/:provider/login\|callback` | OAuth |

### Room routes (singular `/room`)

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/room/create` | Create LK + DB |
| POST | `/room/join`, `/guest-join` | Join + LK token |
| POST | `/room/refresh-token` | Refresh LK JWT |
| GET | `/room/list`, `/archived` | User rooms / archives |
| POST | `/room/:roomId/{kick,mute,ban,...}/:identity` | Moderation |
| GET | `/room/:roomId/presence` | Presence (API rate limit) |
| PUT | `/room/:roomId/settings` | Settings (incl. recordingsAllowed) |
| DELETE | `/room/:roomId` | 202 · `room_delete` (archive, Purge=false) |
| POST | `/room/:roomId/chat/upload` | Chat image |

### Recording routes (plural `/rooms` — code ready)

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/rooms/:id/recording/start\|stop` | Egress control · RecordingsEnabled |
| GET | `/rooms/:id/recordings`, `.../:rid`, `.../:rid/wait` | List/get/long-poll |
| DELETE | `/rooms/:id/recordings`, `.../:recordingId` | Clear |
| GET | `/admin/recordings` | Admin list |
| POST | `/admin/recordings/bulk/delete` | 202 · `recording_delete` |

### Admin / other

Users: list, recent, bulk ban/promote/delete, status, accesses, force-logout, password, detail, sessions, verify, delete.  
Rooms admin: list, events, close/suspend/reactivate/update, participants, bulk, livekit stats, online-count.  
Settings/webhooks/invite-tokens/overview/queue/cert as in route map.

---

## Auth (`internal/auth/` + `middleware/`)

### AuthService

Password: `HashPassword` / `VerifyPassword` — `bcrypt(sha256(password))` with legacy plain-bcrypt fallback.  
Flows: Register, Login (email-verify gate), GuestLogin, refresh rotate (`RotateRefreshToken`), Logout (block refresh + revoke access), profile/avatar/email/password, passkeys (WebAuthn), OAuth Init/ReloadProviders.

### JWT (`jwt.go`)

- Access + 7d refresh (`GenerateTokenPair`); purpose tokens: `email_verify`, `password_reset`
- In-memory access revocation + ban set (process-local; refresh blocklist is DB-hashed)

### Middleware

| Fn | Behavior |
|----|----------|
| `Protected` | Bearer or `access_token` cookie → claims in `Locals("user")`; ban → 403 |
| `RequireAccess` | Hierarchical: superadmin > admin > moderator > user |
| `RequireBearerForMutations` | CSRF: mutations need Authorization header |
| `RejectGuest` | Block guest provider |
| `RequireEmailVerified` | Always DB `EmailVerifiedAt` (never trust JWT claim) |
| `Auth/Resend/Guest/APIRateLimiter` | Per-IP Fiber limiters |
| `RecordingsEnabled` | 403 if system flag off |

---

## Models summary (`internal/models/`)

| Model | Table / notes |
|-------|----------------|
| `User` | email+provider unique; Accesses text[]; EmailVerifiedAt; PasswordChangedAt; refresh stored as SHA-256 |
| `Room` | Settings embedded (`settings_*` incl. RecordingsAllowed, IsPersistent); LastActivityAt; soft `DeletedAt` archive; active-name unique partial index |
| `RoomParticipant` / `RoomPermissions` | Stage, moderator, ban, chat block flags |
| `Passkey` | WebAuthn credentials |
| `BlockedRefreshToken` | Hashed refresh blocklist |
| `SystemSettings` | Singleton ID=1 — registration, OAuth, server, LK, CORS, chat, email, recordings, limits (runtime overlay via `GetEffectiveSettings`) |
| `InviteToken` | Gated registration |
| `UserPreferences` | JSON blob PK=userID |
| `ChatUpload` | Room/user upload metadata (no dedicated repo) |
| `Job` | Queue rows: pending/active/done/failed |
| `VerificationEvent` | sent/resent/success/failed/admin_force/email_change |
| `Webhook` | Outbound subscriptions; events: room.created/ended, participant.joined, recording.completed, ping |
| `Recording` | LiveKit egress lifecycle: pending→started→processing→completed\|failed\|deleting |
| `QueueStats` / overview DTOs | Admin API shapes (not tables) |

Relationships: User→Passkeys/BlockedTokens/Prefs/Participants; Room→Participants/Permissions/ChatUploads/Recordings; Job & Webhook standalone; SystemSettings singleton.

---

## Repositories (`internal/repository/`)

| Repo | Highlights |
|------|------------|
| `UserRepository` | Upsert, hashed refresh CAS, cascade delete, admin filters/batch, guest/unverified cleanup |
| `RoomRepository` | Create/join capacity, stage/mod flags, soft/hard delete, admin filters, KPIs/trends |
| `PasskeyRepository` | CRUD + counter advance |
| `SettingsRepository` | Get/Save/GetEffectiveSettings |
| `InviteTokenRepository` | Create/list/mark used/delete |
| `UserPreferencesRepository` | Get/Upsert/Delete |
| `VerificationEventRepository` | Record + recent by user |
| `WebhookRepository` | CRUD, ListActive(event), secret rotate |
| `RecordingRepository` | Optimistic status transitions, list/admin, stale/retention queries |

No SQL migration dir — schema via `database.RunMigrations()` AutoMigrate + Postgres FKs. Skip: `BEDRUD_SKIP_MIGRATE=1`.

---

## Queue (`internal/queue/`) — 8 job types

Worker polls `jobs`. Postgres `SKIP LOCKED`; SQLite two-step + `MaxOpenConns(1)`. Default poll 500ms, concurrency 1, maxAttempts 3, backoff `2^n * 5s` cap 1h. Depth cap 10000.

| Type | Payload | Handler | Status |
|------|---------|---------|--------|
| `user_delete` | UserID, Email, RoomIDs | `NewUserDeleteHandler` | **Registered** |
| `room_delete` | RoomID, msgs, **Purge** | archive (`false`) or cascade (`true`) | **Registered** |
| `room_suspend` | RoomID | `SuspendRoom` | **Registered** |
| `chat_upload_s3` | base64 + meta | Store + tracker | **Registered** |
| `send_email` | To, Subject, TemplateName, Data | SMTP + Cerberus templates | **Registered** |
| `dispatch_webhook` | URL, Event, Body, Secret | HMAC POST (soft-fail) | **Registered** |
| `process_recording` | egress/file meta | Download → RecordingStore → complete + webhooks | **Implemented; bootstrap commented** |
| `recording_delete` | RecordingID, RoomID, RoomName | Hard-delete DB row | **Implemented; bootstrap commented** |

`handler_stubs.go` is an index comment only — **no stub handlers remain**.

Email templates: `welcome`, `verify_email`, `password_reset`, `password_changed`, `room_invite` (registered, not yet enqueued by handlers), `generic`. Branding from config + SystemSettings.

---

## Services & storage

### `RoomCleanupService`

| Fn | Behavior |
|----|----------|
| `CascadeDeleteRoom` | Stop egress → system msg → LK delete → chat cleanup → delete recordings → HardDeleteRoom |
| `ArchiveRoom` | Soft-delete; **keeps recordings** |
| `SuspendRoom` | Idle + disconnect; clears recording rows |
| `DeleteUserRooms` | Cascade each owned room |

### `RecordingService` (SHIPPED)

Gates: system RecordingsEnabled → room exists → room RecordingsAllowed → max per room → no active.  
`StartRecording` → RoomCompositeEgress MP4 **AudioOnly=true**.  
`StopRecording` → StopEgress only; webhook `egress_ended` owns processing enqueue.

Lifecycle: `pending → started → (webhook) processing → process_recording → completed|failed`.

### Storage

| Component | Role |
|-----------|------|
| `ChatUploadStore` | disk/hybrid/inline/s3 (SigV4) |
| `ChatUploadTracker` | Record + DeleteByRoom + quotas |
| `Avatar` | `./data/uploads/avatars`, max 2MB |
| `RecordingStore` | Disk or S3; key `recordings/{user}/{room}/{id}-….mp4` |

---

## Scheduler (`internal/scheduler/`)

| Cadence | Task |
|---------|------|
| 1 min | Expired rooms + idle LK empty rooms (skip persistent) |
| Weekly 03:00 | Delete old guests (7d) |
| Daily 03:30 | Unverified local/passkey accounts |
| Hourly | Blocked refresh tokens + prune revoked access JWTs |
| Daily 03:00 | Queue cleanup (done 7d / failed 30d); stale recordings 7d |
| Daily 09:00 | Self-signed cert renew if ≤30d |
| `CleanupIntervalHours` | Recording retention on archived rooms (needs recStore) |

---

## LiveKit

### Embedded (`internal/livekit/`)

- Embed `bin/livekit-server` (placeholder OK for build; real binary for run)
- `StartInternalServer` / `RunLiveKit`; skip if `LIVEKIT_MANAGED=true`
- Temp YAML: keys, node IP / STUN, TURN TLS 5349 + UDP 3478 when server TLS, auto webhook `http://localhost:<httpPort>/api/livekit/webhook`
- Hardcoded embedded signaling port **7880**; `/livekit` reverse-proxy when internal

### Clients (`internal/lkutil/`)

`NewClient` (RoomService), `NewEgressClient`, `AuthContext`, `SendSystemMessage` (+ deletedIdentity variant).

### Webhook (`handlers/livekit_webhook.go`)

Verifies LK JWT. Handles participant/room lifecycle + egress → may enqueue `process_recording`. External LK: configure webhook URL manually to `/api/livekit/webhook`.

---

## CLI & install (`internal/cli/`, `usercli/`, `roomcli/`, `install/`)

Cobra root: `run`, `livekit`, `install`/`uninstall`, `cert`, `user`, `room`, `config`, `settings`, `invite-token`, `db`, `version`.  
`--json` via `clioutput`. Installer: `LinuxInstall` — multi-init (systemd/OpenRC/SysV/container), secrets, `/etc/bedrud`, `/var/lib/bedrud`, optional ACME/self-signed/external LK.

`usercli`: promote/demote/create/delete/list/info/password/active with full cascade delete via cleanup service.  
`roomcli`: list/info/close/suspend/reactivate/kick.

Utils: TLS multi-algo certs, SMTP, LiveKit keypair, SafeCreate, OutboundIP.

---

## Dependency graph

```
cmd/bedrud → internal/cli
               ├─ server.Run → handlers → repository → models
               │              → auth → repository
               │              → middleware → auth
               │              → queue → services / storage / email
               │              → scheduler → repository + livekit + auth prune
               │              → services (cleanup, recording)
               │              → storage (chat, avatar, recording)
               │              → livekit (embed process)
               │              → lkutil (RoomService / Egress)
               ├─ livekit.RunLiveKit
               ├─ install.LinuxInstall
               ├─ usercli / roomcli → repository + services + storage + lkutil
               └─ utils / database / clioutput

cmd/server → same wiring inline (dev; Swagger)
```

---

## Config touchpoints (high level)

| Area | Keys / env |
|------|------------|
| Server | host, port, httpPort, TLS, ACME, behindProxy, trustedProxies |
| Auth | JWTSecret, TokenDuration, RequireEmailVerification, OAuth, passkey TTL |
| Database | type sqlite\|postgres, path/DSN, pool |
| LiveKit | host, internalHost, apiKey/secret (≥32 embedded), external, nodeIP, configPath |
| Queue | pollInterval, maxAttempts, concurrency |
| Email | SMTP + template branding |
| Chat | uploads backend/disk/s3/inlineMax |
| Recording | maxFileSizeMB, storageDir, maxPerRoom, retentionHours |
| RateLimit | auth/guest/api/resend max + window |

Runtime overrides: many via `SystemSettings` + `GetEffectiveSettings`.

---

## Gotchas

1. **Singular vs plural paths:** rooms API is `/api/room/*`; recording is `/api/rooms/:id/*`.
2. **Recording is shipped but not fully wired in bootstrap** — uncomment store/egress/handler/routes/queue entries to go live.
3. **Only stage + AdminGenerateToken are HTTP 501** — email/webhook/recording are real.
4. **Refresh tokens hashed** in DB/blocklist — never store raw.
5. **Room soft-delete** archives (`DeletedAt`); hard delete via `HardDeleteRoom` / purge jobs. No `AdminDeleteRoom` name.
6. **Active room names unique** only (`idx_rooms_active_name`); idle/archived can reuse names.
7. **Email verify middleware always hits DB** — JWT `EmailVerifiedAt` is client-facing only.
8. **Ban set + access revoke are process-local**; multi-instance needs shared store for those.
9. **Queue + SQLite:** serialized writes (`MaxOpenConns(1)`); pending jobs stuck → check DB/worker map.
10. **Async APIs return 202** for delete/suspend/bulk — work runs in queue handlers.
11. **CLI is Cobra** (`internal/cli`), not manual arg switch in `main`.
12. **Installer is `LinuxInstall`**, not Debian-only.
13. **Embedded LK placeholder** must exist for embed; real binary from `make init`.
14. **CORS + credentials** refuse wildcard origins at startup.

---

## Verification

```bash
cd server && go vet ./...
cd server && go build ./...
cd server && go test -v -count=1 ./...
# race (CI): go test -race ./...
```

Swagger regen: `make swagger-gen` (swag CLI). UI: `/api/swagger`, `/api/scalar`.
