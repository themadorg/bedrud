---
name: bedrud-http
description: HTTP layer — entrypoints, server bootstrap, route handlers, LiveKit adapter.
license: Apache License
---

# Bedrud HTTP Layer

Go module `bedrud`. `cmd/` + `internal/server/` + `internal/handlers/` + `internal/lkutil/`.

**Path convention:** All API routes live under Fiber group `/api` (Swagger `@BasePath /api`). Tables below use paths relative to `/api` unless noted. Static/upload paths are absolute.

**Critical plural split:** room lifecycle uses singular `/room/...`; recording uses plural `/rooms/:id/...`.

---

## Entrypoints

### `cmd/bedrud/main.go` — Production CLI

Thin wrapper: `cli.Execute(version)` (Cobra under `internal/cli/`). Legacy flags (`--run`, `--livekit`, `--version`) still work via `dispatchLegacy`.

| Command | Calls | Purpose |
|---------|-------|---------|
| `run` / `server` | `server.Run(configPath, version)` | Start full app (API + optional embedded LK + SPA) |
| `livekit` / `--livekit` | `livekit.RunLiveKit(configPath)` | Run LK binary standalone |
| `install` | `install.LinuxInstall(...)` | Systemd install (Debian/Linux) |
| `uninstall` | `install.DebianUninstall()` / Linux uninstall | Remove install |
| `user create/delete/promote/demote/list/info/password/...` | `usercli.*` | Local user management |
| `room list/info/close/suspend/reactivate/kick` | room CLI helpers | Room ops without HTTP |
| `config path/show/get/set/validate` | config CLI | Config inspection |
| `settings`, `invite-token`, `cert`, `db migrate/status` | respective CLIs | Ops tooling |
| `version` | print version | Version string |

Config path: `--config`, `BEDRUD_CONFIG`, or `CONFIG_PATH`.

### `cmd/server/main.go` — Dev API server

Air hot-reload target. No Cobra. Inits subsystems, registers routes, serves Swagger/Scalar + SPA.

- Swagger: `GET /api/swagger/*`
- Scalar: `GET /api/scalar`
- Health: `GET /api/health` · Ready: `GET /api/ready`
- Docs disable: `DISABLE_API_DOCS`

Minor route drift vs prod `server.Run` (e.g. `cmd/server` still registers `GET /admin/stats` → `GetAdminStats`; prod bootstrap does not). Prefer `internal/server/server.go` as production source of truth.

### `ui.go` — Frontend embed

`//go:embed all:frontend` → `UI embed.FS`. Populated by `make build` (`apps/web/build/*` → `server/frontend/`).

---

## `internal/server/server.go` — Bootstrap

`Run(configPath, version string) error` — production sequence:

1. Load config + validate JWT/session secrets
2. Start embedded LiveKit when internal (or log external host)
3. Init session store
4. Init DB + migrations
5. Init repos (`room`, `user`, `recording`, …)
6. Init scheduler (`recordingRepo` passed; recording store cleanup optional)
7. Auth providers (`auth.Init`) + load banned users
8. Fiber app: recover, helmet, CORS (credentials forbid `*`)
9. LK reverse-proxy at `/livekit` (internal only)
10. Build upload tracker, LK client, cleanup service
11. Start queue worker (`user_delete`, `room_delete`, `room_suspend`, `chat_upload_s3`, `send_email`, `dispatch_webhook`; recording job types wired when recording store enabled)
12. Init handlers (auth, room, users, admin, cert, overview, queue, livekit webhook, recording)
13. Register `/api` routes + static uploads
14. Embed SPA (`index.html` for `/`, `shell.html` elsewhere)
15. TLS: ACME / manual certs / plain HTTP; optional HTTP on `httpPort`
16. Graceful shutdown on SIGINT/SIGTERM

Health redirects: `/health` → `/api/health`, `/ready` → `/api/ready`. Ready pings DB (503 if down).

---

## `internal/handlers/` — HTTP Route Handlers

### `auth.go` — OAuth (goth)

`responseWriter`: Fiber `Ctx` → `http.ResponseWriter` adapter for gothic.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `BeginAuthHandler` | GET | `/auth/:provider/login` | Start OAuth |
| `CallbackHandler` | GET | `/auth/:provider/callback` | Complete OAuth → JWT cookies → redirect |

### `auth_handler.go` — Local auth + passkeys + profile

`AuthHandler`: `authService`, `config`, `settingsRepo`, `inviteTokenRepo`, `challengeStore`, `emailCooldown`, `verifEventRepo`.

`NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, challengeStore, emailCooldown, verifEventRepo)`.

| Fn | Method | Route | Auth / notes | Purpose |
|----|--------|-------|--------------|---------|
| `Register` | POST | `/auth/register` | Rate limit | Email/pass signup |
| `Login` | POST | `/auth/login` | Rate limit | Email/pass login |
| `GuestLogin` | POST | `/auth/guest-login` | Rate limit | Name-only guest |
| `RefreshToken` | POST | `/auth/refresh` | Rate limit | Rotate token pair |
| `Logout` | POST | `/auth/logout` | Protected | Block refresh, clear cookies |
| `GetMe` | GET | `/auth/me` | Protected + email verified | Current user |
| `UpdateProfile` | PUT | `/auth/me` | Protected + email verified | Display name / profile fields |
| `UploadAvatar` | POST | `/auth/me/avatar` | Protected + email verified | Multipart avatar |
| `DeleteAvatar` | DELETE | `/auth/me/avatar` | Protected + email verified | Remove avatar |
| `ChangePassword` | PUT | `/auth/password` | Protected + email verified | Old → new password |
| `VerifyEmail` | POST | `/auth/verify` | Public, body `{token}` | Verify email |
| `CheckVerificationStatus` | GET | `/auth/verify/status` | Protected | Verification status |
| `ResendVerification` | POST | `/auth/verify/resend` | Resend rate limit | Resend verification email |
| `ForgotPassword` | POST | `/auth/forgot-password` | Rate limit | Start reset |
| `ResetPassword` | POST | `/auth/reset-password` | Rate limit | Complete reset |
| `PasskeyRegisterBegin` | POST | `/auth/passkey/register/begin` | Protected + verified | WebAuthn reg start |
| `PasskeyRegisterFinish` | POST | `/auth/passkey/register/finish` | Protected + verified | WebAuthn reg complete |
| `PasskeyLoginBegin` | POST | `/auth/passkey/login/begin` | Rate limit | WebAuthn login start |
| `PasskeyLoginFinish` | POST | `/auth/passkey/login/finish` | Rate limit | WebAuthn login complete |
| `PasskeySignupBegin` | POST | `/auth/passkey/signup/begin` | Rate limit | Passkey signup start |
| `PasskeySignupFinish` | POST | `/auth/passkey/signup/finish` | Rate limit | Passkey signup complete |

Helpers: `setAuthCookies` / `clearAuthCookies`, `getSession`, `getRPID` / `getOrigin`.

### `room.go` — Room lifecycle + moderation

`RoomHandler`: `roomRepo`, `userRepo`, `recordingRepo`, `webhookRepo`, `livekitHost`, `apiKey`, `apiSecret`, LK `RoomService` client, `uploadStore`, `uploadMax`, `uploadTracker`, `cleanupSvc`, `settingsRepo`, `deletionInFlight`, `uploadBackend`, `inlineMaxBytes`.

`NewRoomHandler(client, lkCfg, chatCfg, roomRepo, userRepo, recordingRepo, settingsRepo, webhookRepo, uploadTracker, cleanupSvc)`.

