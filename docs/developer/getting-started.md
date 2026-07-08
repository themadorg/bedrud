# Getting Started

First-time setup for Bedrud development.

---

## 1. Clone and install

```bash
git clone https://github.com/themadorg/bedrud.git
cd bedrud
make init
```

`make init` does:

- Downloads embedded LiveKit binary for your OS/arch
- Copies `server/config.local.yaml.example` → `server/config.local.yaml`
- Installs Bun deps for `apps/web`
- Runs `go mod tidy` in `server/`
- Installs Air (Go hot-reload) if missing

---

## 2. Configure secrets

Edit `server/config.local.yaml` (or `server/config.yaml`):

```yaml
auth:
  jwtSecret: "<32+ random chars>"
  sessionSecret: "<32+ random chars>"
livekit:
  apiSecret: "<32+ random chars>"
```

Generate secrets:

```bash
openssl rand -hex 32
```

Never commit `config.local.yaml` or `config.yaml` with real secrets.

---

## 3. Start development

```bash
make dev
```

| Service | URL |
|---------|-----|
| Web (dev) | http://localhost:3000 |
| API server | http://localhost:8090 |
| Swagger UI | http://localhost:8090/api/swagger |
| LiveKit (internal) | http://127.0.0.1:7880 |

The web dev server proxies `/api` requests to the Go backend.

---

## 4. Create an admin user

After registering via the UI:

```bash
cd server
go run ./cmd/bedrud user promote --email your@email.com
```

Or create directly:

```bash
go run ./cmd/bedrud user create \
  --email admin@local.dev \
  --password secret123 \
  --name "Admin"
go run ./cmd/bedrud user promote --email admin@local.dev
```

---

## 5. Verify your setup

```bash
# Health check
curl http://localhost:8090/api/health

# Server tests
make test-back

# Web checks (if working on frontend)
cd apps/web && bun run check
```

---

## IDE setup

### Go (server)

- Open `server/` as module root or whole repo
- `GOPATH` module: `bedrud` (defined in `server/go.mod`)
- Recommended: Go extension with `gopls`, delve for debugging
- Run/debug target: `cmd/server/main.go` (dev) or `cmd/bedrud/main.go` (prod CLI)

### TypeScript (web)

- Open repo root or `apps/web/`
- Path alias: `#/*` → `./src/*` (see `apps/web/tsconfig.json`)
- Run: `bun run dev` or `make dev-web`

---

## What to read next

| If you're working on… | Read |
|----------------------|------|
| Find any file or folder | [Project Tree](../project-tree.md) |
| Go API / auth / rooms | [Server Development](./server-development.md) |
| React UI | [Web Development](./web-development.md) |
| Any PR | [Contributing](./contributing.md) |
| Makefile commands | [Makefile Reference](./makefile.md) |