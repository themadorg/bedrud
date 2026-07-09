---
name: bedrud-api
description: Complete Bedrud API endpoint reference. Dense index of every route, middleware, status code; deep detail in leaf skills.
license: Apache License
---

# Skill: bedrud-api

Umbrella API map for Bedrud. **Canonical prod routes:** `server/internal/server/server.go`. Dev Air: `server/cmd/server/main.go` (minor drift — see below).

Deep detail (bodies, guards, notes):
| Area | Leaf |
|------|------|
| Auth / JWT / verify / passkey / OAuth / prefs / health | `bedrud-api-auth` |
| Room CRUD / moderation / presence / recording contracts | `bedrud-api-rooms` |
| Admin users / rooms / queue / settings / invites / webhooks | `bedrud-api-admin` |
| DTOs, models, queue payloads, Swagger | `bedrud-api-types` |

**Path prefix rule**
- Room CRUD / join / moderation / chat / presence → **singular** `/api/room/...`
- Room-scoped recording (handlers shipped, routes **commented**) → **plural** `/api/rooms/:id/...`
- Admin → `/api/admin/...`

---

## Authentication (summary)

Access + refresh pair, HMAC-SHA256.

| Token | Duration | Cookie | Login JSON |
|-------|----------|--------|------------|
| Access | `tokenDuration` hours | `access_token` HttpOnly | `tokens.accessToken` |
| Refresh | 7 days | `refresh_token` HttpOnly | `tokens.refreshToken` |

- Refresh body / refresh response keys: **snake_case** `refresh_token` / `access_token`.
- Login body tokens: **camelCase** under `tokens`.
- Refresh rotation; concurrent reuse → **409**.
- Password length: **12–128** (`MinPasswordLength` / `MaxPasswordLength`).
- Errors: `{"error":"<message>"}` (+ optional `requiresVerification`, `email`, `already_verified`).

### Middleware

| Middleware | Behavior | Fail |
|-----------|----------|------|
| `Protected()` | Bearer or `access_token` cookie; banned → 403 | 401 / 403 |
| `RequireAccess(level)` | Hierarchical: superadmin(4) > admin(3) > moderator(2) > user(1) | 403 |
| `RequireEmailVerified` | When verification required; guests exempt | 403 |
| `AuthRateLimiter` | ~10/min/IP (config) | 429 |
| `ResendRateLimiter` | ~3/min/IP | 429 |
| `GuestRateLimiter` | ~5/min/IP — **guest-join only** | 429 |
| `APIRateLimiter` | ~30/min/IP — create / refresh-token / chat upload / presence | 429 |
| `RecordingsEnabled` | System setting gate (for recording routes when wired) | 403 |

`RequireBearerForMutations` / `RejectGuest` exist but are **not mounted**.

### Access levels

`superadmin` > `admin` > `moderator` > `user` > `guest`

### Global middleware (all routes)

1. `recover` → 2. `helmet` → 3. `cors` → 4. body limit 2MB

API group prefix: `/api`.

---

## Health / public static

| Method | Path | Auth | Res | Status |
|--------|------|------|-----|--------|
| GET | `/api/health` | none | `{status:"healthy",time}` | 200 |
| GET | `/api/ready` | none | ready / DB fail | 200 / 503 |
| GET | `/health`, `/ready` | none | redirect → `/api/...` | 307 |
| GET | `/api/cert` | none | PEM download | 200 / 404 |
| GET | `/uploads/avatars/*` | none | avatar file | 200 / 400 |
| GET | `/uploads/chat/*` | P+EV | chat upload file | 200 / … |
| POST | `/api/livekit/webhook` | LK JWT | disconnect (+ recording when wired) | — |

Swagger (dev `cmd/server` typically): `GET /api/swagger/*`, `GET /api/scalar`.

---

## Auth — Local

Auth: none + AuthRate unless noted. `P+EV` = Protected + RequireEmailVerified.

