# Handlers Reference

Complete index of HTTP handler structs and methods in `server/internal/handlers/`.

Routes are registered in `internal/server/server.go` and `cmd/server/main.go`. See [routes.md](./routes.md) for path → handler mapping.

**Public docs:** [API Handlers](https://bedrud.org/en/docs/backend/api-handlers) · [API Reference (Swagger)](https://bedrud.org/en/docs/api/api-reference) — [`backend/api-handlers.mdx`](../../apps/site/src/content/docs/en/backend/api-handlers.mdx). Full map: [public-docs.md](./public-docs.md).

---

## Handler inventory

| File | Struct | Methods | Domain |
|------|--------|---------|--------|
| `auth_handler.go` | `AuthHandler` | 24 | Registration, login, JWT, passkeys, verification |
| `auth.go` | — | 1 + `BeginAuthHandler` | OAuth callback and begin |
| `room.go` | `RoomHandler` | 40+ | Rooms, moderation, admin room ops, uploads |
| `users.go` | `UsersHandler` | 14 | Admin user management |
| `admin_handler.go` | `AdminHandler` | 14 | Settings, invites, webhooks, email test |
| `admin_overview.go` | `AdminOverviewHandler` | 1 + helpers | Dashboard KPIs |
| `admin_queue.go` | `AdminQueueHandler` | 1 | Queue stats |
| `preferences_handler.go` | `PreferencesHandler` | 2 | User preferences |
| `livekit_webhook.go` | `LiveKitWebhookHandler` | 1 + internal | LiveKit events |
| `cert_handler.go` | `CertHandler` | 2 | TLS cert info |
| `recording_handler.go` | `RecordingHandler` | 11 | **Planned** — routes commented out |

---

## AuthHandler (`auth_handler.go`)

| Method | Route | Middleware |
|--------|-------|------------|
| `Register` | `POST /auth/register` | AuthRateLimiter |
| `Login` | `POST /auth/login` | AuthRateLimiter |
| `GuestLogin` | `POST /auth/guest-login` | AuthRateLimiter |
| `RefreshToken` | `POST /auth/refresh` | AuthRateLimiter |
| `Logout` | `POST /auth/logout` | Protected |
| `GetMe` | `GET /auth/me` | Protected, RequireEmailVerified |
| `UpdateProfile` | `PUT /auth/me` | Protected, RequireEmailVerified |
| `ChangePassword` | `PUT /auth/password` | Protected, RequireEmailVerified |
| `VerifyEmail` | `POST /auth/verify` | — |
| `CheckVerificationStatus` | `GET /auth/verify/status` | Protected |
| `ResendVerification` | `POST /auth/verify/resend` | ResendRateLimiter |
| `ForgotPassword` | `POST /auth/forgot-password` | AuthRateLimiter |
| `ResetPassword` | `POST /auth/reset-password` | AuthRateLimiter |
| `PasskeyRegisterBegin` | `POST /auth/passkey/register/begin` | Protected, verified |
| `PasskeyRegisterFinish` | `POST /auth/passkey/register/finish` | Protected, verified |
| `PasskeyLoginBegin` | `POST /auth/passkey/login/begin` | AuthRateLimiter |
| `PasskeyLoginFinish` | `POST /auth/passkey/login/finish` | AuthRateLimiter |
| `PasskeySignupBegin` | `POST /auth/passkey/signup/begin` | AuthRateLimiter |
| `PasskeySignupFinish` | `POST /auth/passkey/signup/finish` | AuthRateLimiter |

**Dependencies:** `AuthService`, config, `SettingsRepository`, `InviteTokenRepository`, `ChallengeStore`, `CooldownCache`, `VerificationEventRepository`.

---

## OAuth (`auth.go`)

| Function | Route |
|----------|-------|
| `BeginAuthHandler` | `GET /auth/:provider/login` |
| `CallbackHandler` | `GET /auth/:provider/callback` |

---

## RoomHandler (`room.go`)

### User-facing

| Method | Route |
|--------|-------|
| `CreateRoom` | `POST /room/create` |
| `JoinRoom` | `POST /room/join` |
| `GuestJoinRoom` | `POST /room/guest-join` |
| `RefreshLiveKitToken` | `POST /room/refresh-token` |
| `ListRooms` | `GET /room/list` |
| `ListArchivedRooms` | `GET /room/archived` |
| `DeleteRoom` | `DELETE /room/:roomId` |
| `UpdateSettings` | `PUT /room/:roomId/settings` |
| `UploadChatImage` | `POST /room/:roomId/chat/upload` |
| `GetParticipantInfo` | `GET /room/:roomId/participant/:identity/info` |

### Moderation (all `POST`, room moderator required)

| Method | Route suffix |
|--------|--------------|
| `KickParticipant` | `/kick/:identity` |
| `MuteParticipant` | `/mute/:identity` |
| `BanParticipant` | `/ban/:identity` |
| `DisableParticipantVideo` | `/video/:identity/off` |
| `PromoteParticipant` | `/promote/:identity` |
| `DemoteParticipant` | `/demote/:identity` |
| `BlockChat` | `/chat/:identity/block` |
| `DeafenParticipant` | `/deafen/:identity` |
| `UndeafenParticipant` | `/undeafen/:identity` |
| `AskParticipantAction` | `/ask/:identity/:action` |
| `SpotlightParticipant` | `/spotlight/:identity` |
| `StopScreenShare` | `/screenshare/:identity/stop` |
| `BringToStage` | `/stage/:identity/bring` |
| `RemoveFromStage` | `/stage/:identity/remove` |

### Admin

| Method | Route |
|--------|-------|
| `AdminListRooms` | `GET /admin/rooms` |
| `ListRoomEvents` | `GET /admin/rooms/events` |
| `AdminGenerateToken` | `POST /admin/rooms/:roomId/token` |
| `AdminCloseRoom` | `DELETE /admin/rooms/:roomId` |
| `AdminSuspendRoom` | `POST /admin/rooms/:roomId/suspend` |
| `AdminReactivateRoom` | `POST /admin/rooms/:roomId/reactivate` |
| `AdminUpdateRoom` | `PUT /admin/rooms/:roomId` |
| `GetOnlineCount` | `GET /admin/online-count` |
| `AdminLiveKitStats` | `GET /admin/livekit/stats` |
| `AdminGetRoomParticipants` | `GET /admin/rooms/:roomId/participants` |
| `AdminKickParticipant` | `POST /admin/rooms/:roomId/kick/:identity` |
| `AdminMuteParticipant` | `POST /admin/rooms/:roomId/mute/:identity` |
| `BulkSuspendRooms` | `POST /admin/rooms/bulk-suspend` |
| `BulkCloseRooms` | `POST /admin/rooms/bulk-close` |
| `GetAdminStats` | `GET /admin/stats` |

**Internal helpers:** `resolveRoom`, `isRoomModerator` (via `room_auth.go`), `sendSystemMessage`, `dispatchRoomEvent`, `lkSessionStartedAt`, `maxParticipantsLimit`.

**Dependencies:** LiveKit client, config, repos (room, user, recording, settings, webhook), upload tracker, cleanup service.

---

## UsersHandler (`users.go`)

All under `/api/admin/users` — requires superadmin.

| Method | Route |
|--------|-------|
| `ListUsers` | `GET /users` |
| `ListRecentSignups` | `GET /users/recent` |
| `GetUserDetail` | `GET /users/:id` |
| `UpdateUserStatus` | `PUT /users/:id/status` |
| `UpdateUserAccesses` | `PUT /users/:id/accesses` |
| `DeleteUser` | `DELETE /users/:id` |
| `SetUserPassword` | `PUT /users/:id/password` |
| `ForceLogout` | `POST /users/:id/force-logout` |
| `ListUserSessions` | `GET /users/:id/sessions` |
| `AdminVerifyEmail` | `PUT /users/:id/verify` |
| `AdminResendVerification` | `POST /users/:id/resend-verification` |
| `BulkBanUsers` | `POST /users/bulk-ban` |
| `BulkPromoteUsers` | `POST /users/bulk-promote` |
| `BulkDeleteUsers` | `POST /users/bulk-delete` |

`DeleteUser` and bulk deletes enqueue `user_delete` jobs.

---

## AdminHandler (`admin_handler.go`)

| Method | Route |
|--------|-------|
| `GetSettings` | `GET /admin/settings` |
| `GetPublicSettings` | `GET /settings/public` (no admin) |
| `UpdateSettings` | `PUT /admin/settings` |
| `ValidateSettingsConnectivity` | `POST /admin/settings/validate` |
| `SendTestEmail` | `POST /admin/settings/test-email` |
| `ListInviteTokens` | `GET /admin/invite-tokens` |
| `CreateInviteToken` | `POST /admin/invite-tokens` |
| `DeleteInviteToken` | `DELETE /admin/invite-tokens/:id` |
| `ListWebhooks` | `GET /admin/webhooks` |
| `CreateWebhook` | `POST /admin/webhooks` |
| `UpdateWebhook` | `PUT /admin/webhooks/:id` |
| `DeleteWebhook` | `DELETE /admin/webhooks/:id` |
| `RotateWebhookSecret` | `POST /admin/webhooks/:id/rotate-secret` |
| `TestWebhook` | `POST /admin/webhooks/:id/test` |

---

## Other handlers

| Handler | Method | Route |
|---------|--------|-------|
| `AdminOverviewHandler` | `GetOverview` | `GET /admin/overview` |
| `AdminQueueHandler` | `GetQueueStats` | `GET /admin/queue/stats` |
| `PreferencesHandler` | `GetPreferences` | `GET /auth/preferences` |
| `PreferencesHandler` | `UpdatePreferences` | `PUT /auth/preferences` |
| `LiveKitWebhookHandler` | `Handle` | `POST /livekit/webhook` |
| `CertHandler` | `GetCert` | `GET /admin/cert` |
| `CertHandler` | `GetCertInfo` | `GET /admin/cert/info` |

---

## RecordingHandler (not wired)

Code exists in `recording_handler.go` but routes are commented out in `server.go`:

- `StartRecording`, `StopRecording`, `ListRecordings`, `GetRecording`, `WaitRecordingReady`
- `AdminListRecordings`, `BulkDeleteRecordings`, `ClearRoomRecordings`, `ClearSingleRecording`

See [planned-features.md](./planned-features.md).

---

## Adding a new handler

1. Create method on existing struct or new file in `internal/handlers/`
2. Add constructor `NewXHandler(...)` with explicit dependencies
3. Register route in `server.go` **and** `cmd/server/main.go`
4. Add Swagger annotations if public API
5. Add tests in `*_test.go` alongside handler

Recipe: [developer-guide.md](./developer-guide.md)

---

## Related docs

- [routes.md](./routes.md) — authoritative route table
- [api.md](./api.md) — API summary
- [internal/handlers.md](./internal/handlers.md) — package-level overview
- [architecture.md](./architecture.md) — request flow