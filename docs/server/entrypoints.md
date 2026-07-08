# Entrypoints

The server has two Go entrypoints and one embed file that bundles the frontend.

---

## `cmd/bedrud/main.go` — Production Binary

The main production executable. Thin wrapper around the Cobra CLI:

```go
package main

import "bedrud/internal/cli"

var version = "dev"

func main() {
    cli.Execute(version)
}
```

**Build:** `go build -o bedrud ./cmd/bedrud` (or via `make build-dist`)

**Version injection:** Set at build time via `-ldflags "-X bedrud/internal/cli.Version=1.2.3"`.

### What it does

Delegates all argument parsing to `internal/cli`. Supports:

- Modern subcommands: `bedrud run`, `bedrud user promote`, etc.
- Legacy flags for backward compatibility: `bedrud --run`, `bedrud --livekit`, `bedrud --version`

### Config resolution

The `--config` persistent flag (or `BEDRUD_CONFIG` / `CONFIG_PATH` env) selects the config file. Default: `config.yaml` in the working directory. Installed systems use `/etc/bedrud/config.yaml`.

---

## `cmd/server/main.go` — Development API Server

Air hot-reload target. **No CLI subcommands.** Initializes all subsystems directly in `main()`:

1. Load config
2. Init database + migrations
3. Init auth, repos, handlers
4. Start queue worker + scheduler
5. Register Fiber routes (including Swagger/Scalar)
6. Serve embedded SPA

**Use when:** Local development with hot reload (`make dev-server-hot`).

**Differences from production:**

| Aspect | `cmd/bedrud` | `cmd/server` |
|--------|--------------|--------------|
| CLI | Full Cobra tree | None |
| Bootstrap | `internal/server.Run()` | Inline in `main.go` |
| Swagger | Available | Available (annotations in main) |
| Hot reload | No (restart required) | Yes (Air) |

### Health endpoints

| Endpoint | Response |
|----------|----------|
| `GET /api/health` | `{"status":"healthy","time":<unix>}` |
| `GET /api/ready` | `{"status":"ready","time":<unix>}` |
| `GET /health` | 307 redirect → `/api/health` |
| `GET /ready` | 307 redirect → `/api/ready` |

### API documentation UIs

| URL | UI |
|-----|-----|
| `/api/swagger` | Swagger UI |
| `/api/scalar` | Scalar UI |

---

## `ui.go` — Frontend Embed

```go
package bedrud

import "embed"

//go:embed all:frontend
var UI embed.FS
```

Embeds the compiled React frontend into the Go binary. At runtime, Fiber's `filesystem` middleware serves files from this `embed.FS`.

### Build pipeline

```
apps/web/  →  bun run build  →  apps/web/build/
                                    ↓
                              make build copies to
                                    ↓
                              server/frontend/
                                    ↓
                              //go:embed in ui.go
                                    ↓
                              Single binary serves SPA
```

### SPA routing

- `/` and API routes → `index.html`
- Non-API client routes → `shell.html` (TanStack Start shell)
- `/livekit/*` → reverse-proxied to embedded LiveKit (not from embed.FS)

### Placeholder requirement

`server/frontend/` must exist at build time (even if empty). CI creates a `.gitkeep`. Without `make build`, the binary has no frontend and returns 404 for `/`.

---

## Choosing an Entrypoint

| Scenario | Entrypoint |
|----------|------------|
| Production deployment | `cmd/bedrud` |
| `bedrud install` / CLI admin | `cmd/bedrud` |
| Local dev with Air hot-reload | `cmd/server` |
| Swagger annotation work | `cmd/server` |
| CI `go build ./...` | Both compile; prod artifact uses `cmd/bedrud` |