| Method | Path | Auth | Req | Res | Status |
|--------|------|------|-----|-----|--------|
| POST | `/api/auth/register` | AuthRate | `{email,password,name,inviteToken?}` | `LoginResponse` or verification gate | 200 / 400 / 403 / 409 / 500 |
| POST | `/api/auth/login` | AuthRate | `{email,password}` | `LoginResponse` | 200 / 400 / 401 / 403 |
| POST | `/api/auth/guest-login` | AuthRate | `{name}` | `LoginResponse` | 200 / 400 / 403 / 500 |
| POST | `/api/auth/refresh` | AuthRate | `{refresh_token}` or cookie | `{access_token,refresh_token}` | 200 / 400 / 401 / 403 / 409 |
| POST | `/api/auth/logout` | Protected† | `{refresh_token?}` or cookie | `{message}` | 200 |
| GET | `/api/auth/me` | P+EV | — | `models.User` | 200 |
| PUT | `/api/auth/me` | P+EV | `{name,email?}` | profile (+ verify fields) | 200 / 400 |
| POST | `/api/auth/me/avatar` | P+EV | multipart `avatar` | profile | 200 / 400 / 500 |
| DELETE | `/api/auth/me/avatar` | P+EV | — | profile | 200 / 400 |
| PUT | `/api/auth/password` | P+EV | `{currentPassword,newPassword}` | `{message}` | 200 / 400 |

† Prod: logout = `Protected()` only. Dev Air also mounts `RequireEmailVerified` on logout.

### Password reset

| Method | Path | Req | Status |
|--------|------|-----|--------|
| POST | `/api/auth/forgot-password` | `{email}` | 200 (uniform, no enum) / 400 |
| POST | `/api/auth/reset-password` | `{token,newPassword}` | 200 / 400 / 500 |

New password 12–128. Local/passkey only.

### Email verification

| Method | Path | Auth | Req | Status |
|--------|------|------|-----|--------|
| **POST** | `/api/auth/verify` | none | `{token}` | 200 / 400 / 401 / 404 / 409 / 500 |
| GET | `/api/auth/verify/status` | Protected | — | 200 |
| POST | `/api/auth/verify/resend` | ResendRate | `{email}` | 200 (uniform) / 400 |

Prod verify is **POST body token** (not GET query). Dev Air still registers `GET /auth/verify` (drift).

Success verify returns snake_case tokens + `verified:true` (no cookies set by handler).

### OAuth (Goth)

| Method | Path | Res |
|--------|------|-----|
| GET | `/api/auth/:provider/login` | 307 → IdP |
| GET | `/api/auth/:provider/callback` | redirect `{frontendURL}/auth/callback` (cookies only) or JSON |

Providers from `ConfiguredProviders()` when secrets set: `google`, `github`, `twitter`.

### Passkeys

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| POST | `/api/auth/passkey/register/begin` | P+EV | creation options |
| POST | `/api/auth/passkey/register/finish` | P+EV | `{clientDataJSON,attestationObject}` |
| POST | `/api/auth/passkey/login/begin` | AuthRate | request options |
| POST | `/api/auth/passkey/login/finish` | AuthRate | → `LoginResponse` |
| POST | `/api/auth/passkey/signup/begin` | AuthRate | `{email,name,inviteToken?}` |
| POST | `/api/auth/passkey/signup/finish` | AuthRate | → `LoginResponse` or verification gate |

### Preferences + public settings

| Method | Path | Auth | Req / Res |
|--------|------|------|-----------|
| GET | `/api/auth/preferences` | P+EV | `{preferencesJson}` (default `{}`) |
| PUT | `/api/auth/preferences` | P+EV | body ≤4KB JSON **object** |
| GET | `/api/auth/settings` | none | public: `serverName`, `registrationEnabled`, `tokenRegistrationOnly`, `guestLoginEnabled`, `passkeysEnabled`, `oauthProviders`, `requireEmailVerification`, `chatMaxMessageCount`, `chatMessageTTLHours`, `recordingsEnabled` |

---

## Rooms — CRUD + join + presence

Most: `P+EV`. Exceptions noted.

