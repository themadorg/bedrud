# Room Lifecycle

How rooms are created, joined, moderated, suspended, archived, and deleted — including LiveKit integration.

**Primary handler:** `server/internal/handlers/room.go`  
**Cleanup service:** `server/internal/services/room_cleanup.go`  
**Webhook handler:** `server/internal/handlers/livekit_webhook.go`

**Public docs:** [API Handlers](https://bedrud.org/en/docs/backend/api-handlers) · [LiveKit Integration](https://bedrud.org/en/docs/backend/livekit) · [WebRTC Connectivity](https://bedrud.org/en/docs/architecture/webrtc-connectivity) — [`backend/api-handlers.mdx`](../../apps/site/src/content/docs/en/backend/api-handlers.mdx). Full map: [public-docs.md](./public-docs.md).

---

## Room states

```
                    ┌──────────────┐
                    │   created    │
                    │ is_active=T  │
                    │ deleted_at=N │
                    └──────┬───────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
           ▼               ▼               ▼
    ┌────────────┐  ┌────────────┐  ┌────────────┐
    │  active    │  │ suspended  │  │  archived  │
    │ participants│  │ is_active=F│  │ deleted_at │
    │ in LK room │  │ LK deleted │  │ set        │
    └────────────┘  └────────────┘  └────────────┘
           │               │               │
           └───────────────┴───────────────┘
                           │
                    cascade delete
                    (queue job)
```

| State | DB signals | LiveKit |
|-------|------------|---------|
| **Active** | `is_active=true`, `deleted_at=NULL` | Room exists, participants connected |
| **Suspended** | `is_active=false` | `DeleteRoom` called; DB row kept |
| **Archived** | `deleted_at` set | LK room removed |
| **Deleted** | Row removed | LK + uploads + participants cleaned |

---

## Create room (`POST /api/room/create`)

```
Authenticated user (email verified)
  ├─ Validate room name (ValidateRoomName) or generate random name
  ├─ Check maxRoomsPerUser quota (settings)
  ├─ Check name availability (active rooms only)
  ├─ Insert room row (admin_id = creator)
  ├─ Create LiveKit room via lkClient.CreateRoom
  ├─ Add creator as room_participant (is_moderator implicit via ownership)
  ├─ Mint LiveKit JWT with VideoGrant
  └─ Return room metadata + token + ws URL
```

**Settings embedded in room:** `allowChat`, `allowVideo`, `allowAudio`, `requireApproval`, `e2ee`, `isPersistent`.

---

## Join flows

### Authenticated join (`POST /api/room/join`)

1. Resolve room by ID or name
2. Check room active, not archived, not banned
3. Enforce `requireApproval` → participant `is_approved=false` until moderator approves
4. Upsert `room_participants` row
5. Update `last_activity_at`
6. Mint LiveKit token with appropriate grants
7. Optionally dispatch outbound webhook (`room.participant_joined`)

### Guest join (`POST /api/room/guest-join`)

Same as join but uses guest JWT from prior `guest-login`. Rate limited separately.

### Token refresh (`POST /api/room/refresh-token`)

Re-mints LiveKit JWT when the short-lived token expires without leaving the room. Requires room membership.

---

## Moderation actions

All room moderation routes require `Protected()` + `RequireEmailVerified()` + `isRoomModerator()`.

| Action | Route pattern | LiveKit + DB |
|--------|---------------|--------------|
| Kick | `POST /room/:roomId/kick/:identity` | `RemoveParticipant` + system message |
| Mute | `POST /room/:roomId/mute/:identity` | `MutePublishedTrack` |
| Ban | `POST /room/:roomId/ban/:identity` | Remove + set `is_banned` |
| Video off | `POST /room/:roomId/video/:identity/off` | Unpublish video tracks |
| Promote/Demote | `.../promote/:identity`, `.../demote/:identity` | Set `is_moderator` |
| Block chat | `.../chat/:identity/block` | Set `is_chat_blocked` + data message |
| Deafen/Undeafen | `.../deafen/:identity` | Data channel signal |
| Spotlight | `.../spotlight/:identity` | Data channel signal |
| Stop screenshare | `.../screenshare/:identity/stop` | Unpublish screen track |
| Stage bring/remove | `.../stage/:identity/bring` | Set `is_on_stage` |
| Ask action | `.../ask/:identity/:action` | Prompt user (unmute, etc.) |

**System messages:** `sendSystemMessage()` / `sendTargetedSystemMessage()` publish to LiveKit data channel so all clients update UI state.

**Authorization order:**

1. Superadmin → always allowed
2. Room owner (`admin_id` / `created_by`) → allowed
3. Room moderator (`is_moderator` in `room_participants`) → allowed
4. Otherwise → 403

---

## Room settings (`PUT /api/room/:roomId/settings`)

Room owner or moderator updates embedded `RoomSettings`. Changes apply to new joins; active participants receive system messages for some toggles.

---

## User-initiated delete (`DELETE /api/room/:roomId`)

Owner-only. Enqueues `room_delete` job → returns **202 Accepted**.

Does not block on LiveKit or file cleanup.

---

## Admin room operations

| Endpoint | Effect |
|----------|--------|
| `GET /admin/rooms` | Paginated list with filters |
| `GET /admin/rooms/events` | Activity event log |
| `POST /admin/rooms/:roomId/token` | Admin join token |
| `DELETE /admin/rooms/:roomId` | Close room (sync or async) |
| `POST /admin/rooms/:roomId/suspend` | Suspend → `room_suspend` job |
| `POST /admin/rooms/:roomId/reactivate` | Re-enable suspended room |
| `PUT /admin/rooms/:roomId` | Update metadata/settings |
| `POST /admin/rooms/bulk-suspend` | Bulk suspend (queue) |
| `POST /admin/rooms/bulk-close` | Bulk close (queue) |
| `GET /admin/online-count` | Aggregate LiveKit participant count |
| `GET /admin/livekit/stats` | LiveKit server stats |
| `GET /admin/rooms/:roomId/participants` | LiveKit + DB participant merge |

Admin routes require `superadmin` access.

---

## Cascade delete (`RoomCleanupService.CascadeDeleteRoom`)

Called by `room_delete` and `user_delete` queue handlers.

```
1. Stop active recordings (when wired)
2. Send system message to room (optional)
3. lkClient.DeleteRoom
4. uploadTracker.DeleteByRoom (disk files + S3 + DB rows)
5. Delete room_participants, room_permissions
6. Delete or archive room row (depending on caller)
```

Failures on LiveKit delete are logged but DB cleanup proceeds.

---

## Scheduler idle cleanup

`internal/scheduler/` runs periodic jobs:

- Detect rooms with stale `last_activity_at`
- Suspend or delete per config thresholds
- Purge old completed/failed queue jobs

See [internal/scheduler.md](./internal/scheduler.md).

---

## LiveKit webhook (`POST /api/livekit/webhook`)

Validates webhook signature with LiveKit API secret.

| Event | Handler behavior |
|-------|------------------|
| `participant_left` | Update participant `left_at`, decrement activity |
| `room_finished` | Mark room inactive, update timestamps |
| `egress_started` | Recording state (planned) |
| `egress_ended` | Finalize recording metadata (planned) |

Webhook URL auto-configured for embedded LiveKit (`http://localhost:<httpPort>/api/livekit/webhook`). External LiveKit requires manual dashboard configuration.

---

## Chat image uploads

`POST /api/room/:roomId/chat/upload`:

1. Validate size against settings quota (per-user + global disk threshold)
2. Store via `ChatUploadStore` (disk, inline base64, or S3)
3. Track in `chat_uploads` table
4. S3 uploads may enqueue `chat_upload_s3` for async transfer

Serving: `GET /uploads/chat/*` — JWT protected, path traversal prevented.

---

## Outbound webhooks

Room events (`room.created`, `room.deleted`, `participant.joined`, etc.) enqueue `dispatch_webhook` jobs when webhooks are configured in admin settings.

See [email-webhooks.md](./email-webhooks.md).

---

## List endpoints

| Endpoint | Returns |
|----------|---------|
| `GET /api/room/list` | User's active rooms |
| `GET /api/room/archived` | User's archived (soft-deleted) rooms |

---

## Related docs

- [handlers-reference.md](./handlers-reference.md) — all `RoomHandler` methods
- [queue-deep-dive.md](./queue-deep-dive.md) — `room_delete`, `room_suspend` jobs
- [database-schema.md](./database-schema.md) — room tables and indexes
- [internal/livekit.md](./internal/livekit.md) — embedded server and TLS