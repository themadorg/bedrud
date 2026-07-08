# Database Layer (`internal/database`)

GORM initialization, connection management, and schema migrations.

---

## Files

| File | Purpose |
|------|---------|
| `database.go` | Connection init, singleton, close |
| `migrations.go` | AutoMigrate + raw SQL constraints |

---

## Connection (`database.go`)

### `Initialize(cfg *config.Config) error`

Creates GORM connection based on `database.type`:

| Type | Driver | Notes |
|------|--------|-------|
| `sqlite` | `gorm.io/driver/sqlite` | `database.path` for file location. `SetMaxOpenConns(1)` for queue worker |
| `postgres` | `gorm.io/driver/postgres` | Connection pooling via `maxIdleConns`, `maxOpenConns`, `maxLifetime` |

### Exports

| Function | Purpose |
|----------|---------|
| `GetDB()` | Singleton `*gorm.DB` |
| `Close()` | Close underlying `*sql.DB` |

---

## Migrations (`migrations.go`)

### `RunMigrations() error`

GORM `AutoMigrate` for all models:

- `User`
- `BlockedRefreshToken`
- `Room`, `RoomParticipant`, `RoomPermissions`
- `Passkey`
- `SystemSettings`
- `InviteToken`
- `UserPreferences`
- `ChatUpload`
- `Job`
- `Recording`
- `Webhook`, `WebhookDelivery`
- `VerificationEvent`

### Raw SQL constraints (PostgreSQL)

| Constraint | Purpose |
|-----------|---------|
| `fk_room_permissions_participant` | RoomPermissions → RoomParticipant FK |
| `fk_chat_uploads_room` | ChatUpload → Room FK with `ON DELETE CASCADE` |

SQLite uses GORM-level constraints where possible.

---

## CLI integration

```bash
bedrud db migrate    # Run migrations
bedrud db status     # Connection + migration status
```

---

## Test database

`internal/testutil.SetupTestDB()` creates in-memory SQLite, runs migrations, returns cleanup function.