**Room paths are singular `/room/...` (not `/rooms`).**

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `CreateRoom` | POST | `/room/create` | Create in LK + DB |
| `JoinRoom` | POST | `/room/join` | Add participant + LK token |
| `GuestJoinRoom` | POST | `/room/guest-join` | Unauth guest (public rooms) |
| `RefreshLiveKitToken` | POST | `/room/refresh-token` | Refresh LK JWT |
| `ListRooms` | GET | `/room/list` | User's rooms |
| `ListArchivedRooms` | GET | `/room/archived` | Archived rooms + recording counts |
| `KickParticipant` | POST | `/room/:roomId/kick/:identity` | Remove from LK + broadcast |
| `MuteParticipant` | POST | `/room/:roomId/mute/:identity` | Mute audio tracks |
| `BanParticipant` | POST | `/room/:roomId/ban/:identity` | Ban + remove |
| `DisableParticipantVideo` | POST | `/room/:roomId/video/:identity/off` | Mute camera |
| `PromoteParticipant` | POST | `/room/:roomId/promote/:identity` | Grant moderator |
| `DemoteParticipant` | POST | `/room/:roomId/demote/:identity` | Revoke moderator |
| `BlockChat` | POST | `/room/:roomId/chat/:identity/block` | Set chatBlocked |
| `DeafenParticipant` | POST | `/room/:roomId/deafen/:identity` | Targeted deafen msg |
| `UndeafenParticipant` | POST | `/room/:roomId/undeafen/:identity` | Targeted undeafen msg |
| `AskParticipantAction` | POST | `/room/:roomId/ask/:identity/:action` | `ask_unmute` / `ask_camera` |
| `SpotlightParticipant` | POST | `/room/:roomId/spotlight/:identity` | Broadcast spotlight |
| `StopScreenShare` | POST | `/room/:roomId/screenshare/:identity/stop` | Mute screen share |
| `GetRoomPresence` | GET | `/room/:roomId/presence` | Public-ish presence (API rate limit) |
| `GetParticipantInfo` | GET | `/room/:roomId/participant/:identity/info` | LK participant + tracks |
| `GetParticipantProfile` | GET | `/room/:roomId/participant/:identity/profile` | Profile for participant |
| `BringToStage` | POST | `/room/:roomId/stage/:identity/bring` | **501 stub** |
| `RemoveFromStage` | POST | `/room/:roomId/stage/:identity/remove` | **501 stub** |
| `UpdateSettings` | PUT | `/room/:roomId/settings` | Partial room settings |
| `DeleteRoom` | DELETE | `/room/:roomId` | 202 · enqueue `room_delete` |
| `UploadChatImage` | POST | `/room/:roomId/chat/upload` | Multipart chat image |

**Admin room ops** (under `/admin`, superadmin):

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `AdminListRooms` | GET | `/admin/rooms` | All rooms |
| `ListRoomEvents` | GET | `/admin/rooms/events` | Paginated events |
| `AdminGenerateToken` | POST | `/admin/rooms/:roomId/token` | **501 stub** |
| `AdminCloseRoom` | DELETE | `/admin/rooms/:roomId` | 202 · `room_delete` |
| `AdminSuspendRoom` | POST | `/admin/rooms/:roomId/suspend` | 202 · `room_suspend` |
| `AdminReactivateRoom` | POST | `/admin/rooms/:roomId/reactivate` | Reactivate suspended room |
| `AdminUpdateRoom` | PUT | `/admin/rooms/:roomId` | maxParticipants + settings |
| `AdminGetRoomParticipants` | GET | `/admin/rooms/:roomId/participants` | Live participants |
| `AdminKickParticipant` | POST | `/admin/rooms/:roomId/participants/:identity/kick` | Kick (no creator check) |
| `AdminMuteParticipant` | POST | `/admin/rooms/:roomId/participants/:identity/mute` | Mute audio |
| `BulkSuspendRooms` | POST | `/admin/rooms/bulk/suspend` | Bulk suspend |
| `BulkCloseRooms` | POST | `/admin/rooms/bulk/close` | Bulk close |
| `GetOnlineCount` | GET | `/admin/online-count` | Active participant count |
| `AdminLiveKitStats` | GET | `/admin/livekit/stats` | Aggregate LK stats |
| `GetAdminStats` | GET | `/admin/stats` | KPI stats (wired in `cmd/server`; not in prod `server.go`) |

### `recording_handler.go` — Recording (SHIPPED)

HTTP-only: auth, room lookup, moderator / view ACL. Business logic → `services.RecordingService`.

`RecordingHandler`: `roomRepo`, `recordingService`, `recordingRepo`, `recordingStore`.

`NewRecordingHandler(roomRepo, recordingService, recordingRepo, recStore)`.

DTO: `RecordingDTO` (`id`, `recordingType`, `durationMs`, `fileSize`, `fileUrl`, `status`, `error`, `downloadStatus`, `roomId`, `roomName`, `createdBy`, `createdAt`).

**Recording paths use plural `/rooms/:id/...`.**

