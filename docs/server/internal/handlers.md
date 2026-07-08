# HTTP Handlers (`internal/handlers`)

Fiber HTTP controllers. Each file maps to a route group. Handlers call repositories and services — never raw DB queries.

---

## Handler Files

| File | Struct | Route prefix |
|------|--------|--------------|
| `auth_handler.go` | `AuthHandler` | `/api/auth/*` (local, passkey, verify) |
| `auth.go` | — (functions) | `/api/auth/:provider/*` (OAuth) |
| `room.go` | `RoomHandler` | `/api/room/*`, `/api/admin/rooms/*` |
| `room_auth.go` | — (helpers) | Room-level authorization checks |
| `users.go` | `UsersHandler` | `/api/admin/users/*` |
| `admin_handler.go` | `AdminHandler` | `/api/admin/settings`, `/api/admin/invite-tokens` |
| `admin_overview.go` | `AdminOverviewHandler` | `/api/admin/overview` |
| `admin_queue.go` | `AdminQueueHandler` | `/api/admin/queue` |
| `preferences_handler.go` | `PreferencesHandler` | `/api/auth/preferences` |
| `recording_handler.go` | `RecordingHandler` | `/api/room/:id/recording/*`, `/api/recordings/*` |
| `livekit_webhook.go` | — | `/api/livekit/webhook` |
| `cert_handler.go` | — | `/api/admin/cert/info` |
| `cooldown.go` | `CooldownCache` | Per-key in-memory cooldown (verification resend) |
| `errors.go` | — | Standardized error response helpers |
| `models.go` | — | Shared response DTOs |

---

## AuthHandler

```go
type AuthHandler struct {
    authService      *auth.AuthService
    config           *config.Config
    settingsRepo     *repository.SettingsRepository
    inviteTokenRepo  *repository.InviteTokenRepository
    challengeStore   *auth.ChallengeStore
    emailCooldown    *CooldownCache
    verifEventRepo   *repository.VerificationEventRepository
}
```

Handles: register, login, guest, refresh, logout, profile, password, passkey flows, email verification (`VerifyEmail`, `ResendVerification`, `CheckVerificationStatus`), password reset (`ForgotPassword`, `ResetPassword`), OAuth callback.

---

## RoomHandler

```go
type RoomHandler struct {
    client           livekit.RoomService
    lkCfg            *config.LiveKitConfig
    chatCfg          *config.ChatConfig
    roomRepo         *repository.RoomRepository
    userRepo         *repository.UserRepository
    recordingRepo    *repository.RecordingRepository
    settingsRepo     *repository.SettingsRepository
    webhookRepo      *repository.WebhookRepository
    uploadTracker    *storage.ChatUploadTracker
    cleanupSvc       *services.RoomCleanupService
    deletionInFlight sync.Map
}
```

Additional handlers: `RefreshLiveKitToken`, `ListArchivedRooms`, `ListRoomEvents`, `AdminReactivateRoom`, `AdminGenerateToken`.

### Room lifecycle

| Handler | Behavior |
|---------|----------|
| `CreateRoom` | Create in LK + DB. Auto-gen name. 409 on conflict. 24h expiry |
| `JoinRoom` | Lookup by name, add participant, gen LK token with metadata |
| `GuestJoinRoom` | Public rooms only. Restricted LK token |
| `ListRooms` | User's created rooms |
| `DeleteRoom` | 202 Accepted → `room_delete` job |
| `UpdateSettings` | Partial update (pointer fields). Preserves `isPersistent` |

### Moderation

Uses LiveKit RoomService client for real-time actions:

- Kick/ban: remove from LK + system message via `lkutil.SendSystemMessage`
- Mute/video/screenshare: mute specific track types
- Promote/demote: update LK participant metadata
- Deafen/undeafen: targeted data channel messages
- Spotlight: broadcast to room

### Admin room operations

