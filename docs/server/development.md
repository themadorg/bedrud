# Development Guide

How to build, test, and develop the Bedrud server locally.

> **For contributor workflows and recipes**, see [Developer Documentation](../developer/README.md) and [Server Developer Guide](./developer-guide.md).

**Public docs:** [Development Workflow](https://bedrud.org/en/docs/guides/development) · [Makefile Reference](https://bedrud.org/en/docs/guides/makefile) — [`guides/development.mdx`](../../apps/site/src/content/docs/en/guides/development.mdx). Full map: [public-docs.md](./public-docs.md).

---

## Prerequisites

- Go 1.26+
- Make (repo root `Makefile`)
- Air (hot-reload, installed by `make init`)
- Mage (optional, for `server/magefile.go`)

---

## Setup

From repo root:

```bash
make init      # LiveKit placeholder + config + bun + go mod tidy
```

This copies `server/config.local.yaml.example` → `server/config.local.yaml` and creates the LiveKit binary placeholder.

---

## Running

| Command | What |
|---------|------|
| `make dev` | Full stack: LK + hot-reload server + web frontend |
| `make dev-server` | Backend + LK (no reload) |
| `make dev-server-hot` | Backend + LK + Air hot-reload |
| `make dev-api` | Backend only (no LK) |
| `make dev-livekit` | LK only |

### Direct Go commands

```bash
cd server
go run ./cmd/bedrud run                    # Production entrypoint
go run ./cmd/server                        # Dev API (no CLI)
CONFIG_PATH=config.local.yaml go run ./cmd/bedrud run
```

### Hot reload (Air)

Config: `server/.air.toml`:

| Setting | Value |
|---------|-------|
| Build cmd | `go build -o ./tmp/server ./cmd/server/main.go` |
| Run | `CONFIG_PATH=./config.yaml ./tmp/server` |
| Watch | `.go` files only |
| Exclude | `tmp`, `dist`, `frontend`, `docs`, `*_test.go` |
| Delay | 500ms rebuild debounce |

```bash
make dev-server-hot
# or
cd server && air
```

---

## Building

### Full production binary (with embedded frontend)

From repo root:

```bash
make build         # Frontend → server/frontend/ → Go embed → binary
make build-dist    # Compressed linux/amd64 tarball
```

Build order matters: frontend must be built and copied to `server/frontend/` before `go build`, or the binary serves no SPA.

### Server-only build

```bash
cd server
go build -o bedrud ./cmd/bedrud
```

### Mage tasks

```bash
cd server
mage Build         # Build to dist/bedrud (from cmd/server)
mage Swagger       # Regenerate swagger docs
mage InstallDeps   # go mod tidy
```

---

## Testing

```bash
# From repo root
make test-back

# Or directly
cd server
go test -v -count=1 ./...
go vet ./...
go build ./...
```

### Test utilities

`internal/testutil/` provides:

| Export | Purpose |
|--------|---------|
| `SetupTestDB()` | In-memory SQLite + migrations + cleanup fn |
| `TeardownTestDB(db)` | Close test DB |
| LiveKit mock | Mock RoomService client for handler tests |

### Test config

`config.SetForTest(cfg)` bypasses `sync.Once` in `config.Load` for unit tests.

---

## Linting

```bash
cd server
golangci-lint run    # Config: .golangci.yml
```

---

## Swagger Generation

```bash
make swagger-gen     # From repo root (needs swag CLI)
# or
cd server && mage Swagger
```

Output: `server/docs/docs.go`, `swagger.json`, `swagger.yaml`.

Swagger annotations live in `cmd/server/main.go` and handler files.

---

## LiveKit Binary Placeholder

`server/internal/livekit/bin/livekit-server` must exist at build time (even if empty). CI:

```bash
mkdir -p server/internal/livekit/bin
touch server/internal/livekit/bin/livekit-server
```

`make init` downloads the real binary for local dev.

---

## Environment Overrides

Common dev overrides:

```bash
SERVER_PORT=8090 CONFIG_PATH=config.local.yaml make dev-api
SERVER_HTTP_PORT=8080 bedrud run    # Non-root HTTP
DB_TYPE=sqlite DB_PATH=./test.db go test ./...
QUEUE_POLL_INTERVAL=100 go run ./cmd/bedrud run
```

See [Configuration](./configuration.md) for the full env var list.

---

## Privileged Ports

Default `httpPort` is `:80`. Non-root cannot bind. Options:

1. Set `httpPort: "8080"` in config
2. `sudo setcap 'cap_net_bind_service=+ep' $(which bedrud)` (re-run after each binary update)

---

## Project Files

| File | Purpose |
|------|---------|
| `.air.toml` | Air hot-reload configuration |
| `.golangci.yml` | Linter rules |
| `.swaggo` | Swag type overrides |
| `.env.example` | Environment variable reference |
| `magefile.go` | Mage build tasks (build tag `mage`) |
| `go.mod` | Module `bedrud` |