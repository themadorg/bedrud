---
name: bedrud-http
description: HTTP layer — entrypoints, server bootstrap, route handlers, LiveKit adapter.
license: Apache License
---

# Bedrud HTTP Layer

Go module `bedrud`. `cmd/` + `internal/server/` + `internal/handlers/` + `internal/lkutil/`.

---

## Entrypoints

### `cmd/bedrud/main.go` — Production CLI

| Arg | Calls | Purpose |
|-----|-------|---------|
| `run` / `server` / `--run` | `server.Run(configPath)` | Start full app |
| `--livekit` | `livekit.RunLiveKit(configPath)` | Run LK binary standalone |
| `install` | `install.DebianInstall(...)` | Systemd install on Debian |
| `uninstall` | `install.DebianUninstall()` | Remove install |
| `user promote --email <email>` | `usercli.PromoteUser()` | Add superadmin access |
| `user demote --email <email>` | `usercli.DemoteUser()` | Remove superadmin access |
| `user create --email <e> --password <p> --name <n>` | `usercli.CreateUser()` | Create local user |
| `user delete --email <email>` | `usercli.DeleteUser()` | Delete user |

### `cmd/server/main.go` — Dev API server

Air hot-reload target. No CLI subcommands. Inits all subsystems, registers routes, serves Swagger/Scalar + SPA.
Health: `GET /api/health`. Ready: `GET /api/ready`.

### `ui.go` — Frontend embed

`//go:embed all:frontend` → `UI embed.FS`. Populated by `make build` copying `apps/web/build/*` → `server/frontend/`.

---

## `internal/server/server.go` — Bootstrap

`Run(configPath) error` — production bootstrap sequence:

1. Load config
2. Init LiveKit (internal or external)
3. Init session store
4. Init DB (`database.Initialize`)
5. Run migrations
6. Init scheduler (`scheduler.Initialize(db)`)
7. Init auth providers
8. Init all repos
9. Init cleanup service
10. Init queue worker
11. Init authService, challengeStore, authHandler
12. Init roomHandler
13. Register Fiber routes
14. Setup TLS (self-signed / ACME / manual)
15. Serve embedded SPA frontend
16. Graceful shutdown on SIGINT/SIGTERM

LK reverse-proxy at `/livekit`. CORS: dynamic origin reflection. SPA fallback: `index.html` for `/`, `shell.html` for non-API routes.

---

## `internal/handlers/` — HTTP Route Handlers

### `auth.go` — OAuth flows (goth)

`responseWriter` struct: adapter bridging Fiber `Ctx` → `http.ResponseWriter` for goth.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `BeginAuthHandler` | GET | `/auth/{provider}/login` | Start OAuth → redirect to provider |
| `CallbackHandler` | GET | `/auth/{provider}/callback` | Complete OAuth → upsert user → set JWT cookie → redirect to `/auth/callback?token=...` |

### `auth_handler.go` — Local auth + passkeys