| Method | Path | MW | Handler | Req | Status |
|--------|------|-----|---------|-----|--------|
| POST | `/api/room/create` | P+EV + APIRate | `CreateRoom` | `CreateRoomRequest` | **200** / 400 / 403 / 409 / 500 |
| POST | `/api/room/join` | P+EV | `JoinRoom` | `{roomName}` | 200 / 403 / 404 / 410 |
| POST | `/api/room/guest-join` | GuestRate | `GuestJoinRoom` | `{roomName,guestName}` | 200 / 400 / 403 / 404 / 410 |
| POST | `/api/room/refresh-token` | P+EV + APIRate | `RefreshLiveKitToken` | `{roomName}` | 200 → `{token}` |
| GET | `/api/room/list` | P+EV | `ListRooms` | — | 200 `[]Room` |
| GET | `/api/room/archived` | P+EV | `ListArchivedRooms` | `?page&limit` | 200 |
| PUT | `/api/room/:roomId/settings` | P+EV | `UpdateSettings` | partial | 200 / 403 / 404 |
| DELETE | `/api/room/:roomId` | P+EV | `DeleteRoom` | — | **202** queued / 403 / 404 / 409 |
| POST | `/api/room/:roomId/chat/upload` | P+EV + APIRate | `UploadChatImage` | multipart `file` | 200 / **202** S3 queue / 413 / 507 |
| GET | `/api/room/:roomId/presence` | **APIRate only** | `GetRoomPresence` | `?countOnly` | 200 (no JWT) |

### CreateRoomRequest

```go
{ name string, maxParticipants int, isPublic bool, mode string, settings RoomSettings }
// mode: standard|webinar|broadcast; empty name → auto slug
// create forces settings.recordingsAllowed = true
// isPersistent stripped for non-superadmin
```

### Create / join response highlights

- Create: room fields + `livekitHost` (HTTP **200**, not 201).
- Join: + `token`, `adminId`, `expiresAt`, `activeRecordingId`.
- Archived owned: 200 `{status:"archived_owned",...}`; inactive non-owner: **410**.
- Guest: public+active only; needs `guestLoginEnabled`; `guest-` identity.

### Delete / upload notes

- User delete room: enqueues `room_delete` with `Purge:false` (archive; recordings preserved) → **202**.
- Chat upload: participant + `allowChat`; MIME png/jpeg/gif/webp; S3 over threshold → 202 `{status:"upload_queued",job_type:"chat_upload_s3"}`.

---

## Rooms — Moderation

All `P+EV`. Path `:roomId` + `:identity` (ask also `:action`).

| Method | Path | Handler | Authz | Notes |
|--------|------|---------|-------|-------|
| POST | `.../kick/:identity` | `KickParticipant` | room admin / superadmin | LK remove + system msg |
| POST | `.../ban/:identity` | `BanParticipant` | room admin / superadmin | + DB ban |
| POST | `.../mute/:identity` | `MuteParticipant` | room admin / superadmin | mute audio |
| POST | `.../video/:identity/off` | `DisableParticipantVideo` | `isRoomModerator` | mute camera |
| POST | `.../screenshare/:identity/stop` | `StopScreenShare` | `isRoomModerator` | mute SS tracks |
| POST | `.../promote/:identity` | `PromoteParticipant` | room admin / superadmin | + DB moderator |
| POST | `.../demote/:identity` | `DemoteParticipant` | room admin / superadmin | |
| POST | `.../chat/:identity/block` | `BlockChat` | `isRoomModerator` | metadata |
| POST | `.../deafen/:identity` | `DeafenParticipant` | `isRoomModerator` | data msg |
| POST | `.../undeafen/:identity` | `UndeafenParticipant` | `isRoomModerator` | |
| POST | `.../ask/:identity/:action` | `AskParticipantAction` | `isRoomModerator` | `unmute`\|`camera` |
| POST | `.../spotlight/:identity` | `SpotlightParticipant` | `isRoomModerator` | broadcast |
| GET | `.../participant/:identity/info` | `GetParticipantInfo` | self or mod | LK tracks |
| GET | `.../participant/:identity/profile` | `GetParticipantProfile` | caller in LK room | `{id,name,avatarUrl}` |
| POST | `.../stage/:identity/bring` | `BringToStage` | P+EV | **501** `{error:"not yet implemented"}` |
| POST | `.../stage/:identity/remove` | `RemoveFromStage` | P+EV | **501** same |

