---
name: bedrud-api-admin
description: Admin endpoints — users, rooms, queue, settings, invite tokens, webhooks, overview, recordings.
license: Apache License
---

# Bedrud API — Admin Endpoints

---

All routes: `Protected()` + `RequireEmailVerified` + `RequireAccess(superadmin)`.
Prefix: `/api/admin`.

**Registration sources:**
- Prod: `server/internal/server/server.go`
- Dev API: `server/cmd/server/main.go` (minor drift — see Notes)

**Bulk body:** all bulk endpoints use `{ids: []string}` (`BulkIDsRequest`). Max 500 IDs.

**Bulk result:**
```json
{"results":{"<id>":{"success":true,"name":"...","error":"..."}},"totalProcessed":N,"totalFailed":N}
```

---

## Admin — Users

| Method | Path | Handler | Req | Res | Status |
|--------|------|---------|-----|-----|--------|
| GET | `/api/admin/users` | `ListUsers` | query filters | `{users:[UserDetails], total, page, limit}` | 200 |
| GET | `/api/admin/users/recent` | `ListRecentSignups` | query filters | `{users:[], total, page, limit}` | 200 |
| GET | `/api/admin/users/:id` | `GetUserDetail` | — | `{user:UserDetails, rooms:[Room]}` | 200 / 404 |
| GET | `/api/admin/users/:id/sessions` | `ListUserSessions` | `page, limit` | `{sessions:[], total, page, limit}` | 200 / 404 |
| PUT | `/api/admin/users/:id/status` | `UpdateUserStatus` | `{active: bool}` | `{message:"User status updated successfully"}` | 200 / 400 / 409 |
| PUT | `/api/admin/users/:id/accesses` | `UpdateUserAccesses` | `{accesses: []string}` | `{message:"User accesses updated"}` | 200 / 400 / 409 |
| PUT | `/api/admin/users/:id/password` | `SetUserPassword` | `{password: string}` | `{message:"Password updated successfully"}` | 200 / 400 |
| POST | `/api/admin/users/:id/force-logout` | `ForceLogout` | — | `{message:"All sessions revoked"}` | 200 |
| POST | `/api/admin/users/:id/verify` | `AdminVerifyEmail` | — | `{message:"Email verified successfully"}` | 200 / 400 |
| POST | `/api/admin/users/:id/verify/resend` | `AdminResendVerification` | — | `{message:"Verification email sent"}` | 200 / 400 |
| DELETE | `/api/admin/users/:id` | `DeleteUser` | — | `{message:"User deletion queued", rooms:N}` | 202 / 400 / 409 |
| POST | `/api/admin/users/bulk/ban` | `BulkBanUsers` | `{ids:[]string}` | `BulkResult` | 200 |
| POST | `/api/admin/users/bulk/promote` | `BulkPromoteUsers` | `{ids:[]string}` | `BulkResult` | 200 |
| POST | `/api/admin/users/bulk/delete` | `BulkDeleteUsers` | `{ids:[]string}` | `{message, count, skipped}` | 202 |

### ListUsers query
`page` (default 1), `limit` (default 50, max 100), `q`, `provider` (csv: local/google/github/guest), `role` (csv: superadmin/admin/moderator/user/guest), `status` (csv: active/banned), `verified` (true/false), `created` (today/7d/30d), `sort` (name/email/provider/createdAt), `order` (asc/desc).

### ListRecentSignups query
`page`, `limit`, `q`, `provider` (csv; default excludes guests), `dateFrom`/`dateTo` (YYYY-MM-DD), `sort` (createdAt/name), `order`.

### UserDetails
```go
{id, email, name, provider, isActive, isAdmin, accesses []string, emailVerifiedAt *string, createdAt}
```

### Sessions item
```json
{"id":"...","roomId":"...","roomName":"...","joinedAt":"RFC3339","leftAt":null,"isActive":true,"durationSeconds":120}
```

### Guards
- Self-modify blocked: status, accesses, ban, promote, delete (400 / bulk skip).
- Last-superadmin guard: cannot demote/ban/delete sole superadmin (409 / bulk skip).
- Password: 12–128 chars (`MinPasswordLength` / `MaxPasswordLength`). Accesses/password clear refresh token.

### DeleteUser
- Soft-deactivates + ban immediately, then enqueues `user_delete` (priority 1).
- 202 Accepted. 409 if deletion already in flight.
- Payload: `{userId, email, roomIds[]}`.

### Bulk delete
```json
{"message":"Deletion queued for N users","count":N,"skipped":["id","id(self)","id(last-superadmin)"]}
```