`AuthHandler` struct: `authService`, `config`, `settingsRepo`, `inviteTokenRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `Register` | POST | `/auth/register` | Email/pass signup |
| `Login` | POST | `/auth/login` | Email/pass login |
| `GuestLogin` | POST | `/auth/guest` | Name-only guest |
| `RefreshToken` | POST | `/auth/refresh` | Rotate token pair |
| `GetMe` | GET | `/auth/me` | Return current user |
| `UpdateProfile` | PUT | `/auth/profile` | Update display name |
| `ChangePassword` | PUT | `/auth/password` | Validate old, set new |
| `Logout` | POST | `/auth/logout` | Block refresh token, clear cookies |
| `PasskeyRegisterBegin` | POST | `/auth/passkey/register/begin` | WebAuthn reg start |
| `PasskeyRegisterFinish` | POST | `/auth/passkey/register/finish` | WebAuthn reg complete |
| `PasskeyLoginBegin` | POST | `/auth/passkey/login/begin` | WebAuthn login start |
| `PasskeyLoginFinish` | POST | `/auth/passkey/login/finish` | WebAuthn login complete |
| `PasskeySignupBegin` | POST | `/auth/passkey/signup/begin` | Full passkey signup start |
| `PasskeySignupFinish` | POST | `/auth/passkey/signup/finish` | Full passkey signup complete |
| `VerifyEmail` | GET | `/auth/verify` | Verify email via token from link |
| `CheckVerificationStatus` | GET | `/auth/verify/status` | Check email verified |
| `ResendVerification` | POST | `/auth/verify/resend` | Resend verification email |

Helpers: `setAuthCookies` (HttpOnly, secure/sameSite/domain from config), `clearAuthCookies`, `getSession`, `getRPID`/`getOrigin`.
Constructor: `NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, challengeStore, emailCooldown, verifEventRepo)` — 7 deps.

### `room.go` — Room lifecycle + participant moderation

`RoomHandler` struct: `roomRepo`, `userRepo`, `livekitHost`, `apiKey`, `apiSecret`, `livekit.RoomService` client, `uploadTracker`, `cleanupSvc`, `settingsRepo`, `uploadStore`, `uploadMax`, `uploadBackend`, `inlineMaxBytes`, `deletionInFlight sync.Map`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `CreateRoom` | POST | `/rooms` | Create in LK + DB |
| `JoinRoom` | POST | `/rooms/join` | Lookup room, add participant, gen LK token |
| `GuestJoinRoom` | POST | `/rooms/guest-join` | Unauth guest for public rooms |
| `ListRooms` | GET | `/rooms` | User's created rooms |
| `DeleteRoom` | DELETE | `/rooms/:roomId` | 202. Enqueues `room_delete` job |
| `UpdateSettings` | PATCH | `/rooms/:roomId/settings` | Partial update |
| `PromoteParticipant` | POST | `/rooms/:roomId/participants/:identity/promote` | Add moderator |
| `DemoteParticipant` | POST | `/rooms/:roomId/participants/:identity/demote` | Remove moderator |
| `KickParticipant` | DELETE | `/rooms/:roomId/participants/:identity` | Remove from LK + broadcast |
| `BanParticipant` | DELETE | `/rooms/:roomId/participants/:identity/ban` | Remove from LK + DB banned |
| `MuteParticipant` | POST | `/rooms/:roomId/participants/:identity/mute` | Mute all audio tracks |
| `DisableParticipantVideo` | POST | `/rooms/:roomId/participants/:identity/disable-video` | Mute camera track |
| `StopScreenShare` | POST | `/rooms/:roomId/participants/:identity/stop-screen-share` | Mute screen-share |
| `BlockChat` | POST | `/rooms/:roomId/participants/:identity/block-chat` | Set chatBlocked |
| `DeafenParticipant` | POST | `/rooms/:roomId/participants/:identity/deafen` | Send "deafen" data msg |
| `UndeafenParticipant` | POST | `/rooms/:roomId/participants/:identity/undeafen` | Send "undeafen" data msg |
| `AskParticipantAction` | POST | `/rooms/:roomId/participants/:identity/ask/:action` | ask_unmute / ask_camera |
| `SpotlightParticipant` | POST | `/rooms/:roomId/participants/:identity/spotlight` | Broadcast spotlight |
| `GetParticipantInfo` | GET | `/rooms/:roomId/participants/:identity` | Identity, name, state, tracks |
| `UploadChatImage` | POST | `/rooms/:roomId/chat/upload` | Multipart upload |
| `AdminListRooms` | GET | `/admin/rooms` | All rooms |
| `AdminCloseRoom` | DELETE | `/admin/rooms/:roomId` | 202. Enqueues `room_delete` |
| `AdminSuspendRoom` | POST | `/admin/rooms/:roomId/suspend` | 202. Enqueues `room_suspend` |
| `AdminUpdateRoom` | PATCH | `/admin/rooms/:roomId` | Update maxParticipants + settings |
| `AdminGetRoomParticipants` | GET | `/admin/rooms/:roomId/participants` | Live participants |
| `AdminKickParticipant` | DELETE | `/admin/rooms/:roomId/participants/:identity` | Kick (no creator check) |
| `AdminMuteParticipant` | POST | `/admin/rooms/:roomId/participants/:identity/mute` | Mute audio |
| `AdminLiveKitStats` | GET | `/admin/livekit/stats` | Aggregate |
| `BulkSuspendRooms` | POST | `/admin/rooms/bulk/suspend` | Enqueue per-room suspend |
| `BulkCloseRooms` | POST | `/admin/rooms/bulk/close` | Enqueue per-room delete |
| `GetAdminStats` | GET | `/admin/stats` | Aggregate KPIs |
| `ListRoomEvents` | GET | `/admin/rooms/events` | Paginated room events |
| `AdminGenerateToken` | POST | `/admin/rooms/:roomId/token` | 501 stub |
| `GetOnlineCount` | GET | `/admin/online-count` | Active participant count |
| `BringToStage` | POST | `/rooms/:roomId/participants/:identity/bring-to-stage` | Stub |
| `RemoveFromStage` | POST | `/rooms/:roomId/participants/:identity/remove-from-stage` | Stub |

### `users.go` — Admin user management

`UsersHandler` struct: `userRepo`, `roomRepo`, `passkeyRepo`, `prefsRepo`, `cleanupSvc`, `verifEventRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `ListUsers` | GET | `/admin/users` | All users with computed IsAdmin |
| `ListRecentSignups` | GET | `/admin/users/recent` | Recent signups with filters |
| `UpdateUserAccesses` | PUT | `/admin/users/:id/accesses` | Replace entire Accesses |
| `UpdateUserStatus` | PUT | `/admin/users/:id/status` | Set IsActive |
| `SetUserPassword` | PUT | `/admin/users/:id/password` | Admin-set password |
| `ForceLogout` | POST | `/admin/users/:id/force-logout` | Revoke all sessions |
| `AdminVerifyEmail` | POST | `/admin/users/:id/verify` | Force-verify email |
| `AdminResendVerification` | POST | `/admin/users/:id/verify/resend` | Resend on behalf |
| `GetUserDetail` | GET | `/admin/users/:id` | User details + rooms |
| `DeleteUser` | DELETE | `/admin/users/:id` | 202 async 3-phase |
| `BulkBanUsers` | POST | `/admin/users/bulk/ban` | Bulk deactivate |
| `BulkPromoteUsers` | POST | `/admin/users/bulk/promote` | Bulk add admin |
| `BulkDeleteUsers` | POST | `/admin/users/bulk/delete` | 202 enqueue per-user |

