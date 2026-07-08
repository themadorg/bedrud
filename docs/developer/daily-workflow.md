# Daily Workflow

How to run Bedrud during day-to-day development.

---

## Choose your dev mode

| Goal | Command | Notes |
|------|---------|-------|
| Everything | `make dev` | LK + server + web concurrently |
| Frontend work | `make dev-web` | HMR at :3000, needs API at :8090 |
| Backend work | `make dev-server-hot` | Air hot-reload on Go changes |
| API without video | `make dev-api` | Fastest backend iteration |
| LiveKit debugging | `make dev-livekit` | LK only at :7880 |
| Docs site | `make dev-site` | Astro at apps/site |

Press `Ctrl+C` to stop `make dev` (kills all child processes).

---

## Backend hot reload

```bash
make dev-server-hot
```

Uses Air (`server/.air.toml`):

- Rebuilds `cmd/server` on `.go` file changes
- Excludes `*_test.go`, `frontend/`, `dist/`
- Runs with `CONFIG_PATH=./config.yaml`

For CLI changes (`internal/cli/`), use `go run ./cmd/bedrud` — Air targets `cmd/server` only.

---

## Frontend dev server

```bash
make dev-web
```

- TanStack Start dev server on `:3000`
- Proxies `/api/*` → `http://localhost:8090`
- Start backend separately (`make dev-api` or `make dev-server`)

---

## After changing Go API

1. Implement handler + route (see [Server Developer Guide](../server/developer-guide.md))
2. Regenerate Swagger if you added annotations:
   ```bash
   make swagger-gen
   ```
3. Run tests:
   ```bash
   make test-back
   ```
4. If frontend consumes new endpoint, update `apps/web/src/lib/api.ts` and types in `packages/api-types/` if shared

---

## After changing database models

GORM auto-migrates on server startup. For manual migration:

```bash
go run ./cmd/bedrud db migrate
```

Verify with:

```bash
go run ./cmd/bedrud db status
```

---

## Production-like local binary

```bash
make local-build   # Frontend embedded into Go binary
make local-run     # Run single binary with SQLite
```

Useful for testing embed/SPA routing without separate dev servers.

---

## Useful URLs while developing

| URL | Purpose |
|-----|---------|
| http://localhost:3000 | Web dev |
| http://localhost:8090 | API + embedded SPA (when using dev-server) |
| http://localhost:8090/api/swagger | Swagger UI |
| http://localhost:8090/api/scalar | Scalar API docs |

Open Swagger quickly:

```bash
make swagger-open
```

---

## Environment overrides (common)

```bash
# Custom config path
CONFIG_PATH=/path/to/config.yaml make dev-api

# Non-privileged HTTP when testing TLS redirect
SERVER_HTTP_PORT=8080 go run ./cmd/bedrud run

# Faster queue polling during job debugging
QUEUE_POLL_INTERVAL=100 CONFIG_PATH=config.local.yaml make dev-api

# Postgres instead of SQLite
DB_TYPE=postgres DB_HOST=localhost DB_USER=bedrud DB_PASSWORD=... make dev-api
```

Full list: [Server Configuration](../server/configuration.md).