---

## Admin — Rooms & Dashboard

| Method | Path | Handler | Req | Res | Status |
|--------|------|---------|-----|-----|--------|
| GET | `/api/admin/overview` | `GetOverview` | — | `OverviewResponse` | 200 |
| GET | `/api/admin/stats` | `GetAdminStats` | — | aggregate KPIs | 200 † |
| GET | `/api/admin/rooms` | `AdminListRooms` | query filters | `{rooms:[enriched], total, page, limit}` | 200 |
| GET | `/api/admin/rooms/events` | `ListRoomEvents` | query filters | `{events:[RoomEvent], total, page, limit}` | 200 |
| DELETE | `/api/admin/rooms/:roomId` | `AdminCloseRoom` | — | `{message:"Room close queued"}` | 202 / 409 |
| POST | `/api/admin/rooms/:roomId/suspend` | `AdminSuspendRoom` | — | `{message:"Room suspension queued"}` | 202 / 400 |
| POST | `/api/admin/rooms/:roomId/reactivate` | `AdminReactivateRoom` | — | `models.Room` | 200 / 400 |
| PUT | `/api/admin/rooms/:roomId` | `AdminUpdateRoom` | partial room | `models.Room` | 200 |
| POST | `/api/admin/rooms/:roomId/token` | `AdminGenerateToken` | — | `{error:"not yet implemented"}` | **501** |
| GET | `/api/admin/rooms/:roomId/participants` | `AdminGetRoomParticipants` | — | `{participants:[...], room:Room}` | 200 |
| POST | `/api/admin/rooms/:roomId/participants/:identity/kick` | `AdminKickParticipant` | — | `{status:"success"}` | 200 |
| POST | `/api/admin/rooms/:roomId/participants/:identity/mute` | `AdminMuteParticipant` | — | `{status:"success"}` | 200 |
| POST | `/api/admin/rooms/bulk/suspend` | `BulkSuspendRooms` | `{ids:[]string}` | `BulkResult` | **202** |
| POST | `/api/admin/rooms/bulk/close` | `BulkCloseRooms` | `{ids:[]string}` | `BulkResult` | **202** |
| GET | `/api/admin/online-count` | `GetOnlineCount` | — | `{count:int}` | 200 |
| GET | `/api/admin/livekit/stats` | `AdminLiveKitStats` | — | LiveKit aggregate | 200 |
| GET | `/api/admin/cert-info` | `GetCertInfo` | — | cert metadata | 200 / 503 |

† `GET /stats` registered in **dev** `cmd/server/main.go` only — not in prod `server.go`. Handler exists.

### AdminListRooms query
`page`, `limit` (max 100), `q`, `visibility` (public/private csv), `status` (active/suspended/archived csv), `occupancy` (empty/1-5/6-20/20+), `capacity` (legacy, same buckets), `created` (today/7d/30d), `owner`, `dateFrom`/`dateTo`, `lastActivityFrom`/`lastActivityTo`, `sort` (name/createdAt/maxParticipants/participantsCount/lastActivityAt/createdBy), `order`.

### ListRoomEvents query
`page`, `limit`, `q`, `type` (csv: room_created, room_joined), `dateFrom`/`dateTo` (YYYY-MM-DD), `order`.

### Close vs suspend vs reactivate
| Action | Method | Queue job | Effect |
|--------|--------|-----------|--------|
| Close | `DELETE .../:roomId` | `room_delete` (Purge=true) | Full wipe LK + DB |
| Suspend | `POST .../suspend` | `room_suspend` | End calls, keep room row inactive |
| Reactivate | `POST .../reactivate` | sync | `IsActive=true`, new 24h expiry, recreate LK room |

### AdminUpdateRoom body
```go
{ maxParticipants *int, settings *AdminUpdateRoomSettingsInput }
// AdminUpdateRoomSettingsInput — all *bool merge:
// allowChat, allowVideo, allowAudio, requireApproval, e2ee, isPersistent
```
Partial merge only. `isPersistent` superadmin-only path (this endpoint). No `recordingsAllowed` in admin input (room-level field lives on `RoomSettings` elsewhere).

### LiveKit stats
```json
{"totalParticipants":42,"totalPublishers":10,"activeRooms":5,"rooms":[{"name":"...","numParticipants":8,"numPublishers":3,"creationTime":...}]}
```

