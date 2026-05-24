---
name: bedrud-api-admin
description: Admin endpoints — users, rooms, queue, settings, invite tokens.
license: Apache License
---

# Bedrud API — Admin Endpoints

---

All routes: `Protected()` + `RequireAccess(superadmin)`. Prefix: `/api/admin`.

---

## Admin — Users

| Method | Path | Req | Res | Status |
|--------|------|-----|-----|--------|
| GET | `/api/admin/users` | query: `page, limit, q, provider` | `{"users":[UserDetails],"total":int,"page":int,"limit":int}` | 200 |
| GET | `/api/admin/users/recent` | query: `page, limit, q, provider, dateFrom, dateTo` | `{"users":[RecentUser],"total":int,"page":int,"limit":int}` | 200 |
| GET | `/api/admin/users/:id` | — | `{"user":UserDetails,"rooms":[Room]}` | 200 |
| PUT | `/api/admin/users/:id/status` | `{active: bool}` | `{"message":"User status updated successfully"}` | 200 |
| PUT | `/api/admin/users/:id/accesses` | `{accesses: []string}` | `{"message":"User accesses updated"}` | 200 |
| PUT | `/api/admin/users/:id/password` | `{password: string}` | `{"message":"Password updated successfully"}` | 200 |
| POST | `/api/admin/users/:id/force-logout` | — | `{"message":"User logged out"}` | 200 |
| POST | `/api/admin/users/:id/verify` | — | `{"message":"Email verified"}` | 200 |
| POST | `/api/admin/users/:id/verify/resend` | — | `{"message":"Verification email sent"}` | 200 |
| DELETE | `/api/admin/users/:id` | — | `202 {"message":"User deletion started","rooms":N}` | 202 / 400 / 403 / 404 / 500 |
| POST | `/api/admin/users/bulk/ban` | `{userIds: []string}` | `{"message":"N users banned"}` | 200 |
| POST | `/api/admin/users/bulk/promote` | `{userIds: []string}` | `{"message":"N users promoted"}` | 200 |
| POST | `/api/admin/users/bulk/delete` | `{userIds: []string}` | `202 {"message":"N user deletions started"}` | 202 |

### DeleteUser Notes
- **Self-deletion guard**: 400 if targeting own ID.
- **3-phase async**: Phase 1 (notify + stop LK rooms) → Phase 2 (hard-delete rooms + chat cleanup) → Phase 3 (delete passkeys, prefs, user). 202 Accepted.
- **Partial failure**: LK failures non-fatal. DB failures abort entire deletion.

---

## Admin — Rooms

| Method | Path | Req | Res |
|--------|------|-----|-----|
| GET | `/api/admin/stats` | — | Aggregate KPIs |
| GET | `/api/admin/overview` | — | `OverviewResponse` (health, KPIs, activity, composition, attention) |
| GET | `/api/admin/rooms` | query: `page, limit` | `{"rooms":[],"total":int,"page":int,"limit":int}` |
| GET | `/api/admin/rooms/events` | query: `page, limit, type, search` | `{"events":[RoomEvent],...}` |
| POST | `/api/admin/rooms/bulk/suspend` | `{roomIds: []string}` | `{"message":"N rooms queued for suspension"}` |
| POST | `/api/admin/rooms/bulk/close` | `{roomIds: []string}` | `{"message":"N rooms queued for deletion"}` |
| POST | `/api/admin/rooms/:roomId/suspend` | — | `{"status":"success"}` |
| POST | `/api/admin/rooms/:roomId/reactivate` | — | `{"status":"success"}` |
| POST | `/api/admin/rooms/:roomId/close` | — | `{"status":"success"}` |
| PUT | `/api/admin/rooms/:roomId` | `{maxParticipants *int, settings *AdminUpdateRoomSettingsInput}` | `models.Room` |
| POST | `/api/admin/rooms/:roomId/token` | — | 501 stub |
| GET | `/api/admin/rooms/:roomId/participants` | — | `{"participants":[...],"room":Room}` |
| POST | `/api/admin/rooms/:roomId/participants/:identity/kick` | — | `{"status":"success"}` |
| POST | `/api/admin/rooms/:roomId/participants/:identity/mute` | — | `{"status":"success"}` |
| GET | `/api/admin/online-count` | — | `{"count":int}` |
| GET | `/api/admin/livekit/stats` | — | `{"totalParticipants":42,"totalPublishers":10,"activeRooms":5}` |
| GET | `/api/admin/cert-info` | — | Certificate metadata |
| POST | `/api/admin/settings/validate` | — | Runtime checks |

