# Services (`internal/services`)

Cross-cutting business logic used by handlers, queue workers, and CLI tools.

---

## RoomCleanupService (`room_cleanup.go`)

```go
type RoomCleanupService struct {
    roomRepo      *repository.RoomRepository
    client        livekit.RoomService
    apiKey        string
    apiSecret     string
    uploadTracker *storage.ChatUploadTracker
}
```

### Methods

| Method | Purpose |
|--------|---------|
| `CascadeDeleteRoom(ctx, room, reason, deletedIdentity)` | Close LK room → broadcast end message → `AdminDeleteRoom` → chat upload tracker cleanup |
| `SuspendRoom(ctx, room)` | Close LK room → mark room inactive in DB |
| `DeleteUserRooms(ctx, user, rooms)` | Iterate rooms, `CascadeDeleteRoom` each |

### Used by

- `queue/handler_room_delete.go`
- `queue/handler_room_suspend.go`
- `queue/handler_user_delete.go`
- `usercli/usercli.go`
- `roomcli/roomcli.go`

---

## RecordingService (`recording_service.go`)

```go
type RecordingService struct {
    recordingRepo  *repository.RecordingRepository
    roomRepo       *repository.RoomRepository
    recordingStore storage.RecordingStore
    client         livekit.RoomService  // egress client
    config         *config.RecordingConfig
}
```

### Methods

| Method | Purpose |
|--------|---------|
| `StartRecording(ctx, roomID, userID, type)` | Start LiveKit egress, create DB record |
| `StopRecording(ctx, roomID, recordingID)` | Stop egress |
| `ListRecordings(roomID)` | List room recordings |
| `GetDownloadPath(recordingID)` | Resolve file path for download |
| `DeleteRecording(recordingID)` | Delete file + DB row |
| `ProcessEgress(ctx, payload)` | Handle egress completion webhook |
| `CleanupExpired()` | Delete recordings past retention period |

### Recording types

`audio`, `video`, `screen`, `composite`

### Storage

Files stored in `recording.storageDir` (default `./data/recordings`). Max file size from `recording.maxFileSizeMB` (default 2048).