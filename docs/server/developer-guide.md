# Server Developer Guide

Step-by-step recipes for extending the Bedrud Go backend.

**Public docs:** [Development Workflow](https://bedrud.org/en/docs/guides/development) · [Contributing](https://bedrud.org/en/docs/contributing). Internal contributor guide: [docs/developer/](../developer/). Full map: [public-docs.md](./public-docs.md).

---

## Add an API endpoint

### 1. Define the handler

Create or extend a file in `internal/handlers/`. Example: add `GetRoomStats` to `room.go`:

```go
func (h *RoomHandler) GetRoomStats(c *fiber.Ctx) error {
    roomID := c.Params("roomId")
    room, err := h.roomRepo.GetRoom(roomID)
    if err != nil || room == nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Room not found"})
    }
    // authorization check via room_auth helpers or claims
    return c.JSON(fiber.Map{"participants": 0})
}
```

### 2. Add repository methods (if needed)

In `internal/repository/room_repository.go`:

```go
func (r *RoomRepository) GetRoomStats(roomID string) (int, error) {
    // GORM query here
}
```

### 3. Register the route

Add to **both** `internal/server/server.go` and `cmd/server/main.go`:

```go
api.Get("/room/:roomId/stats",
    middleware.Protected(),
    middleware.RequireEmailVerified(cfg, userRepo),
    roomHandler.GetRoomStats,
)
```

### 4. Choose middleware

| Need | Middleware |
|------|------------|
| Logged-in user | `Protected()` |
| Email verified (when enabled) | `RequireEmailVerified(cfg, userRepo)` |
| Superadmin only | `RequireAccess(models.AccessSuperAdmin)` |
| Rate limit | `AuthRateLimiter`, `APIRateLimiter`, or `GuestRateLimiter` |

Admin routes go in the `adminGroup` block.

### 5. Add Swagger annotation (optional)

```go
// @Summary Get room stats
// @Tags rooms
// @Security BearerAuth
// @Param roomId path string true "Room ID"
// @Success 200 {object} map[string]interface{}
// @Router /room/{roomId}/stats [get]
```

Run `make swagger-gen`.

### 6. Wire frontend (if applicable)

Add `authFetch` call in `apps/web/src/lib/api.ts`. Add types to `packages/api-types/` if shared across clients.

### 7. Test

```go
func TestGetRoomStats(t *testing.T) {
    db, cleanup := testutil.SetupTestDB()
    defer cleanup()
    // ...
}
```

---

## Add a database model

### 1. Create model struct

`internal/models/my_feature.go`:

```go
package models

import "time"

type MyFeature struct {
    ID        string    `gorm:"type:varchar(36);primaryKey" json:"id"`
    Name      string    `gorm:"not null" json:"name"`
    CreatedAt time.Time `json:"createdAt"`
}
```

### 2. Register in migrations

`internal/database/migrations.go` — add to `AutoMigrate` list:

```go
&models.MyFeature{},
```

### 3. Create repository

`internal/repository/my_feature_repository.go`:

```go
type MyFeatureRepository struct { *gorm.DB }

func NewMyFeatureRepository(db *gorm.DB) *MyFeatureRepository {
    return &MyFeatureRepository{db}
}
```

### 4. Run migration

Restart server or:

```bash
go run ./cmd/bedrud db migrate
```

---

## Add a queue job

For async work (deletes, emails, webhooks). Handlers return `202 Accepted`.

### 1. Define payload

`internal/queue/job.go`:

```go
type MyJobPayload struct {
    EntityID string `json:"entity_id"`
}
```

### 2. Create handler

`internal/queue/handler_my_job.go`:

```go
func NewMyJobHandler(deps ...) queue.Handler {
    return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
        var payload MyJobPayload
        if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
            return err
        }
        // do work
        return nil
    }
}
```

### 3. Register in worker map

In `internal/server/server.go`:

```go
queueWorker := queue.NewWorker(database.GetDB(), map[string]queue.Handler{
    // existing handlers...
    "my_job": queue.NewMyJobHandler(...),
}, ...)
```

### 4. Enqueue from HTTP handler

```go
import "bedrud/internal/queue"

payload := queue.MyJobPayload{EntityID: id}
if err := queue.Enqueue(c.Context(), db, "my_job", payload); err != nil {
    return c.Status(500).JSON(fiber.Map{"error": "Failed to enqueue"})
}
return c.Status(202).JSON(fiber.Map{"message": "Job started"})
```

### 5. Test

See `internal/queue/queue_test.go` for patterns.

---

## Extend SystemSettings

Admin panel settings live in `models/settings.go` as the `SystemSettings` struct.

1. Add field with GORM tags + `json` tag
2. Update `admin_handler.go` `GetSettings` / `UpdateSettings` if masking needed
3. Read from `settingsRepo.GetSettings()` in handlers
4. GORM auto-migrates new columns on startup

Secrets returned as `"******"` in GET responses.

---

## Add a CLI command

1. Create handler in `internal/cli/my_cmd.go`:

```go
func newMyCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "my-cmd",
        Short: "Does something",
        RunE: func(cmd *cobra.Command, args []string) error {
            return mycli.DoSomething(resolveConfigPath("/etc/bedrud/config.yaml"))
        },
    }
}
```

2. Register in `internal/cli/root.go`:

```go
root.AddCommand(newMyCmd())
```

3. Put business logic in `internal/mycli/` if non-trivial (follow `usercli` pattern).

---

## Add email template

1. Add HTML + text in `internal/queue/templates/` (use Cerberus hybrid layout)
2. Reference template name in `SendEmailPayload.TemplateName`
3. `handler_email.go` renders with branding from config/DB

---

## Add outbound webhook event

1. Create `Webhook` row via admin API or seed
2. From handler, enqueue `dispatch_webhook`:

```go
queue.Enqueue(ctx, db, "dispatch_webhook", queue.WebhookPayload{
    URL:    webhook.URL,
    Event:  "room.created",
    Body:   map[string]any{"roomId": room.ID},
    Secret: webhook.Secret,
})
```

---

## Debug checklist

| Symptom | Check |
|---------|-------|
| 401 on all routes | JWT secret mismatch, expired token |
| 403 email not verified | `requireEmailVerification` + `EmailVerifiedAt` nil |
| 404 on new route | Registered in both `server.go` and `cmd/server/main.go`? |
| Queue jobs stuck pending | `QUEUE_POLL_INTERVAL`, SQLite write contention |
| LiveKit connection fail | `livekit.host` URL, embedded LK running, `/livekit` proxy |
| CORS error from web dev | `cors.allowedOrigins` includes `http://localhost:3000` |

More: [Debugging Guide](../developer/debugging.md).