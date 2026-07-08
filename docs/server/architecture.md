# Backend Architecture

Deep dive into how the Bedrud Go backend is wired: startup, request flow, dependency graph, and async subsystems.

**Source of truth:** `server/internal/server/server.go` (`Run()`)

**Public docs:** [Server Architecture](https://bedrud.org/en/docs/architecture/server) · [Backend Overview](https://bedrud.org/en/docs/backend/overview) · [Advanced Topics](https://bedrud.org/en/docs/backend/advanced) — [`apps/site/src/content/docs/en/`](../../apps/site/src/content/docs/en/). Full map: [public-docs.md](./public-docs.md).

---

## High-level diagram

```
                    ┌─────────────────────────────────────────┐
                    │              bedrud binary               │
                    │  cmd/bedrud run  →  server.Run()         │
                    └────────────────────┬────────────────────┘
                                         │
         ┌───────────────────────────────┼───────────────────────────────┐
         │                               │                               │
         ▼                               ▼                               ▼
  ┌─────────────┐               ┌──────────────┐               ┌──────────────┐
  │ Fiber HTTP  │               │ Embedded SPA │               │ LiveKit      │
  │ /api/*      │               │ (ui.go)      │               │ embedded or  │
  │ /uploads/*  │               │              │               │ external     │
  └──────┬──────┘               └──────────────┘               └──────┬───────┘
         │                                                            │
         │  middleware → handlers → repos → GORM → DB                 │
         │                                                            │
         ├──────────────── Queue Worker (async jobs) ────────────────┤
         ├──────────────── Scheduler (idle cleanup, job purge) ──────┤
         └──────────────── LiveKit webhook → handlers ────────────────┘
```

---

## Startup sequence (`Run()`)

Order matters — later steps depend on earlier initialization.

| Step | What happens |
|------|----------------|
| 1 | Load `config.yaml` via `config.Load()` |
| 2 | Configure zerolog level and console output |
| 3 | Validate `jwtSecret` (required, warn if &lt; 32 chars) and `sessionSecret` |
| 4 | Warn if email verification enabled but SMTP missing |
| 5 | Validate TLS cert pair if TLS enabled (non-ACME) |
| 6 | Start embedded LiveKit if `livekit.external=false` and internal host is localhost |
| 7 | `auth.InitializeSessionStore()` for OAuth/passkey sessions |
| 8 | `database.Initialize()` + `RunMigrations()` |
| 9 | Create repositories (room, user, recording, passkey, settings, invite, prefs, webhook) |
| 10 | `scheduler.Initialize()` — background idle-room + job cleanup |
| 11 | `auth.Init()` — JWT signing, Goth OAuth providers, ban cache |
| 12 | Load inactive user IDs into in-memory ban set |
| 13 | Create Fiber app with trusted-proxy config |
| 14 | Mount `/livekit` reverse proxy (embedded LK only) |
| 15 | Global middleware: recover, helmet, CORS |
| 16 | Wire storage (chat uploads), LiveKit client, `RoomCleanupService` |
| 17 | Start queue worker with registered job handlers |
| 18 | Instantiate all HTTP handlers and register routes |
| 19 | Mount Swagger/Scalar, SPA filesystem, TLS/ACME listener |
| 20 | Block on SIGINT/SIGTERM, graceful shutdown |

See [Bootstrap](./bootstrap.md) for TLS, ACME, and shutdown details.

---

## Request lifecycle

```
Client
  │
  ▼
Fiber (recover, helmet, CORS)
  │
  ▼
Route group /api
  │
  ├─ Rate limiter (auth, guest, API, resend) — optional per route
  ├─ Protected() — JWT from Authorization header or access_token cookie
  ├─ RequireEmailVerified() — blocks unverified users on protected routes
  ├─ RequireAccess(superadmin) — hierarchical RBAC for /api/admin/*
  │
  ▼
Handler method (internal/handlers/)
  │
  ├─ Parse/validate request body (Fiber Bind)
  ├─ Authorize (claims, room moderator check, admin access)
  ├─ Repository calls (GORM)
  ├─ LiveKit API calls (lkutil client)
  ├─ Enqueue async job (queue.Enqueue) → return 202
  │
  ▼
JSON response (fiber.Map or typed DTO)
```

### JWT context

After `middleware.Protected()`, handlers read:

```go
claims := c.Locals("user").(*auth.Claims)
// claims.UserID, claims.Email, claims.Accesses, claims.Provider
```

### Room moderation authorization

Room-scoped actions (kick, mute, promote, etc.) use `isRoomModerator()` in `handlers/room_auth.go`:

1. **Superadmin** — global bypass
2. **Room owner** — `AdminID` or `CreatedBy`
3. **Room moderator** — `room_participants.is_moderator = true` for that room

---

## Dependency graph

```
server.Run()
├── config.Config
├── database.DB
│   └── repositories.*
├── auth.AuthService
│   ├── userRepo, passkeyRepo
│   └── JWT, OAuth (Goth), ban cache, challenge store
├── lkutil.Client → LiveKit RoomService
├── services.RoomCleanupService
│   ├── roomRepo, lkClient, uploadTracker
│   └── (recordingRepo, egressClient — planned)
├── storage.ChatUploadStore + ChatUploadTracker
├── queue.Worker
│   ├── user_delete → RoomCleanupService
│   ├── room_delete → RoomCleanupService
│   ├── room_suspend → RoomCleanupService
│   ├── chat_upload_s3 → upload store
│   ├── send_email → SMTP templates
│   └── dispatch_webhook → HTTP POST
├── scheduler (gocron)
│   ├── idle room suspend/delete
│   └── old job row cleanup
└── handlers.*
    ├── auth, room, users, admin, preferences
    ├── livekit_webhook, cert, overview, queue stats
    └── (recording — planned, routes commented out)
```

Handlers receive dependencies via constructor injection in `server.go` — no global service locator except `config.Get()` and `database.GetDB()` used in middleware.

---

## Sync vs async operations

| Pattern | When | HTTP status | Example |
|---------|------|-------------|---------|
| **Sync** | Fast DB/LK ops | 200/201 | Join room, update settings |
| **Async (queue)** | Slow, failure-prone, bulk | 202 Accepted | User delete, room delete, bulk suspend |
| **Fire-and-forget webhook** | Advisory notifications | N/A (background) | `dispatch_webhook` job |

Handlers that enqueue jobs should return immediately with a job ID or acknowledgment — the client polls admin queue stats or trusts eventual consistency.

---

## LiveKit integration paths

| Path | Purpose |
|------|---------|
| **Embedded** | `livekit.StartInternalServer()` subprocess; `/livekit` reverse proxy for WS |
| **External** | Client connects to `livekit.host`; webhook URL must be configured manually |
| **Room API** | `lkutil.NewClient()` — create/delete rooms, list participants, mute, kick |
| **Token minting** | Handlers generate JWT with `VideoGrant` on join/refresh |
| **Data channel** | `lkutil.SendSystemMessage()` for moderation events visible in UI |
| **Webhook** | `POST /api/livekit/webhook` — participant_left, room_finished, egress events |

See [Room Lifecycle](./room-lifecycle.md) and [internal/livekit.md](./internal/livekit.md).

---

## Dual entrypoints caveat

Routes are registered in **two places**:

- `internal/server/server.go` — production (`bedrud run`)
- `cmd/server/main.go` — dev API (Air hot-reload)

When adding routes, update both until registration is consolidated. See [Entrypoints](./entrypoints.md).

---

## Related docs

| Topic | Document |
|-------|----------|
| HTTP routes | [routes.md](./routes.md) |
| Auth flows | [auth-flows.md](./auth-flows.md) |
| Database tables | [database-schema.md](./database-schema.md) |
| Queue internals | [queue-deep-dive.md](./queue-deep-dive.md) |
| Handler method index | [handlers-reference.md](./handlers-reference.md) |
| Settings merge | [settings-system.md](./settings-system.md) |
| Code patterns | [backend-patterns.md](./backend-patterns.md) |