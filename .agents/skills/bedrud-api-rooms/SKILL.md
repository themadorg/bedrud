---
name: bedrud-api-rooms
description: Room endpoints — CRUD, join, moderation, online count.
license: Apache License
---

# Bedrud API — Room Endpoints

---

## Rooms — CRUD + Join

| Method | Path | Auth | Handler | Req | Res | Status |
|--------|------|------|---------|-----|-----|--------|
| POST | `/api/room/create` | Protected | `CreateRoom` | `CreateRoomRequest` | inline room + livekitHost | 201 / 409 |
| POST | `/api/room/join` | Protected | `JoinRoom` | `JoinRoomRequest` | inline room + token + livekitHost | 200 / 404 |
| POST | `/api/room/guest-join` | GuestRate | `GuestJoinRoom` | `GuestJoinRoomRequest` | `{id, name, token, adminId, livekitHost}` | 200 / 404 |
| GET | `/api/room/list` | Protected | `ListRooms` | — | `[]models.Room` | 200 |
| DELETE | `/api/room/:roomId` | Protected | `DeleteRoom` | — | `{"status":"success"}` | 200 / 403 / 404 |
| PUT | `/api/room/:roomId/settings` | Protected | `UpdateSettings` | `{isPublic *bool, maxParticipants *int, settings *RoomSettings}` | `models.Room` | 200 / 403 / 404 |
| POST | `/api/room/:roomId/chat/upload` | Protected | `UploadChatImage` | multipart `{file}` | `ChatAttachment` | 200 / 400 / 413 |

### CreateRoomRequest
```go
{ name string, maxParticipants int, isPublic bool, mode string, settings RoomSettings }
```

### Create Response
```json
{"id":"uuid","name":"xxx-xxxx-xxx","createdBy":"uuid","isActive":true,"isPublic":false,"maxParticipants":20,"settings":{},"livekitHost":"ws://...","mode":"standard"}
```

### Join Response
```json
{"id":"uuid","name":"room-name","token":"lk-jwt","createdBy":"uuid","adminId":"uuid","isActive":true,"maxParticipants":20,"expiresAt":"...","settings":{},"livekitHost":"ws://...","mode":"standard"}
```

### Notes
- Create: auto-gen name if empty. 409 on conflict. Strips `isPersistent` (superadmin-only via AdminUpdateRoom). Creator auto-added as approved participant. 24h expiry.
- Join: lookup by name. Generates LK token. Rejects banned.
- Guest join: public rooms only. Restricted LK token. `guest-` prefixed identity.
- Delete: creator or superadmin only.
- Settings: partial update via pointer fields. Preserves `isPersistent`.
- Chat upload: MIME png/jpeg/gif/webp. SHA256 content hash. Max size from config.

---

## Rooms — Moderation

All `Protected()`. Path params `:roomId` + `:identity`.

| Method | Path | Action | Res |
|--------|------|--------|-----|
| POST | `/api/room/:roomId/kick/:identity` | Remove from LK + broadcast "kick" | `{"status":"success"}` |
| POST | `/api/room/:roomId/ban/:identity` | Remove from LK + DB banned + broadcast "ban" | `{"status":"success"}` |
| POST | `/api/room/:roomId/mute/:identity` | Mute all audio tracks | `{"status":"success"}` |
| POST | `/api/room/:roomId/video/:identity/off` | Mute camera track | `{"status":"success"}` |
| POST | `/api/room/:roomId/screenshare/:identity/stop` | Mute screen-share tracks | `{"status":"success"}` |
| POST | `/api/room/:roomId/promote/:identity` | Add "moderator" to LK metadata | `{"status":"success"}` |
| POST | `/api/room/:roomId/demote/:identity` | Remove "moderator" | `{"status":"success"}` |
| POST | `/api/room/:roomId/chat/:identity/block` | Set `chatBlocked: true` | `{"status":"success"}` |
| POST | `/api/room/:roomId/deafen/:identity` | Send "deafen" data msg | `{"status":"success"}` |
| POST | `/api/room/:roomId/undeafen/:identity` | Send "undeafen" data msg | `{"status":"success"}` |
| POST | `/api/room/:roomId/ask/:identity/:action` | ask_unmute / ask_camera | `{"status":"success"}` |
| POST | `/api/room/:roomId/spotlight/:identity` | Broadcast "spotlight" | `{"status":"success"}` |
| GET | `/api/room/:roomId/participant/:identity/info` | Identity, name, state, tracks | inline obj |
| POST | `/api/room/:roomId/stage/:identity/bring` | 501 stub | 501 |
| POST | `/api/room/:roomId/stage/:identity/remove` | 501 stub | 501 |

## Recording Endpoints

> **TODO oncoming feature:** Recording functionality is planned for a future release.

Routes under `/api/rooms/:id/recording/` (note plural `rooms` vs singular `room` above). All use `RecordingsEnabled` middleware + `Protected`.

| Method | Path | Handler | Req | Res | Status |
|--------|------|---------|-----|-----|--------|
| POST | `/api/rooms/:id/recording/start` | `StartRecording` | — | `{id, status, roomId}` | 201 / 403 / 409 |
| POST | `/api/rooms/:id/recording/stop` | `StopRecording` | — | `{id, status}` | 200 / 403 / 404 |
| GET | `/api/rooms/:id/recordings` | `ListRecordings` | `?page=&limit=` | `{recordings[], total, page, limit}` | 200 |
| GET | `/api/rooms/:id/recordings/:rid` | `GetRecording` | — | `Recording` | 200 / 404 |

### Authorization (3 layers)
1. System: `middleware.RecordingsEnabled()` → checks `SystemSettings.RecordingsEnabled`
2. Room: `RecordingService.gateRoom()` → checks `Room.Settings.RecordingsAllowed`
3. User: `isRoomModerator()` in handler → only moderators/creator/admin

### Recording Status
`pending` → `started` → `processing` → `completed` | `failed`

### Recording Response
```json
{
  "id": "uuid",
  "roomId": "uuid",
  "roomName": "room",
  "egressId": "lk-egress-id",
  "status": "completed",
  "fileUrl": "...",
  "durationMs": 300000,
  "createdBy": "user-uuid",
  "createdAt": "2025-03-15T10:30:00Z"
}
```

### Participant Info Response
```json
{"identity":"uuid","name":"John","state":"ACTIVE","joinedAt":"...","tracks":[{"sid":"TR_xxx","type":"AUDIO","source":"MICROPHONE","muted":false}]}
```

### Authorization
- Room actions: creator, room admin, or superadmin/admin.
- Self-info: participant can view own. Admin/mod can view any.

---

## Online Count

| Method | Path | Auth | Res |
|--------|------|------|-----|
| GET | `/api/room/online-count` | Protected | `{"count": <int>}` |