| Fn | Method | Route | Auth stack | Purpose |
|----|--------|-------|------------|---------|
| `StartRecording` | POST | `/rooms/:id/recording/start` | Protected + verified + **RecordingsEnabled** (+ API rate limit) | Start composite egress · 201 |
| `StopRecording` | POST | `/rooms/:id/recording/stop` | Protected + verified + **RecordingsEnabled** | Stop active recording |
| `ListRecordings` | GET | `/rooms/:id/recordings` | Protected + verified + **RecordingsEnabled** | Paginated list (participants / owner / superadmin) |
| `GetRecording` | GET | `/rooms/:id/recordings/:rid` | Protected + verified + **RecordingsEnabled** | Single recording |
| `WaitRecordingReady` | GET | `/rooms/:id/recordings/:rid/wait` | Protected + verified + **RecordingsEnabled** | Long-poll ≤15s until egress started/failed |
| `ClearRoomRecordings` | DELETE | `/rooms/:id/recordings` | Protected + verified | Clear all for room (creator/superadmin) |
| `ClearSingleRecording` | DELETE | `/rooms/:id/recordings/:recordingId` | Protected + verified | Clear one recording |
| `AdminListRecordings` | GET | `/admin/recordings` | Superadmin group | Global list + filters |
| `BulkDeleteRecordings` | POST | `/admin/recordings/bulk/delete` | Superadmin group | 202 · enqueue `recording_delete` (max 500) |

Helpers: `canViewRoomRecordings`, `validateUUID`, `getRoomAdminID`, `recordingToDTO` / `computeDownloadStatus`.

**Bootstrap wiring:** handlers + middleware + swagger are complete. Production `server.Run` / `cmd/server` still have recording store, egress client, handler construct, and route lines present as commented templates next to the live room routes — wire them the same way as integration tests (`handlers_integration_test.go`).

### `middleware/recordings_enabled.go` — `RecordingsEnabled`

`RecordingsEnabled(settingsRepo) fiber.Handler`:

1. Load system settings
2. If `!settings.RecordingsEnabled` → **403** `"Recordings are disabled on this server"`
3. Else `c.Next()`

First gate; service layer re-checks (room `RecordingsAllowed`, limits). Apply on room-scoped recording start/stop/list/get/wait. Admin recording routes rely on superadmin group (and service rules).

### `users.go` — Admin user management

`UsersHandler`: `userRepo`, `roomRepo`, `passkeyRepo`, `prefsRepo`, `cleanupSvc`, `verifEventRepo`, `deletionInFlight`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `ListUsers` | GET | `/admin/users` | All users + IsAdmin |
| `ListRecentSignups` | GET | `/admin/users/recent` | Recent signups + filters |
| `BulkBanUsers` | POST | `/admin/users/bulk/ban` | Bulk deactivate |
| `BulkPromoteUsers` | POST | `/admin/users/bulk/promote` | Bulk add admin |
| `BulkDeleteUsers` | POST | `/admin/users/bulk/delete` | 202 · per-user `user_delete` |
| `UpdateUserStatus` | PUT | `/admin/users/:id/status` | Set IsActive |
| `UpdateUserAccesses` | PUT | `/admin/users/:id/accesses` | Replace Accesses |
| `ForceLogout` | POST | `/admin/users/:id/force-logout` | Revoke sessions |
| `SetUserPassword` | PUT | `/admin/users/:id/password` | Admin-set password |
| `GetUserDetail` | GET | `/admin/users/:id` | Detail + rooms |
| `ListUserSessions` | GET | `/admin/users/:id/sessions` | Session list |
| `AdminVerifyEmail` | POST | `/admin/users/:id/verify` | Force-verify |
| `AdminResendVerification` | POST | `/admin/users/:id/verify/resend` | Resend on behalf |
| `DeleteUser` | DELETE | `/admin/users/:id` | 202 · async `user_delete` |

### `admin_handler.go` — Settings, invites, webhooks

`AdminHandler`: `settingsRepo`, `inviteTokenRepo`, `webhookRepo`, `recordingRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetPublicSettings` | GET | `/auth/settings` | Unauth reg/flags (incl. recordingsEnabled) |
| `GetSettings` | GET | `/admin/settings` | Full settings (secrets masked) |
| `UpdateSettings` | PUT | `/admin/settings` | Update settings |
| `SendTestEmail` | POST | `/admin/settings/send-test-email` | SMTP test |
| `ValidateSettingsConnectivity` | POST | `/admin/settings/validate` | Runtime connectivity checks |
| `ListInviteTokens` | GET | `/admin/invite-tokens` | Tokens + `used` |
| `CreateInviteToken` | POST | `/admin/invite-tokens` | Create token |
| `DeleteInviteToken` | DELETE | `/admin/invite-tokens/:id` | Delete token |
| `ListWebhooks` | GET | `/admin/webhooks` | Outbound webhooks |
| `CreateWebhook` | POST | `/admin/webhooks` | Create |
| `UpdateWebhook` | PUT | `/admin/webhooks/:id` | Update |
| `DeleteWebhook` | DELETE | `/admin/webhooks/:id` | Delete |
| `RotateWebhookSecret` | POST | `/admin/webhooks/:id/rotate-secret` | Rotate secret |
| `TestWebhook` | POST | `/admin/webhooks/:id/test` | Fire test delivery |