`isRoomModerator` = superadmin **or** room admin (AdminID‖CreatedBy) **or** DB `room_participants.is_moderator`. Kick/ban/mute/promote/demote are **stricter** (admin/superadmin only).

There is **no** live `/api/room/online-count` — use `GET /api/admin/online-count`.

---

## Recording — handlers shipped, routes not live

Handlers + service + tests exist. **Route registration is commented** in both entrypoints (`// TODO oncoming feature: recording routes`). Queue handlers `process_recording` / `recording_delete` **not registered**.

Prefix when enabled: **`/api/rooms/:id/...`** (plural). Intended MW: P+EV + `RecordingsEnabled`.

| Method | Path | Handler | Status (contract) |
|--------|------|---------|-------------------|
| POST | `/api/rooms/:id/recording/start` | `StartRecording` | 201 |
| POST | `/api/rooms/:id/recording/stop` | `StopRecording` | 200 |
| GET | `/api/rooms/:id/recordings` | `ListRecordings` | 200 |
| GET | `/api/rooms/:id/recordings/:rid` | `GetRecording` | 200 |
| GET | `/api/rooms/:id/recordings/:rid/wait` | `WaitRecordingReady` | 200 / 408 |
| DELETE | `/api/rooms/:id/recordings` | `ClearRoomRecordings` | 200 / 207 |
| DELETE | `/api/rooms/:id/recordings/:recordingId` | `ClearSingleRecording` | 200 |

Admin global (also commented): `GET /api/admin/recordings`, `POST /api/admin/recordings/bulk/delete` (`{ids}`).

Gates: system `recordingsEnabled` → room `settings.recordingsAllowed` → user role. Lifecycle: `pending→started→processing→completed|failed` (+`deleting`). DTO: see `bedrud-api-types` `RecordingDTO`.

---

## Admin

All: `Protected()` + `RequireEmailVerified` + `RequireAccess(superadmin)`. Prefix `/api/admin`.

**Bulk body:** `{ids: []string}` (`BulkIDsRequest`). Max 500.

**Bulk result:**
```json
{"results":{"<id>":{"success":true,"name":"...","error":"..."}},"totalProcessed":N,"totalFailed":N}
```

### Users

| Method | Path | Handler | Req | Status |
|--------|------|---------|-----|--------|
| GET | `/users` | `ListUsers` | filters: page,limit,q,provider,role,status,verified,created,sort,order | 200 |
| GET | `/users/recent` | `ListRecentSignups` | page,limit,q,provider,dateFrom/To,sort,order | 200 |
| GET | `/users/:id` | `GetUserDetail` | — | 200 / 404 |
| GET | `/users/:id/sessions` | `ListUserSessions` | page,limit | 200 †prod |
| PUT | `/users/:id/status` | `UpdateUserStatus` | `{active}` | 200 / 400 / 409 |
| PUT | `/users/:id/accesses` | `UpdateUserAccesses` | `{accesses:[]string}` | 200 / 400 / 409 |
| PUT | `/users/:id/password` | `SetUserPassword` | `{password}` 12–128 | 200 / 400 |
| POST | `/users/:id/force-logout` | `ForceLogout` | — | 200 |
| POST | `/users/:id/verify` | `AdminVerifyEmail` | — | 200 / 400 |
| POST | `/users/:id/verify/resend` | `AdminResendVerification` | — | 200 / 400 |
| DELETE | `/users/:id` | `DeleteUser` | — | **202** queued / 400 / 409 |
| POST | `/users/bulk/ban` | `BulkBanUsers` | `{ids}` | 200 |
| POST | `/users/bulk/promote` | `BulkPromoteUsers` | `{ids}` | 200 |
| POST | `/users/bulk/delete` | `BulkDeleteUsers` | `{ids}` | **202** `{message,count,skipped}` |