### OverviewResponse (`GET /overview`)
```go
{
  health: {status, tls?, realtime, alertsCount, uptimeSeconds, dbStatus},
  kpis: {totalUsers, onlineNow, totalRooms, activeSessions, pendingActions}, // each KpiEntry
  activityTrend: [{date, roomsCreated, roomsActive, participants}], // 7 days
  roomComposition: {live, public, private, persistent, stale},
  needsAttention: [{type, severity, message, ...}],
  recentSignups: [{id, name, email, provider, createdAt}],
  recentRoomEvents: [RoomEvent],
  instanceInfo: {name, version, uptimeSeconds, startedAt}
}
```
Online count prefers LiveKit ListRooms; falls back to DB participants.

### Stats (`GET /stats`)
```json
{"totalRooms","activeRooms","privateRooms","publicRooms","emptyRooms","flaggedRooms":0,"pendingActions":0,"roomsLast24h","roomsLast7d","avgUsersPerRoom","onlineUsers","totalUsers","staleRooms","moderationFlags":0}
```

### Cert info
```json
{"enabled":true,"status":"valid|expiring|expired|error|not_configured","daysRemaining":N,"notAfter","subject","issuer","notBefore","sans":[]}
```
TLS off → `{enabled:false, status:"not_configured"}`.

---

## Admin — Queue

| Method | Path | Handler | Res |
|--------|------|---------|-----|
| GET | `/api/admin/queue` | `GetQueueStats` | `QueueStats` |

```json
{
  "pending":3,"active":1,"done24h":150,"failed24h":2,"total":200,"maxDepth":50,
  "oldestPending":"...","recentFailures":[{"id","type","error","attempts","updatedAt","age"}],
  "processedPerMin":5.2,"failedPerMin":0.1,"failRate":0.013,
  "pendingEmail":0,"failedEmail24h":0,"lastSendError":"...","lastSendErrorAt":"..."
}
```

### Job types (worker map in `server.go`)
| Type | Status |
|------|--------|
| `user_delete` | active |
| `room_delete` | active |
| `room_suspend` | active |
| `chat_upload_s3` | active |
| `send_email` | active |
| `dispatch_webhook` | active |
| `process_recording` | **TODO** — not registered |
| `recording_delete` | **TODO** — not registered |

Worker: poll interval + concurrency from `QueueConfig` / env.

---

## Admin — Settings

| Method | Path | Handler | Req | Res |
|--------|------|---------|-----|-----|
| GET | `/api/admin/settings` | `GetSettings` | — | `SystemSettings` (secrets masked) |
| PUT | `/api/admin/settings` | `UpdateSettings` | partial JSON | `SystemSettings` (masked) |
| POST | `/api/admin/settings/validate` | `ValidateSettingsConnectivity` | partial settings | `{checks:{livekit?,s3?,tls?,email?}}` |
| POST | `/api/admin/settings/send-test-email` | `SendTestEmail` | `{to:string}` | success / SMTP error | 200 / 400 / 408 ‡ |

‡ `send-test-email` registered in **prod** `server.go` only — not in dev `cmd/server/main.go`.

### Masked secrets
Placeholder: `••••••••` (not asterisks). Fields:
`googleClientSecret`, `githubClientSecret`, `twitterClientSecret`, `jwtSecret`, `sessionSecret`, `livekitApiSecret`, `chatUploadS3AccessKey`, `chatUploadS3SecretKey`, `emailPassword`.

PUT is **partial merge**: only keys present in body apply. Sending masked placeholder keeps existing secret. Validates ranges/URLs/CORS/TLS cross-fields. Reloads OAuth providers after save. Singleton ID=1.

### Validate checks
Returns only checks for fields present: LiveKit ListRooms, TLS cert pair, S3, email MX. Each: `{status:"ok"|"error"|"skipped", message?}`.

### Recordings setting
`recordingsEnabled` bool on `SystemSettings`. Public via `GET /api/auth/settings` as `recordingsEnabled`. Per-room: `RoomSettings.recordingsAllowed`.

---

## Admin — Invite Tokens

| Method | Path | Handler | Req | Res | Status |
|--------|------|---------|-----|-----|--------|
| GET | `/api/admin/invite-tokens` | `ListInviteTokens` | `page, limit` | `{tokens:[{InviteToken + used bool}], total}` | 200 |
| POST | `/api/admin/invite-tokens` | `CreateInviteToken` | `{email?, expiresInHours?}` | `InviteToken` | 201 |
| DELETE | `/api/admin/invite-tokens/:id` | `DeleteInviteToken` | — | `{status:"success"}` | 200 |

- Token: crypto-random hex (16 bytes → 32 hex chars).
- `expiresInHours`: default 72, max 720.
- Email: optional pre-bind; validated if set.
- List adds computed `used` (`usedAt != nil`).

