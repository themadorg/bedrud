---
name: bedrud-api-rooms
description: Room endpoints — CRUD, join, moderation, presence, chat upload, stage stubs, recording.
license: Apache License
---

# Bedrud API — Room Endpoints

Source of truth: `server/internal/server/server.go` and `server/cmd/server/main.go` route registration.
Handlers: `server/internal/handlers/room.go`, `server/internal/handlers/recording_handler.go`.

**Path prefix rule**
- Room CRUD / join / moderation / chat / presence → **singular** `/api/room/...`
- Recording (room-scoped) → **plural** `/api/rooms/:id/...`
- Admin room ops / online-count → `/api/admin/...` (see `bedrud-api-admin`, not listed here)

**Auth shorthand**
- `P+EV` = `Protected()` + `RequireEmailVerified`
- Most room routes: `P+EV`. Exceptions called out per row.
- Errors: `{"error":"<msg>"}` unless noted.

---

## Rooms — CRUD + Join + Presence

| Method | Path | Auth / MW | Handler | Req | Res | Status |
|--------|------|-----------|---------|-----|-----|--------|
| POST | `/api/room/create` | P+EV + APIRate | `CreateRoom` | `CreateRoomRequest` | room + `livekitHost` | 200 / 400 / 403 / 409 / 500 |
| POST | `/api/room/join` | P+EV | `JoinRoom` | `{roomName}` | join payload + token | 200 / 403 / 404 / 410 / 500 |
| POST | `/api/room/guest-join` | GuestRate only | `GuestJoinRoom` | `{roomName, guestName}` | guest join payload + token | 200 / 400 / 403 / 404 / 410 |
| POST | `/api/room/refresh-token` | P+EV + APIRate | `RefreshLiveKitToken` | `{roomName}` | `{token}` | 200 / 400 / 403 / 404 / 410 |
| GET | `/api/room/list` | P+EV | `ListRooms` | — | `[]models.Room` (caller’s latest) | 200 |
| GET | `/api/room/archived` | P+EV | `ListArchivedRooms` | `?page=&limit=` | `{rooms[], total, page, limit}` | 200 |
| PUT | `/api/room/:roomId/settings` | P+EV | `UpdateSettings` | partial settings body | `models.Room` | 200 / 400 / 403 / 404 |
| DELETE | `/api/room/:roomId` | P+EV | `DeleteRoom` | — | `{"message":"Room deletion queued"}` | **202** / 403 / 404 / 409 / 500 |
| POST | `/api/room/:roomId/chat/upload` | P+EV + APIRate | `UploadChatImage` | multipart `file` | `ChatAttachment` or queued | 200 / 202 / 400 / 403 / 404 / 413 / 507 |
| GET | `/api/room/:roomId/presence` | **APIRate only** (no JWT) | `GetRoomPresence` | `?countOnly=1` optional | `{participants[]}` or `{count}` | 200 / 404 / 500 |

### CreateRoomRequest
```go
{ name string, maxParticipants int, isPublic bool, mode string, settings RoomSettings }
```
- `mode`: `standard` (default) | `webinar` | `broadcast`
- Name: trim + lowercase; empty → auto-gen URL-safe slug. 409 on conflict.
- `settings.recordingsAllowed` forced `true` on create.
- Non-superadmin: `isPersistent` stripped; enforces `MaxRoomsPerUser` (403).
- Max participants clamped by `MaxParticipantsLimit` (settings).
- **Note:** handler returns `c.JSON(...)` → **HTTP 200** (not 201).

### Create Response
```json
{"id":"uuid","name":"xxx-xxxx-xxx","createdBy":"uuid","isActive":true,"isPublic":false,"maxParticipants":20,"settings":{},"livekitHost":"ws://...","mode":"standard"}
```

### Join Response (active room)
```json
{"id":"uuid","name":"room-name","token":"lk-jwt","createdBy":"uuid","adminId":"uuid","isActive":true,"isPublic":false,"maxParticipants":20,"expiresAt":"...","settings":{},"livekitHost":"ws://...","mode":"standard","activeRecordingId":""}
```

### Join edge cases
| Condition | Status | Body |
|-----------|--------|------|
| Archived, caller is creator | 200 | `{"status":"archived_owned","name","mode","settings"}` |
| Inactive, not creator | 410 | `{"error":"room is no longer active"}` |
| Private + not participant | 403 | private / requires approval |
| Full / banned | 403 | room full / banned |

### Guest join
- Public + active only. `guest-` + short ID identity. Guest name required, max 50, sanitized.
- Blocked if `GuestLoginEnabled` false (403).
- Res: `{id, name, token, adminId, isPublic, livekitHost, activeRecordingId}` (no full room dump).

### Refresh token
- Same private/ban/active gates as join. Returns `{token}` only. LK token TTL = `livekitTokenTTL`.

### Archived list item
```json
{"id":"...","name":"...","createdAt":"...","deletedAt":"...","recordingCount":0}
```
Default page size 20 (max 100). Count from `recordingRepo.CountByRoom`.

