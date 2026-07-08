# Backend Patterns

Conventions, error handling, authorization recipes, and testing patterns for Bedrud Go backend contributors.

**Public docs:** [Development Workflow](https://bedrud.org/en/docs/guides/development) · [Contributing](https://bedrud.org/en/docs/contributing) · [Advanced Topics](https://bedrud.org/en/docs/backend/advanced). Full map: [public-docs.md](./public-docs.md).

---

## Layer responsibilities

| Layer | Does | Does not |
|-------|------|----------|
| **Handler** | HTTP parse/validate, status codes, enqueue jobs | Raw SQL, business logic duplication |
| **Repository** | GORM queries, transactions | HTTP concerns, LiveKit calls |
| **Service** | Cross-entity orchestration (cleanup) | HTTP request parsing |
| **Model** | Schema, validation helpers | External API calls |
| **Queue handler** | Slow/idempotent side effects | Return HTTP responses |

---

## Error responses

Standard JSON shape:

```go
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
    "error": "Human-readable message",
})
```

| Status | When |
|--------|------|
| 400 | Validation failure, bad input |
| 401 | Missing/invalid JWT |
| 403 | Banned, insufficient access, not room moderator |
| 404 | Entity not found |
| 409 | Conflict (duplicate email, room name taken) |
| 422 | Semantic validation (rare) |
| 202 | Async job enqueued |
| 500 | Unexpected server error (log with zerolog) |
| 503 | Not ready (database down) |

Log internal details; return safe messages to clients.

```go
log.Error().Err(err).Str("roomId", id).Msg("Failed to delete room")
return c.Status(500).JSON(fiber.Map{"error": "Failed to delete room"})
```

---

## Logging

Use structured zerolog:

```go
import "github.com/rs/zerolog/log"

log.Info().Str("userId", id).Str("room", name).Msg("Room created")
log.Warn().Err(err).Str("jobID", job.ID).Msg("queue: retry scheduled")
log.Debug().Str("path", path).Msg("Proxying LiveKit request")
```

Never log passwords, tokens, or webhook secrets.

---

## Authorization patterns

### Global admin

```go
// Route registration — not in handler
adminGroup := api.Group("/admin",
    middleware.Protected(),
    middleware.RequireEmailVerified(cfg, userRepo),
    middleware.RequireAccess(models.AccessSuperAdmin),
)
```

### Authenticated user (self)

```go
claims := c.Locals("user").(*auth.Claims)
if claims.UserID != targetUserID && !containsAccess(claims.Accesses, "superadmin") {
    return c.Status(403).JSON(fiber.Map{"error": "Forbidden"})
}
```

### Room moderator

```go
room, roomName, err := h.resolveRoom(c, roomID)
if err != nil { ... }

ownerID := room.AdminID
if ownerID == "" { ownerID = room.CreatedBy }

if !isRoomModerator(claims, ownerID, room.ID, h.roomRepo) {
    return c.Status(403).JSON(fiber.Map{"error": "Not authorized to moderate this room"})
}
```

### Email verification gate

Most user routes use `middleware.RequireEmailVerified(cfg, userRepo)` at registration time — don't duplicate the check in handlers unless there's an exception path.

---

## Async job pattern

```go
// In handler — enqueue and return immediately
if err := queue.Enqueue(c.Context(), database.GetDB(), "room_delete", map[string]string{
    "roomId": room.ID,
}); err != nil {
    if strings.Contains(err.Error(), "queue depth limit") {
        return c.Status(503).JSON(fiber.Map{"error": "Server busy, try again later"})
    }
    log.Error().Err(err).Msg("Failed to enqueue room_delete")
    return c.Status(500).JSON(fiber.Map{"error": "Failed to schedule deletion"})
}
return c.Status(202).JSON(fiber.Map{"message": "Room deletion scheduled"})
```

Queue handlers must tolerate duplicate delivery — use idempotent DB operations.

---

## LiveKit calls

Always use authenticated context:

```go
ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: roomName})
_, err := h.lkClient.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{
    Room: roomName, Identity: identity,
})
```

Shared helpers in `internal/lkutil/` — prefer `lkutil.NewClient`, `lkutil.AuthContext`, `lkutil.SendSystemMessage`.

---

## Settings access

```go
settings, err := h.settingsRepo.GetEffectiveSettings()
if err != nil { ... }

if !settings.RegistrationEnabled {
    return c.Status(403).JSON(fiber.Map{"error": "Registration is disabled"})
}
```

Use `GetEffectiveSettings()` in handlers — not raw `GetSettings()` — so config.yaml defaults apply.

---

## Repository transactions

For multi-table updates:

```go
err := r.db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&room).Error; err != nil { return err }
    if err := tx.Create(&participant).Error; err != nil { return err }
    return nil
})
```

Keep transactions in repository methods, not handlers.

---

## Testing patterns

### Test database

```go
import "bedrud/internal/testutil"

db := testutil.SetupTestDB(t)
defer testutil.TeardownTestDB(t, db)
```

SQLite in-memory for unit tests. See [testing.md](./testing.md).

### Handler tests

- Use Fiber `app.Test(req)` with httptest
- Mock repositories via interfaces where tests exist
- Queue handler tests insert `models.Job` directly and call handler func

### Table-driven tests

```go
tests := []struct {
    name       string
    input      string
    wantStatus int
}{
    {"valid name", "my-room", 201},
    {"too short", "ab", 400},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

### Commands

```bash
cd server && go test -v -count=1 ./...
cd server && go vet ./...
cd server && go build ./...
```

---

## Swagger annotations

Public API handlers should include swag comments:

```go
// CreateRoom godoc
// @Summary      Create a new room
// @Tags         rooms
// @Security     BearerAuth
// @Success      201  {object}  CreateRoomResponse
// @Router       /room/create [post]
func (h *RoomHandler) CreateRoom(c *fiber.Ctx) error {
```

Regenerate: `make swagger-gen` from repo root.

---

## Common pitfalls

| Pitfall | Fix |
|---------|-----|
| Route only in `server.go` | Also add to `cmd/server/main.go` |
| Forgetting `RequireEmailVerified` | Match neighboring routes in same group |
| Sync delete of large resources | Enqueue `room_delete` / `user_delete` |
| Using `config.Get()` for runtime toggles | Use `settingsRepo.GetEffectiveSettings()` |
| SQLite concurrent writes | Keep `SetMaxOpenConns(1)`; avoid long transactions |
| Rate limit behind proxy | Set `behindProxy: true` |
| CORS credentials + wildcard | Explicit origins required — server refuses to start |
| Room name uniqueness | Check active rooms only; partial index enforces at DB level |

---

## File placement guide

| Adding… | Location |
|---------|----------|
| New REST endpoint | `internal/handlers/`, register in `server.go` |
| New table | `internal/models/`, migrate in `database/migrations.go` |
| New query | `internal/repository/` |
| Cross-cutting logic | `internal/services/` |
| Background job | `internal/queue/handler_*.go` |
| Middleware | `internal/middleware/` |
| Config field | `config/config.go` + `configuration.md` |

Step-by-step recipes: [developer-guide.md](./developer-guide.md)

---

## Related docs

- [developer-guide.md](./developer-guide.md) — add endpoint/model/queue job
- [architecture.md](./architecture.md) — request lifecycle
- [handlers-reference.md](./handlers-reference.md) — method index
- [testing.md](./testing.md) — CI and test layout
- [docs/developer/server-development.md](../developer/server-development.md) — contributor workflow