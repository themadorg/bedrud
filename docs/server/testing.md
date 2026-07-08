# Testing

Test conventions and utilities for the Bedrud server.

---

## Running tests

```bash
# From repo root
make test-back

# Verbose, no cache
cd server && go test -v -count=1 ./...

# Single package
cd server && go test -v ./internal/handlers/...

# Race detector (CI)
cd server && go test -race ./...
```

Also run before commits:

```bash
cd server && go vet ./...
cd server && go build ./...
```

---

## Test file layout

Tests live alongside source files as `*_test.go`:

| Package | Test files | Focus |
|---------|-----------|-------|
| `config/` | `config_test.go`, `configint_test.go` | Config loading, `ConfigInt` |
| `internal/auth/` | `auth_test.go`, `jwt_test.go` | JWT, revocation, auth flows |
| `internal/handlers/` | `*_test.go` (15+ files) | Handler integration, authz, room CRUD |
| `internal/middleware/` | `auth_test.go`, `ratelimit_test.go` | Middleware behavior |
| `internal/models/` | `user_test.go`, `room_test.go`, `passkey_test.go` | Model validation |
| `internal/queue/` | `queue_test.go`, `handler_*_test.go` | Job queue, email, webhooks |
| `internal/repository/` | `*_repository_test.go` | GORM data access |
| `internal/services/` | `room_cleanup_test.go`, `recording_service_test.go` | Service logic |
| `internal/storage/` | `chat_upload_test.go` | Upload backends, tracker |
| `internal/lkutil/` | `lkutil_test.go` | LiveKit helpers |
| `internal/scheduler/` | `scheduler_test.go` | Scheduler tasks |
| `internal/utils/` | `tls_test.go`, `keys_test.go`, `safeio_test.go` | Utilities |

---

## Test utilities (`internal/testutil/`)

### `SetupTestDB() (*gorm.DB, func())`

Creates in-memory SQLite, runs migrations, returns DB + cleanup function.

### `TeardownTestDB(db)`

Close and clean up test database.

### LiveKit mock

Mock `RoomService` client for handler tests without a real LiveKit instance.

---

## Config in tests

```go
cfg := &config.Config{ /* minimal fields */ }
config.SetForTest(cfg)
```

Bypasses `sync.Once` in `config.Load`. **Test-only** — do not use in production code.

---

## Handler integration tests

`handlers/handlers_integration_test.go` and related files test full HTTP request/response cycles with test DB and mocked LiveKit.

Patterns:
- `httptest` or Fiber test helpers
- In-memory SQLite via `testutil.SetupTestDB()`
- JWT generation with test config secrets

---

## Mock patterns

| Mock | Package | Used for |
|------|---------|----------|
| `mockObjectDeleter` | `storage/chat_upload_test.go` | S3 delete failures |
| LiveKit RoomService mock | `testutil/livekit_mock.go` | Room handler tests |

---

## CI order

From `AGENTS.md`:

1. `go vet ./...`
2. `go build ./...`
3. `go test -race ./...`