### UpdateSettings body
```go
{ isPublic *bool, maxParticipants *int, settings *RoomSettings }
```
- Auth: room admin (AdminID‖CreatedBy) or superadmin.
- Partial: only sent fields applied. **`isPersistent` preserved** (admin-only via AdminUpdateRoom).
- Syncs `maxParticipants` to LiveKit when changed.

### DeleteRoom
- Creator or superadmin. Enqueues `room_delete` (archive, `Purge:false` — recordings preserved).
- **202** Accepted. In-flight guard → 409.

### Chat upload
- Participant only; `allowChat` required.
- MIME via storage sniff (png/jpeg/gif/webp). Max size `chat.uploads.maxBytes`.
- Quotas: `MaxUploadBytesPerUser` / `GlobalDiskThresholdBytes` → 507.
- Sync → `ChatAttachment` `{url, mime, size, w, h}`.
- S3 + over inline threshold → **202** `{status:"upload_queued", job_type:"chat_upload_s3", size, mime}`.

### Presence
- **Unauthenticated** (rate-limited). Pre-join welcome UI.
- Private rooms: empty `participants` (or 404 if `countOnly`).
- Public: LiveKit `ListParticipants` + avatar lookup for non-guests.
- `?countOnly=1|true` → `{"count":N}` (public only).

---

## Rooms — Moderation

All `P+EV`. Path params `:roomId` + `:identity` (ask also `:action`).

| Method | Path | Handler | Authz | Action | Res |
|--------|------|---------|-------|--------|-----|
| POST | `/api/room/:roomId/kick/:identity` | `KickParticipant` | room admin / superadmin | Remove from LK + “kick” system msg | `{"status":"success"}` |
| POST | `/api/room/:roomId/ban/:identity` | `BanParticipant` | room admin / superadmin | Remove + DB ban + “ban” | `{"status":"success"}` |
| POST | `/api/room/:roomId/mute/:identity` | `MuteParticipant` | room admin / superadmin | Mute all audio tracks | `{"status":"success"}` |
| POST | `/api/room/:roomId/video/:identity/off` | `DisableParticipantVideo` | `isRoomModerator` | Mute camera | `{"status":"success"}` |
| POST | `/api/room/:roomId/screenshare/:identity/stop` | `StopScreenShare` | `isRoomModerator` | Mute screen + SS-audio | `{"status":"success"}` |
| POST | `/api/room/:roomId/promote/:identity` | `PromoteParticipant` | room admin / superadmin | LK metadata `moderator` + DB flag | `{"status":"success"}` / `already_moderator` |
| POST | `/api/room/:roomId/demote/:identity` | `DemoteParticipant` | room admin / superadmin | Remove moderator + clear DB | `{"status":"success"}` |
| POST | `/api/room/:roomId/chat/:identity/block` | `BlockChat` | `isRoomModerator` | `chatBlocked: true` metadata | `{"status":"success"}` |
| POST | `/api/room/:roomId/deafen/:identity` | `DeafenParticipant` | `isRoomModerator` | Targeted “deafen” data msg | `{"status":"success"}` |
| POST | `/api/room/:roomId/undeafen/:identity` | `UndeafenParticipant` | `isRoomModerator` | Targeted “undeafen” | `{"status":"success"}` |
| POST | `/api/room/:roomId/ask/:identity/:action` | `AskParticipantAction` | `isRoomModerator` | `:action`=`unmute`\|`camera` → ask_* | `{"status":"success"}` |
| POST | `/api/room/:roomId/spotlight/:identity` | `SpotlightParticipant` | `isRoomModerator` | Broadcast “spotlight” | `{"status":"success"}` |
| GET | `/api/room/:roomId/participant/:identity/info` | `GetParticipantInfo` | self or `isRoomModerator` | LK identity/tracks | see below |
| GET | `/api/room/:roomId/participant/:identity/profile` | `GetParticipantProfile` | caller must be **in** LK room | name + avatar | `{id, name, avatarUrl}` |
| POST | `/api/room/:roomId/stage/:identity/bring` | `BringToStage` | P+EV (no body logic) | **501 stub** | `{"error":"not yet implemented"}` |
| POST | `/api/room/:roomId/stage/:identity/remove` | `RemoveFromStage` | P+EV | **501 stub** | `{"error":"not yet implemented"}` |

### `isRoomModerator`
Superadmin **or** room admin (AdminID‖CreatedBy) **or** DB `room_participants.is_moderator` for this room. (`room_auth.go`)

Kick/ban/mute/promote/demote use **stricter** admin/superadmin only (not promoted room mods).

Common guards: no self-target (400); often cannot target room admin (403).

### Participant info
```json
{"identity":"uuid","name":"John","state":"ACTIVE","joinedAt":1234567890,"tracks":[{"sid":"TR_xxx","type":"AUDIO","source":"MICROPHONE","muted":false}]}
```