DeleteUser: soft-deactivate + ban then enqueue `user_delete`. Self / last-superadmin guards.

### Rooms & dashboard

| Method | Path | Handler | Status |
|--------|------|---------|--------|
| GET | `/overview` | `GetOverview` | 200 |
| GET | `/stats` | `GetAdminStats` | 200 **dev only** |
| GET | `/rooms` | `AdminListRooms` | 200 (rich filters) |
| GET | `/rooms/events` | `ListRoomEvents` | 200 |
| **DELETE** | `/rooms/:roomId` | `AdminCloseRoom` | **202** `{message:"Room close queued"}` / 409 |
| POST | `/rooms/:roomId/suspend` | `AdminSuspendRoom` | **202** |
| POST | `/rooms/:roomId/reactivate` | `AdminReactivateRoom` | 200 |
| PUT | `/rooms/:roomId` | `AdminUpdateRoom` | 200 partial settings |
| POST | `/rooms/:roomId/token` | `AdminGenerateToken` | **501** not implemented |
| GET | `/rooms/:roomId/participants` | `AdminGetRoomParticipants` | 200 |
| POST | `/rooms/.../participants/:identity/kick` | `AdminKickParticipant` | 200 |
| POST | `/rooms/.../participants/:identity/mute` | `AdminMuteParticipant` | 200 |
| POST | `/rooms/bulk/suspend` | `BulkSuspendRooms` | **202** `{ids}` |
| POST | `/rooms/bulk/close` | `BulkCloseRooms` | **202** `{ids}` |
| GET | `/online-count` | `GetOnlineCount` | 200 `{count}` |
| GET | `/livekit/stats` | `AdminLiveKitStats` | 200 |
| GET | `/cert-info` | `GetCertInfo` | 200 / 503 |

| Action | Method | Job | Effect |
|--------|--------|-----|--------|
| Close | `DELETE .../:roomId` | `room_delete` Purge=true | Full wipe LK + DB |
| Suspend | `POST .../suspend` | `room_suspend` | End calls; keep row inactive |
| Reactivate | `POST .../reactivate` | sync | IsActive=true, new 24h expiry, recreate LK |

AdminUpdateRoom: `{maxParticipants *int, settings *AdminUpdateRoomSettingsInput}` — `allowChat/Video/Audio`, `requireApproval`, `e2ee`, `isPersistent` (*bool merge). No `recordingsAllowed` in admin input.

### Queue

| Method | Path | Res |
|--------|------|-----|
| GET | `/queue` | `QueueStats` (+ email fields: pendingEmail, failedEmail24h, lastSendError*) |

### Job types (worker map in `server.go`)

| Type | Status |
|------|--------|
| `user_delete` | **active** |
| `room_delete` | **active** |
| `room_suspend` | **active** |
| `chat_upload_s3` | **active** |
| `send_email` | **active** (not a stub) |
| `dispatch_webhook` | **active** (not a stub) |
| `process_recording` | **not registered** (TODO) |
| `recording_delete` | **not registered** (TODO) |

Worker: poll + concurrency from `QueueConfig` / env.

### Settings

| Method | Path | Notes |
|--------|------|-------|
| GET | `/settings` | secrets masked as `••••••••` |
| PUT | `/settings` | **partial merge**; masked placeholder keeps existing |
| POST | `/settings/validate` | LiveKit / TLS / S3 / email checks |
| POST | `/settings/send-test-email` | `{to}` — **prod only** |

Masked: OAuth secrets, `jwtSecret`, `sessionSecret`, `livekitApiSecret`, `chatUploadS3AccessKey`, `chatUploadS3SecretKey`, `emailPassword`.

### Invite tokens

| Method | Path | Req | Status |
|--------|------|-----|--------|
| GET | `/invite-tokens` | page, limit | 200 `{tokens,total}` (+ computed `used`) |
| POST | `/invite-tokens` | `{email?, expiresInHours?}` | **201** |
| DELETE | `/invite-tokens/:id` | — | 200 |