---

## Admin — Webhooks

| Method | Path | Handler | Req | Res | Status |
|--------|------|---------|-----|-----|--------|
| GET | `/api/admin/webhooks` | `ListWebhooks` | `page, limit` | `{webhooks:[], total, page, limit}` | 200 |
| POST | `/api/admin/webhooks` | `CreateWebhook` | `{name, url, events[]}` | webhookDTO (plaintext secret) | 201 |
| PUT | `/api/admin/webhooks/:id` | `UpdateWebhook` | `{name?, url?, events[]?, isActive?}` | webhookDTO (masked secret) | 200 |
| DELETE | `/api/admin/webhooks/:id` | `DeleteWebhook` | — | `{status:"deleted"}` | 200 |
| POST | `/api/admin/webhooks/:id/rotate-secret` | `RotateWebhookSecret` | — | `{secret}` | 200 |
| POST | `/api/admin/webhooks/:id/test` | `TestWebhook` | — | `{status, httpStatus?, latencyMs, error?}` | 200 |

### Allowed events
`room.created`, `room.ended`, `participant.joined`, `recording.completed`, `ping`.

Secret returned once on create/rotate; list/get mask via `MaskedSecret()`. Test sends HMAC-signed `ping` with headers `X-Bedrud-Signature`, `X-Bedrud-Event`, `X-Bedrud-Timestamp`.

---

## Admin — Recordings

> **Not live.** Handlers + Swagger exist; route registration is commented in both entrypoints (`// TODO oncoming feature: admin recording routes`). Queue handlers `process_recording` / `recording_delete` also unregistered.

| Method | Path | Handler | Req | Res | Status |
|--------|------|---------|-----|-----|--------|
| GET | `/api/admin/recordings` | `AdminListRecordings` | filters | `{recordings:[RecordingDTO], total, page, limit}` | 200 |
| POST | `/api/admin/recordings/bulk/delete` | `BulkDeleteRecordings` | `{ids:[]string}` | `BulkResult` | 202 |

### List query (when wired)
`page` (default 1), `perPage` (default 1000, max 1000; invalid → 20), `roomId` (max 36 chars), `status` (completed/processing/failed/deleting), `createdAfter`/`createdBefore` (RFC3339).

### RecordingDTO
```json
{"id","recordingType","durationMs","fileSize","fileUrl","status","error","downloadStatus","roomId","roomName","createdBy","createdAt"}
```
`downloadStatus`: `ready` | `processing` | `failed`.

### Bulk delete (when wired)
Marks each recording `deleting`, enqueues `recording_delete`. Superadmin check inside handler (in addition to group middleware). Max 500 IDs.

---

## Shared DTOs

```go
type BulkIDsRequest struct {
  IDs []string `json:"ids"`
}
type BulkItemResult struct {
  Success bool   `json:"success"`
  Name    string `json:"name,omitempty"`
  Error   string `json:"error,omitempty"`
}
type BulkResult struct {
  Results        map[string]BulkItemResult `json:"results"`
  TotalProcessed int                       `json:"totalProcessed"`
  TotalFailed    int                       `json:"totalFailed"`
}
```

Errors: always `{"error":"<message>"}`.

---

## Entrypoint drift

| Route | Prod `server.go` | Dev `cmd/server/main.go` |
|-------|------------------|--------------------------|
| `GET /stats` | ❌ | ✅ |
| `GET /users/:id/sessions` | ✅ | ❌ |
| `POST /settings/send-test-email` | ✅ | ❌ |
| Recording admin | ❌ (commented) | ❌ (commented) |

Canonical prod bootstrap: `server/internal/server/server.go`.

---

## Source index

| Concern | File |
|---------|------|
| Route registration (prod) | `server/internal/server/server.go` |
| Route registration (dev) | `server/cmd/server/main.go` |
| Users admin | `server/internal/handlers/users.go` |
| Rooms admin | `server/internal/handlers/room.go` |
| Settings / invites / webhooks | `server/internal/handlers/admin_handler.go` |
| Overview | `server/internal/handlers/admin_overview.go` |
| Queue stats | `server/internal/handlers/admin_queue.go` |
| Recordings admin (unwired) | `server/internal/handlers/recording_handler.go` |
| Cert info | `server/internal/handlers/cert_handler.go` |
| Bulk DTOs | `server/internal/handlers/models.go` |
| Overview models | `server/internal/models/stats.go` |
| QueueStats | `server/internal/models/queue_stats.go` |
| Webhook events | `server/internal/models/webhook.go` |