### `admin_handler.go` — System settings + invite tokens

`AdminHandler` struct: `settingsRepo`, `inviteTokenRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetSettings` | GET | `/admin/settings` | Full system settings |
| `GetPublicSettings` | GET | `/settings` | Unauth. reg flags only |
| `UpdateSettings` | PUT | `/admin/settings` | Replace entire settings |
| `ListInviteTokens` | GET | `/admin/invite-tokens` | All tokens with `used` bool |
| `CreateInviteToken` | POST | `/admin/invite-tokens` | Crypto-random hex, email-bind |
| `DeleteInviteToken` | DELETE | `/admin/invite-tokens/:id` | Delete by ID |
| `ValidateSettingsConnectivity` | POST | `/admin/settings/validate` | Runtime checks |

### `preferences_handler.go`

`PreferencesHandler` struct: `prefsRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetPreferences` | GET | `/api/auth/preferences` | User's preferencesJson blob |
| `UpdatePreferences` | PUT | `/api/auth/preferences` | Validate JSON + ≤4KB, upsert |

### `admin_overview.go`

`AdminOverviewHandler` struct: `roomRepo`, `userRepo`, `settingsRepo`, `lkCfg`, `livekit.RoomService` client, `db`, `startTime`, `version`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetOverview` | GET | `/api/admin/overview` | Aggregated system stats |

### `cert_handler.go`

`CertHandler` struct: `cfg *config.Config`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetCert` | GET | `/api/cert` | Download server TLS cert PEM |
| `GetCertInfo` | GET | `/api/admin/cert-info` | Certificate metadata |

> **TODO oncoming feature:** Recording functionality is planned for a future release.

### `recording_handler.go`

`RecordingHandler` struct: `roomRepo`, `recordingService`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `StartRecording` | POST | `/api/rooms/:id/recording/start` | Auth + moderator check → delegate to RecordingService |
| `StopRecording` | POST | `/api/rooms/:id/recording/stop` | Auth + moderator check → delegate to RecordingService |
| `ListRecordings` | GET | `/api/rooms/:id/recordings` | Paginated recording list via service |
| `GetRecording` | GET | `/api/rooms/:id/recordings/:rid` | Single recording via service |

> **TODO oncoming feature:** Recording functionality is planned for a future release.

### `recordings_enabled.go` (middleware)

`RecordingsEnabled(settingsRepo)` → middleware that checks `SystemSettings.RecordingsEnabled`. Returns 403 if disabled. Applied to all 4 recording routes.

### `livekit_webhook.go`

`LiveKitWebhookHandler` struct: `lkCfg`, `roomRepo`, `db`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `Handle` | POST | `/api/livekit/webhook` | Validate LK JWT + SHA256 checksum. Handles `participant_disconnected`, `room_finished`, `egress_ended` |

### `cooldown.go`

`CooldownCache` — in-memory, TTL-based, thread-safe. Used for verification email resend gating.
`NewCooldownCache(ttl)`, `Allow(key)`, `Remaining(key)`.

### `errors.go` — Shared error helpers

`internalError(err)` — logs real error, returns generic `{"error":"An internal error occurred"}`.

### `room_auth.go`

`isRoomModerator(claims, roomOwnerID, roomID, roomRepo)` — true if superadmin, room creator/admin, or room moderator.

### `models.go` — Shared response DTOs

`ErrorResponse{Error string}`, `AuthResponse{User UserResponse, Token string}`, `UserResponse{ID, Email, Name, Provider, AvatarURL}`.

---

## `internal/lkutil/lkutil.go` — Shared LiveKit Helpers

Cross-cutting package used by handlers, services, and user CLI.

| Export | Signature | Purpose |
|--------|-----------|---------|
| `NewClient(lkCfg)` | `func(*config.LiveKitConfig) livekit.RoomService` | Create LiveKit RoomService protobuf client. Respects `InternalHost`/`Host`, handles `SkipTLSVerify` |
| `AuthContext(ctx, apiKey, apiSecret, grants...)` | `func(context.Context, string, string, ...*lkauth.VideoGrant) context.Context` | Inject Bearer token into twirp context |
| `SendSystemMessage(ctx, client, roomName, event, message)` | `func(context.Context, livekit.RoomService, string, string, string)` | Send typed system data message over LiveKit data channel (topic `"system"`, kind `RELIABLE`) |
