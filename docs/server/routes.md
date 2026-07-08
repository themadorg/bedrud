# Complete Route Table

Authoritative list of HTTP routes registered in `internal/server/server.go`. All API routes are under `/api`.

**Legend:** Auth = middleware chain. `EmailVerified` = `RequireEmailVerified` when `requireEmailVerification` is enabled.

---

## Health

| Method | Path | Auth | Handler |
|--------|------|------|---------|
| GET | `/api/health` | none | Inline — `{"status":"healthy"}` |
| GET | `/api/ready` | none | Inline — DB ping, 503 if unavailable |
| GET | `/health` | none | 307 → `/api/health` |
| GET | `/ready` | none | 307 → `/api/ready` |

---

## Auth — Local

| Method | Path | Auth | Rate limit |
|--------|------|------|------------|
| POST | `/api/auth/register` | none | Auth |
| POST | `/api/auth/login` | none | Auth |
| POST | `/api/auth/guest-login` | none | Auth |
| POST | `/api/auth/refresh` | none | Auth |
| POST | `/api/auth/logout` | Protected | — |
| GET | `/api/auth/me` | Protected + EmailVerified | — |
| PUT | `/api/auth/me` | Protected + EmailVerified | — |
| PUT | `/api/auth/password` | Protected + EmailVerified | — |
| POST | `/api/auth/verify` | none | — |
| GET | `/api/auth/verify/status` | Protected | — |
| POST | `/api/auth/verify/resend` | none | Resend |
| POST | `/api/auth/forgot-password` | none | Auth |
| POST | `/api/auth/reset-password` | none | Auth |
| GET | `/api/auth/settings` | none | `adminHandler.GetPublicSettings` |

---

## Auth — OAuth

| Method | Path | Rate limit |
|--------|------|------------|
| GET | `/api/auth/:provider/login` | Auth |
| GET | `/api/auth/:provider/callback` | Auth |

Providers: `google`, `github`, `twitter`.

---

## Auth — Passkeys

| Method | Path | Auth | Rate limit |
|--------|------|------|------------|
| POST | `/api/auth/passkey/register/begin` | Protected + EmailVerified | — |
| POST | `/api/auth/passkey/register/finish` | Protected + EmailVerified | — |
| POST | `/api/auth/passkey/login/begin` | none | Auth |
| POST | `/api/auth/passkey/login/finish` | none | Auth |
| POST | `/api/auth/passkey/signup/begin` | none | Auth |
| POST | `/api/auth/passkey/signup/finish` | none | Auth |

---