`expiresInHours`: default **72**, max **720**. Token: 16 random bytes → 32 hex.

### Webhooks

| Method | Path | Req | Status |
|--------|------|-----|--------|
| GET | `/webhooks` | page, limit | 200 |
| POST | `/webhooks` | `{name,url,events[]}` | 201 (plaintext secret once) |
| PUT | `/webhooks/:id` | partial | 200 |
| DELETE | `/webhooks/:id` | — | 200 |
| POST | `/webhooks/:id/rotate-secret` | — | 200 `{secret}` |
| POST | `/webhooks/:id/test` | — | 200 status/latency |

Events: `room.created`, `room.ended`, `participant.joined`, `recording.completed`, `ping`.

### Admin recordings (not live)

Handlers exist; routes commented. Contracts: `GET /recordings`, `POST /recordings/bulk/delete` with `{ids}` → 202.

---

## Key DTO index (full shapes → `bedrud-api-types`)

| Name | Location |
|------|----------|
| `LoginResponse` / `TokenPair` | `internal/auth` — `tokens.accessToken` / `refreshToken` |
| `RefreshRequest` / `LogoutRequest` | snake_case `refresh_token` |
| `User` / `UserDetails` | includes `emailVerifiedAt`, accesses |
| `CreateRoomRequest`, `JoinRoomRequest`, `GuestJoinRoomRequest` | room handler |
| `RefreshLiveKitTokenRequest` | `{roomName}` → `{token}` |
| `Room` / `RoomSettings` | + `recordingsAllowed`, `isPersistent` |
| `BulkIDsRequest` / `BulkResult` | `{ids}` |
| `RecordingDTO` | shipped type; routes may be unwired |
| `SystemSettings` / public settings | admin + auth |
| `InviteToken` | create: `expiresInHours` |
| `QueueStats` | + email counters |
| `OverviewResponse` | admin overview |
| `webhookDTO` / create/update webhook | admin |
| Queue payloads | snake_case in `internal/queue/job.go` |

Password constants: `MinPasswordLength=12`, `MaxPasswordLength=128`.

---

## Entrypoint drift (prod vs dev)

| Item | Prod `server.go` | Dev `cmd/server/main.go` |
|------|------------------|--------------------------|
| `GET /admin/stats` | ❌ | ✅ |
| `GET /admin/users/:id/sessions` | ✅ | ❌ |
| `POST /admin/settings/send-test-email` | ✅ | ❌ |
| Verify email | **POST** `/auth/verify` | **GET** `/auth/verify` |
| Logout MW | `Protected` only | + `RequireEmailVerified` |
| Recording room/admin routes | commented | commented |
| Swagger / Scalar | often prod-off | mounted in dev |

**Canonical for agents:** prod bootstrap `server/internal/server/server.go`.

---

## Source file index (high traffic)

| Concern | Path under `server/` |
|---------|----------------------|
| Routes prod | `internal/server/server.go` |
| Routes dev | `cmd/server/main.go` |
| Auth handler | `internal/handlers/auth_handler.go` |
| OAuth | `internal/handlers/auth.go` |
| Rooms | `internal/handlers/room.go`, `room_auth.go` |
| Recording (unwired routes) | `internal/handlers/recording_handler.go` |
| Users admin | `internal/handlers/users.go` |
| Settings / invites / webhooks | `internal/handlers/admin_handler.go` |
| Overview | `internal/handlers/admin_overview.go` |
| Queue stats | `internal/handlers/admin_queue.go` |
| Preferences | `internal/handlers/preferences_handler.go` |
| Shared DTOs | `internal/handlers/models.go` |
| Middleware | `internal/middleware/auth.go`, `ratelimit.go` |
| Queue types / handlers | `internal/queue/job.go`, `handler_*.go` |
| Models | `internal/models/*.go` |
| Swagger | `docs/swagger.yaml` — regen `make swagger-gen` |

---

## Swagger

- UI: `GET /api/swagger/*` · Scalar: `GET /api/scalar`
- Base path `/api` · Bearer security
- Regen: `make swagger-gen` (`swag` CLI)