### `preferences_handler.go`

`PreferencesHandler{prefsRepo}`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetPreferences` | GET | `/auth/preferences` | User JSON prefs (`"{}"` default) |
| `UpdatePreferences` | PUT | `/auth/preferences` | Validate JSON ≤4KB, upsert |

### `admin_overview.go`

`AdminOverviewHandler`: `roomRepo`, `userRepo`, `settingsRepo`, `lkCfg`, LK client, `db`, `startTime`, `version`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetOverview` | GET | `/admin/overview` | Aggregated system stats / health / KPIs |

### `admin_queue.go`

`AdminQueueHandler{db}`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetQueueStats` | GET | `/admin/queue` | Pending/active/done/failed + email diagnostics |

### `cert_handler.go`

`CertHandler{cfg}`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetCert` | GET | `/cert` | Download TLS cert PEM (full path `/api/cert`) |
| `GetCertInfo` | GET | `/admin/cert-info` | Cert metadata |

### `livekit_webhook.go`

`LiveKitWebhookHandler`: `lkCfg`, `roomRepo`, `recordingRepo`, `webhookRepo`, `db`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `Handle` | POST | `/livekit/webhook` | LiveKit JWT + checksum; no app JWT |

Events: `participant_left`, `room_finished`, `egress_started`, `egress_ended` (recording status / enqueue process).

### Supporting files

| File | Role |
|------|------|
| `cooldown.go` | `CooldownCache` — verification resend TTL gate |
| `errors.go` | `internalError(err)` — log real, return generic 500 JSON |
| `room_auth.go` | `isRoomModerator(claims, roomOwnerID, roomID, roomRepo)` |
| `models.go` | `ErrorResponse`, `AuthResponse`, `UserResponse`, `BulkIDsRequest`, `BulkItemResult`, `BulkResult`, password length constants |

### Non-API static routes (absolute)

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/uploads/chat/*` | Protected + verified | Disk chat images |
| GET | `/uploads/avatars/*` | Public | Avatar files |

---

## Route map cheat sheet (prod `server.Run`)

```
/api/health  /api/ready
/api/auth/*  (register, login, guest-login, refresh, logout, me, password, verify*, forgot/reset, passkey/*, preferences, settings, :provider/*)
/api/room/*  (singular — create, join, guest-join, list, archived, :roomId/*)
/api/rooms/:id/recording/*  + /api/rooms/:id/recordings*   (plural — recording SHIPPED)
/api/admin/* (users, rooms, settings, invite-tokens, webhooks, overview, queue, cert-info, online-count, livekit/stats, recordings*)
/api/livekit/webhook
/api/cert
/uploads/chat/*  /uploads/avatars/*
/livekit/*  (reverse proxy when internal LK)
```

Admin group middleware: `Protected` → `RequireEmailVerified` → `RequireAccess(superadmin)`.

---

## `internal/lkutil/lkutil.go` — Shared LiveKit helpers

Used by handlers, services, user CLI.

| Export | Signature | Purpose |
|--------|-----------|---------|
| `NewClient(lkCfg)` | `func(*config.LiveKitConfig) livekit.RoomService` | RoomService protobuf client (`InternalHost`/`Host`, `SkipTLSVerify`) |
| `NewEgressClient(lkCfg)` | `func(*config.LiveKitConfig) (livekit.Egress, error)` | Egress client for recordings |
| `AuthContext(ctx, apiKey, apiSecret, grants...)` | `func(...) (context.Context, error)` | Inject Bearer JWT into twirp context |
| `SendSystemMessage(...)` | system data on topic `"system"`, reliable | Broadcast system event |
| `SendSystemMessageWithDeletedIdentity(...)` | same + `deletedIdentity` | Room/user delete messaging |

`SystemMessage` JSON: `type`, `event`, `message`, optional `deletedIdentity`.

---

## Stubs (HTTP 501 only)

| Fn | Route | Status |
|----|-------|--------|
| `BringToStage` | `POST /room/:roomId/stage/:identity/bring` | 501 |
| `RemoveFromStage` | `POST /room/:roomId/stage/:identity/remove` | 501 |
| `AdminGenerateToken` | `POST /admin/rooms/:roomId/token` | 501 |

Do **not** treat recording endpoints as stubs — they are implemented.
