# LiveKit Helpers (`internal/lkutil`)

Shared LiveKit client utilities used by handlers, services, and CLI tools.

---

## Exports

### `NewClient(lkCfg *config.LiveKitConfig) livekit.RoomService`

Creates a LiveKit RoomService protobuf client.

- Respects `InternalHost` / `Host` from config
- Handles `SkipTLSVerify` for self-signed certs

### `AuthContext(ctx, apiKey, apiSecret, grants...) context.Context`

Injects Bearer token into twirp context for LiveKit API calls.

```go
ctx = lkutil.AuthContext(ctx, apiKey, apiSecret, &lkauth.VideoGrant{
    RoomJoin: true,
    Room:     roomName,
})
```

### `SendSystemMessage(ctx, client, roomName, event, message)`

Sends a typed system data message over the LiveKit data channel:

- Topic: `"system"`
- Kind: `RELIABLE`
- Used for kick, ban, deafen, spotlight, room-ended notifications

---

## Consumers

| Package | Usage |
|---------|-------|
| `handlers/room.go` | Room creation, moderation, system messages |
| `handlers/users.go` | User deletion room cleanup |
| `services/room_cleanup.go` | Cascade delete, suspend |
| `services/recording_service.go` | Egress start/stop |
| `usercli/usercli.go` | CLI user delete room cleanup |
| `roomcli/roomcli.go` | CLI room management |