### AdminUpdateRoom Notes
- Partial merge: only sent fields override. `isPersistent` superadmin-only.
- Close vs delete: close removes from LK + marks inactive (record preserved). Delete removes from LK + DB.

---

## Admin — Queue

| Method | Path | Res |
|--------|------|-----|
| GET | `/api/admin/queue` | `QueueStats` |

### Queue Stats Response
```json
{"pending":3,"active":1,"done24h":150,"failed24h":2,"total":200,"maxDepth":50,"oldestPending":"...","recentFailures":[...],"processedPerMin":5.2,"failedPerMin":0.1,"failRate":0.013,"pendingEmail":0,"failedEmail24h":0,"lastSendError":"...","lastSendErrorAt":"..."}
```

> **TODO oncoming feature:** Recording functionality is planned for a future release.
7 job types: `user_delete`, `room_delete`, `room_suspend`, `chat_upload_s3` (active) + `send_email`, `dispatch_webhook`, `process_recording` (stubs).

---

## Admin — Settings

| Method | Path | Req | Res |
|--------|------|-----|-----|
| GET | `/api/admin/settings` | — | `SystemSettings` (secrets masked) |
| PUT | `/api/admin/settings` | `SystemSettings` (full body) | `SystemSettings` (secrets masked) |

### Masked Fields
`googleClientSecret`, `githubClientSecret`, `twitterClientSecret`, `jwtSecret`, `sessionSecret`, `livekitApiSecret`, `chatUploadS3SecretKey` → return `"******"`.

PUT replaces entire settings. Singleton ID=1.

---

## Admin — Invite Tokens

| Method | Path | Req | Res | Status |
|--------|------|-----|-----|--------|
| GET | `/api/admin/invite-tokens` | — | `{"tokens":[{InviteToken + used bool}]}` | 200 |
| POST | `/api/admin/invite-tokens` | `{email string, expiresIn int}` | `InviteToken` | 201 |
| DELETE | `/api/admin/invite-tokens/:id` | — | `{"status":"success"}` | 200 |

Token: crypto-random hex, varchar(64). Email: optional pre-bind. `expiresIn`: hours, default 72.

---

## Admin — Webhooks

| Method | Path | Req | Res | Status |
|--------|------|-----|-----|--------|
| GET | `/api/admin/webhooks` | — | `[Webhook]` | 200 |
| POST | `/api/admin/webhooks` | `{url, events[], secret?, isActive}` | `Webhook` (secret returned) | 201 |
| PUT | `/api/admin/webhooks/:id` | `{url?, events[], isActive?}` | `Webhook` | 200 |
| DELETE | `/api/admin/webhooks/:id` | — | `{success: true}` | 200 |
| POST | `/api/admin/webhooks/:id/rotate-secret` | — | `{id, secret}` | 200 |
| POST | `/api/admin/webhooks/:id/test` | — | `{success, statusCode}` | 200 |

> **TODO oncoming feature:** Recording functionality is planned for a future release.
Events: `room.ended`, `participant.joined`, `participant.left`, `recording.completed`.
Secret returned once on create/rotate, masked in list/get.

## Recordings Settings

> **TODO oncoming feature:** Recording functionality is planned for a future release.

`RecordingsEnabled` boolean in `SystemSettings`. Toggle in admin UI Settings → General.
Per-room `recordingsAllowed` in `RoomSettings`.