| Handler | Behavior |
|---------|----------|
| `AdminListRooms` | Paginated all rooms |
| `AdminCloseRoom` | 202 → `room_delete` job |
| `AdminSuspendRoom` | 202 → `room_suspend` job |
| `AdminUpdateRoom` | Superadmin settings merge (includes `isPersistent`) |
| `AdminGetRoomParticipants` | Live LK participants + track stats |
| `BulkSuspendRooms` / `BulkCloseRooms` | Enqueue per-room jobs |

### Chat upload

`UploadChatImage` — multipart upload via `ChatUploadStore`. Tracks disk uploads in `ChatUploadTracker`. S3 uploads may be async via `chat_upload_s3` job.

---

## UsersHandler

```go
type UsersHandler struct {
    userRepo      *repository.UserRepository
    roomRepo      *repository.RoomRepository
    passkeyRepo   *repository.PasskeyRepository
    prefsRepo     *repository.UserPreferencesRepository
    uploadTracker *storage.ChatUploadTracker
    client        livekit.RoomService
}
```

Admin user management (superadmin only). `DeleteUser` returns 202 and enqueues `user_delete` job. Self-deletion guard returns 400.

---

## AdminHandler

```go
type AdminHandler struct {
    settingsRepo    *repository.SettingsRepository
    inviteTokenRepo *repository.InviteTokenRepository
    webhookRepo     *repository.WebhookRepository
    recordingRepo   *repository.RecordingRepository
}
```

- `GetSettings` / `UpdateSettings` — singleton `SystemSettings` (secrets masked as `******`)
- `GetPublicSettings` — unauthenticated: registration flags, OAuth providers, passkeys
- `SendTestEmail` — SMTP connectivity test from admin panel
- `ValidateSettingsConnectivity` — test LK, SMTP, S3 connections
- Invite token CRUD
- Webhook CRUD + `RotateWebhookSecret` + `TestWebhook`

## AdminOverviewHandler

Aggregates dashboard data in `GetOverview`:

- System health (DB, LiveKit reachability)
- KPIs: users, rooms, online participants (prefers LiveKit live count over stale DB)
- 7-day activity trend
- Room composition breakdown
- Attention items (failed jobs, expiring cert)
- Recent signups and room events
- Instance info (version, uptime)

## AdminQueueHandler

`GetQueueStats` — pending/active/done/failed counts, throughput, recent failures.

## CertHandler

- `GetCert` — public PEM certificate at `/api/cert`
- `GetCertInfo` — admin cert details (expiry, SANs, days remaining)

## CooldownCache (`cooldown.go`)

In-memory per-key TTL cache. `Allow(key)` returns false if still in cooldown. Used for verification email resend (key = email). Not shared across instances (Redis TODO).

---

## RecordingHandler

Manages room recordings via `RecordingService`:

- Start/stop egress via LiveKit
- List recordings per room
- Download recording files from `RecordingStore`
- Delete triggers `recording_delete` async job

Gated by `middleware.RecordingsEnabled()`.

---

## LiveKit Webhook

`livekit_webhook.go` handles POST from LiveKit:

- Participant disconnect detection
- Egress completion → enqueue `process_recording` job
- JWT validation using LiveKit API key/secret

---

## Shared DTOs (`models.go`)

```go
ErrorResponse  { Error string }
AuthResponse   { User UserResponse, Token string }
UserResponse   { ID, Email, Name, Provider, AvatarURL }
UserDetails    { ID, Email, Name, Provider, IsActive, IsAdmin, Accesses, CreatedAt }
```

---

## Authorization patterns

### JWT (global)

```go
claims := c.Locals("user").(*auth.Claims)
userID := claims.UserID
```

### Room-level (`room_auth.go`)

Checks: room creator, room admin permissions, or `superadmin`/`admin` in claims.Accesses.

### Admin routes

Chained middleware: `Protected()` → `RequireAccess(superadmin)`.

---

## Error handling (`errors.go`)

Standard pattern:

```go
return c.Status(fiber.StatusBadRequest).JSON(handlers.ErrorResponse{Error: "message"})
```

Status codes: 400 (validation), 401 (auth), 403 (authorization), 404 (not found), 409 (conflict), 413 (too large), 429 (rate limit), 500 (server), 501 (not implemented), 202 (async accepted).