### Participant profile
- Guests (`guest-*`): LK display name, empty `avatarUrl`.
- Users: DB name/avatar with LK fallback.
- 403 if caller not currently in the LiveKit room.

---

## Recording (room-scoped) — SHIPPED handlers

> **Handlers + service are implemented** (`recording_handler.go`, `RecordingService`, tests).
> **Production routes are currently commented out** in `server/internal/server/server.go` and `cmd/server/main.go` (`// TODO oncoming feature: recording routes`). Not live until uncommented + egress client wired.
> Paths below = handler contracts / integration-test wiring.

Prefix: **`/api/rooms/:id/...`** (plural `rooms`, param `:id` = room UUID).

Intended MW (from integration tests): `P+EV` + `RecordingsEnabled(settingsRepo)` (+ APIRate on start).

| Method | Path | Handler | Authz | Req | Res | Status |
|--------|------|---------|-------|-----|-----|--------|
| POST | `/api/rooms/:id/recording/start` | `StartRecording` | `isRoomModerator` | — | `{id, status:"started", roomId}` | 201 / 403 / 404 / 409 / 500 |
| POST | `/api/rooms/:id/recording/stop` | `StopRecording` | `isRoomModerator` | — | `{id, status:"processing"}` | 200 / 403 / 404 / 500 |
| GET | `/api/rooms/:id/recordings` | `ListRecordings` | `canViewRoomRecordings` (+ creator fallback) | `?page=&limit=` | `{recordings: RecordingDTO[], total, page, limit}` | 200 / 404 |
| GET | `/api/rooms/:id/recordings/:rid` | `GetRecording` | view gate / creator | — | `RecordingDTO` | 200 / 404 |
| GET | `/api/rooms/:id/recordings/:rid/wait` | `WaitRecordingReady` | creator or can-view | long-poll ≤15s | `{status:"active"|"failed"|"timeout",...}` | 200 / 408 / 404 |
| DELETE | `/api/rooms/:id/recordings` | `ClearRoomRecordings` | room admin / superadmin | — | `{message}` or 207 partial | 200 / 207 / 403 / 404 |
| DELETE | `/api/rooms/:id/recordings/:recordingId` | `ClearSingleRecording` | room admin / superadmin | — | `{message:"Recording cleared"}` | 200 / 403 / 404 |

Admin-global recording list/bulk-delete live under `/api/admin/recordings` (also commented) → `bedrud-api-admin`.

### Auth gates (3 layers)
1. **System:** `middleware.RecordingsEnabled` → `SystemSettings.RecordingsEnabled` (403 if off)
2. **Room:** `RecordingService.gateRoom` → `Room.Settings.RecordingsAllowed`
3. **User:** start/stop → `isRoomModerator`; list/get → `canViewRoomRecordings` (superadmin / owner / active participant; archived → owner+superadmin only; denied as **404** to avoid leaks)

### Recording status lifecycle
`pending` → `started` → `processing` → `completed` | `failed` (+ `deleting`)

### RecordingDTO
```json
{
  "id": "uuid",
  "recordingType": "composite",
  "durationMs": 0,
  "fileSize": 0,
  "fileUrl": "...",
  "status": "completed",
  "error": "",
  "downloadStatus": "ready",
  "roomId": "uuid",
  "roomName": "room",
  "createdBy": "user-uuid",
  "createdAt": "2025-03-15T10:30:00Z"
}
```
`downloadStatus`: `failed` | `processing` | `ready` (computed).

### Wait endpoint
Polls every 500ms up to 15s. Returns `active` once started/processing/completed; `failed` with error; **408** timeout.

---

## RoomSettings model

```go
type RoomSettings struct {
    AllowChat         bool // default true
    AllowVideo        bool // default true
    AllowAudio        bool // default true
    RequireApproval   bool // default false
    E2EE              bool // default false
    IsPersistent      bool // superadmin-only via AdminUpdateRoom
    RecordingsAllowed bool // create forces true; gate for recording service
}
```

---

## Source files

| Concern | Path |
|---------|------|
| Route registration (prod) | `server/internal/server/server.go` |
| Route registration (dev Air) | `server/cmd/server/main.go` |
| Room handlers | `server/internal/handlers/room.go` |
| Moderator helper | `server/internal/handlers/room_auth.go` |
| Recording handlers | `server/internal/handlers/recording_handler.go` |
| Recording service | `server/internal/services/recording_service.go` |
| RecordingsEnabled MW | `server/internal/middleware/recordings_enabled.go` |
| Models | `server/internal/models/room.go`, `recording.go` |
| Chat upload | `server/internal/storage/chat_upload.go` |
| Room tests | `room_*_test.go`, `recording_handler_test.go` |

---

## Out of scope here

Admin room list/close/suspend/bulk, admin participant kick/mute, `/api/admin/online-count`, admin recordings → **`bedrud-api-admin`**.
LiveKit webhook (`/api/livekit/webhook`) → **`bedrud-realtime`** / server skill.
