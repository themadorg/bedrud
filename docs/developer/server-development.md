# Server Development

Guide for working on the Go backend. For architecture reference, see [docs/server/](../server/). For step-by-step recipes, see [Server Developer Guide](../server/developer-guide.md).

**Public docs (bedrud.org):** [Backend Documentation](https://bedrud.org/en/docs/backend/overview) — deployment, config, operator guides. Mapped to internal docs in [public-docs.md](../server/public-docs.md). Sources: [`apps/site/src/content/docs/en/`](../../apps/site/src/content/docs/en/).

---

## Entry points

| Binary | Path | When to use |
|--------|------|-------------|
| Dev API | `cmd/server/main.go` | Daily dev, Air hot-reload, Swagger |
| Production CLI | `cmd/bedrud/main.go` | Production, `bedrud install`, CLI admin |

Both wire the same handlers; dev entrypoint inlines bootstrap, production uses `internal/server.Run()`.

**Register new routes in both** `internal/server/server.go` and `cmd/server/main.go` until route registration is consolidated.

---

## Layered architecture

```
HTTP Request
    ↓
Middleware (auth, rate limit, email verified)
    ↓
Handler (internal/handlers/)     ← business logic, HTTP status codes
    ↓
Repository (internal/repository/) ← GORM queries only
    ↓
Model (internal/models/)          ← table schemas
    ↓
Database (SQLite / PostgreSQL)
```

Async side effects go through the **queue** (`internal/queue/`) — return `202 Accepted` from handlers.

Cross-cutting logic lives in **services** (`internal/services/`) — cleanup, recordings.

---

## Project conventions

### Logging

```go
import "github.com/rs/zerolog/log"

log.Info().Str("roomId", id).Msg("Room created")
log.Error().Err(err).Msg("Failed to delete room")
```

### Auth context in handlers

```go
claims := c.Locals("user").(*auth.Claims)
userID := claims.UserID
```

### Error responses

```go
return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
    "error": "Room name is required",
})
```

### Repository pattern

```go
// ✅ Handler calls repository
room, err := h.roomRepo.GetRoom(id)

// ❌ Handler queries DB directly
database.GetDB().Where("id = ?", id).First(&room)
```

---

## Common tasks (quick links)

| Task | Guide section |
|------|---------------|
| Add REST endpoint | [developer-guide.md § Add an API endpoint](../server/developer-guide.md#add-an-api-endpoint) |
| Add database model | [developer-guide.md § Add a model](../server/developer-guide.md#add-a-database-model) |
| Add async job | [developer-guide.md § Add a queue job](../server/developer-guide.md#add-a-queue-job) |
| Add admin setting | [developer-guide.md § Extend SystemSettings](../server/developer-guide.md#extend-systemsettings) |
| Add CLI command | [developer-guide.md § Add a CLI command](../server/developer-guide.md#add-a-cli-command) |

---

## Backend deep dives

Read these when you need to understand *how* the backend works, not just where files live:

| Topic | Document |
|-------|----------|
| Startup and request flow | [architecture.md](../server/architecture.md) |
| Tables and migrations | [database-schema.md](../server/database-schema.md) |
| Login, JWT, passkeys, OAuth | [auth-flows.md](../server/auth-flows.md) |
| Rooms and LiveKit | [room-lifecycle.md](../server/room-lifecycle.md) |
| Every handler method | [handlers-reference.md](../server/handlers-reference.md) |
| Async job queue | [queue-deep-dive.md](../server/queue-deep-dive.md) |
| Email and webhooks | [email-webhooks.md](../server/email-webhooks.md) |
| config.yaml vs DB settings | [settings-system.md](../server/settings-system.md) |
| Code conventions | [backend-patterns.md](../server/backend-patterns.md) |
| Public ↔ internal map | [public-docs.md](../server/public-docs.md) |

### Public site docs (operators)

| Topic | bedrud.org | MDX source |
|-------|------------|------------|
| Backend overview | [backend/overview](https://bedrud.org/en/docs/backend/overview) | `apps/site/.../en/backend/overview.mdx` |
| Configuration | [getting-started/configuration](https://bedrud.org/en/docs/getting-started/configuration) | `.../getting-started/configuration.mdx` |
| Deployment | [guides/deployment](https://bedrud.org/en/docs/guides/deployment) | `.../guides/deployment.mdx` |
| Webhooks (UI) | [guides/webhooks](https://bedrud.org/en/docs/guides/webhooks) | `.../guides/webhooks.mdx` |
| WebRTC / TURN | [architecture/webrtc-connectivity](https://bedrud.org/en/docs/architecture/webrtc-connectivity) | `.../architecture/webrtc-connectivity.mdx` |

---

## Testing

```bash
make test-back
# or
cd server && go test -v -count=1 ./internal/handlers/...
```

Use `testutil.SetupTestDB()` for integration tests. Use `config.SetForTest(cfg)` to inject test config.

See [Server Testing](../server/testing.md).

---

## Swagger

Annotate handlers with swag comments:

```go
// @Summary List rooms
// @Tags rooms
// @Security BearerAuth
// @Success 200 {array} models.Room
// @Router /room/list [get]
func (h *RoomHandler) ListRooms(c *fiber.Ctx) error { ... }
```

Regenerate:

```bash
make swagger-gen
```

---

## Config during development

- File: `server/config.local.yaml` (from example template)
- Override: `CONFIG_PATH=...` env var
- Admin runtime overrides: `SystemSettings` in DB (via admin panel)

See [Configuration](../server/configuration.md).

---

## Useful CLI while developing

```bash
cd server

# Users
go run ./cmd/bedrud user list
go run ./cmd/bedrud user promote --email you@example.com

# Rooms
go run ./cmd/bedrud room list
go run ./cmd/bedrud room close --id <uuid>

# DB
go run ./cmd/bedrud db migrate
go run ./cmd/bedrud db status

# Config
go run ./cmd/bedrud config validate
```

Full CLI reference: [docs/server/cli.md](../server/cli.md).