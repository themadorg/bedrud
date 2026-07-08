# Makefile Reference

All commands from the repo root `Makefile`. Run `make help` for the live list.

---

## Development

| Target | Description |
|--------|-------------|
| `make init` | Install all deps (LK binary, config, bun, go mod, air) |
| `make dev` | LiveKit + server + web concurrently |
| `make dev-web` | Frontend only (:3000, proxies /api → :8090) |
| `make dev-server` | Backend + LiveKit (manual restart on Go changes) |
| `make dev-server-hot` | Backend + LK + Air hot-reload |
| `make dev-api` | Backend only, no LiveKit |
| `make dev-livekit` | LiveKit only |
| `make dev-site` | Astro docs site dev server |
| `make dev-ios` | Open Xcode project |
| `make dev-android` | Open Android Studio |
| `make dev-desktop` | `cargo run` desktop app |

---

## API documentation

| Target | Description |
|--------|-------------|
| `make swagger-gen` | Regenerate Swagger from `cmd/server/main.go` annotations |
| `make swagger-open` | Open Swagger UI in browser |
| `make scalar-open` | Open Scalar UI in browser |

Requires `swag` CLI installed for `swagger-gen`.

---

## Build

| Target | Description |
|--------|-------------|
| `make build-front` | Build React app only |
| `make build-embed` | Copy web build → `server/frontend/` |
| `make build-back` | Build Go binary |
| `make build` | Full: frontend → embed → binary |
| `make build-dist` | Production linux/amd64 tarball |
| `make build-site` | Build Astro site to `site/` |
| `make local-build` | Single binary with embedded frontend |
| `make local-run` | Build + run locally |
| `make livekit-download` | Download LK binary for current OS/arch |

---

## Test & lint

| Target | Description |
|--------|-------------|
| `make test-back` | `go test -v -count=1 ./...` in server |
| `make fmt` | Format Go + web code |
| `make lint` | Lint web (Biome) |
| `make lint-fix` | Auto-fix web lint issues |

---

## Mobile & desktop

| Target | Description |
|--------|-------------|
| `make build-android-debug` | Debug APK |
| `make build-android` | Release APK |
| `make build-ios` | iOS release archive |
| `make build-ios-sim` | Simulator build + tests |
| `make build-desktop` | Release desktop binary |

---

## Clean

| Target | Description |
|--------|-------------|
| `make clean` | Remove build artifacts |
| `make full-clean` | Also remove node_modules, gradle cache |

---

## Docker

| Target | Description |
|--------|-------------|
| `make docker` | Build Debian-based image |
| `make docker-alpine` | Alpine variant |
| `make docker-distroless` | Distroless variant |