## Preferences

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/auth/preferences` | Protected + EmailVerified |
| PUT | `/api/auth/preferences` | Protected + EmailVerified |

---

## Rooms

| Method | Path | Auth | Rate limit |
|--------|------|------|------------|
| POST | `/api/room/create` | Protected + EmailVerified | API |
| POST | `/api/room/join` | Protected + EmailVerified | — |
| POST | `/api/room/guest-join` | none | Guest |
| POST | `/api/room/refresh-token` | Protected + EmailVerified | API |
| GET | `/api/room/list` | Protected + EmailVerified | — |
| GET | `/api/room/archived` | Protected + EmailVerified | — |
| PUT | `/api/room/:roomId/settings` | Protected + EmailVerified | — |
| DELETE | `/api/room/:roomId` | Protected + EmailVerified | — |
| POST | `/api/room/:roomId/chat/upload` | Protected + EmailVerified | API |
| GET | `/api/room/online-count` | — | (not registered at room prefix; see admin) |

### Room moderation

All: Protected + EmailVerified.

| Method | Path |
|--------|------|
| POST | `/api/room/:roomId/kick/:identity` |
| POST | `/api/room/:roomId/ban/:identity` |
| POST | `/api/room/:roomId/mute/:identity` |
| POST | `/api/room/:roomId/video/:identity/off` |
| POST | `/api/room/:roomId/screenshare/:identity/stop` |
| POST | `/api/room/:roomId/promote/:identity` |
| POST | `/api/room/:roomId/demote/:identity` |
| POST | `/api/room/:roomId/chat/:identity/block` |
| POST | `/api/room/:roomId/deafen/:identity` |
| POST | `/api/room/:roomId/undeafen/:identity` |
| POST | `/api/room/:roomId/ask/:identity/:action` |
| POST | `/api/room/:roomId/spotlight/:identity` |
| GET | `/api/room/:roomId/participant/:identity/info` |
| POST | `/api/room/:roomId/stage/:identity/bring` |
| POST | `/api/room/:roomId/stage/:identity/remove` |

---

## Chat upload static files

| Method | Path | Auth |
|--------|------|------|
| GET | `/uploads/chat/*` | Protected + EmailVerified |

Serves disk-backed chat images. Path traversal prevented. Inline (base64) and S3 images are not served here.

---

## Admin

All routes: `Protected` + `EmailVerified` + `RequireAccess(superadmin)`.

Prefix: `/api/admin`.

### Users

| Method | Path | Handler |
|--------|------|---------|
| GET | `/users` | `ListUsers` |
| GET | `/users/recent` | `ListRecentSignups` |
| GET | `/users/:id` | `GetUserDetail` |
| PUT | `/users/:id/status` | `UpdateUserStatus` |
| PUT | `/users/:id/accesses` | `UpdateUserAccesses` |
| POST | `/users/:id/force-logout` | `ForceLogout` |
| POST | `/users/:id/verify` | `AdminVerifyEmail` |
| POST | `/users/:id/verify/resend` | `AdminResendVerification` |
| GET | `/users/:id/sessions` | `ListUserSessions` |
| DELETE | `/users/:id` | `DeleteUser` (202 async) |

### Rooms

| Method | Path | Handler |
|--------|------|---------|
| GET | `/rooms` | `AdminListRooms` |
| GET | `/rooms/events` | `ListRoomEvents` |
| DELETE | `/rooms/:roomId` | `AdminCloseRoom` (202) |
| POST | `/rooms/:roomId/suspend` | `AdminSuspendRoom` (202) |
| POST | `/rooms/:roomId/reactivate` | `AdminReactivateRoom` |
| PUT | `/rooms/:roomId` | `AdminUpdateRoom` |
| POST | `/rooms/:roomId/token` | `AdminGenerateToken` |
| GET | `/rooms/:roomId/participants` | `AdminGetRoomParticipants` |
| POST | `/rooms/:roomId/participants/:identity/kick` | `AdminKickParticipant` |
| POST | `/rooms/:roomId/participants/:identity/mute` | `AdminMuteParticipant` |
| POST | `/rooms/bulk/suspend` | `BulkSuspendRooms` (202) |
| POST | `/rooms/bulk/close` | `BulkCloseRooms` (202) |
| GET | `/online-count` | `GetOnlineCount` |
| GET | `/livekit/stats` | `AdminLiveKitStats` |

### Overview & queue

| Method | Path | Handler |
|--------|------|---------|
| GET | `/overview` | `GetOverview` |
| GET | `/queue` | `GetQueueStats` |

### Settings

| Method | Path | Handler |
|--------|------|---------|
| GET | `/settings` | `GetSettings` |
| PUT | `/settings` | `UpdateSettings` |
| POST | `/settings/send-test-email` | `SendTestEmail` |
| POST | `/settings/validate` | `ValidateSettingsConnectivity` |

### Invite tokens

| Method | Path |
|--------|------|
| GET | `/invite-tokens` |
| POST | `/invite-tokens` |
| DELETE | `/invite-tokens/:id` |

### Webhooks

| Method | Path |
|--------|------|
| GET | `/webhooks` |
| POST | `/webhooks` |
| PUT | `/webhooks/:id` |
| DELETE | `/webhooks/:id` |
| POST | `/webhooks/:id/rotate-secret` |
| POST | `/webhooks/:id/test` |

### Certificates

| Method | Path |
|--------|------|
| GET | `/cert-info` | `GetCertInfo` |

---

## Public certificate

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/cert` | none | `GetCert` (PEM) |

---

## LiveKit webhook

| Method | Path | Auth |
|--------|------|------|
| POST | `/api/livekit/webhook` | LiveKit JWT signature (no app auth) |

---

## LiveKit reverse proxy

| Method | Path | Condition |
|--------|------|-----------|
| `*` | `/livekit/*` | Embedded LiveKit only (`livekit.external: false`) |

Proxied to `http://127.0.0.1:7880` with `/livekit` prefix stripped.

---

## SPA (embedded frontend)

| Method | Path | Response |
|--------|------|----------|
| `*` | `/` | `frontend/index.html` (SSR homepage) |
| `*` | `/*` (non-API) | `frontend/shell.html` (client router shell) |
| `*` | static assets | From `embed.FS` via filesystem middleware |

---

## Planned routes (commented out in source)

Recording API routes and admin recording routes exist in handler code but are **not registered** in `server.go` yet. See [Planned Features](./planned-features.md).