# CI/CD

What GitHub Actions runs and how to reproduce locally.

---

## Triggers

- Push to `master`
- Pull requests targeting `master`
- Release tags `v*`

---

## CI jobs (typical)

| Job | Steps | Local equivalent |
|-----|-------|------------------|
| **Server** | `go vet` → `go build` → `go test -race ./...` | See below |
| **Web** | `bun run check` → `bun run build` | `cd apps/web && bun run check && bun run build` |
| **Site** | `bun run check` → `typecheck:astro` → `build` | `cd apps/site && bun run check && ...` |
| **Android** | Lint, unit tests, debug APK | `cd apps/android && ./gradlew test` |
| **iOS** | Simulator build + tests | `make build-ios-sim` |
| **Desktop** | `cargo build`, `cargo test` | `cargo test -p bedrud-desktop` |

Exact workflow files: `.github/workflows/`

---

## Pre-PR checklist

### Backend changes

```bash
cd server
go vet ./...
go build ./...
go test -v -count=1 ./...
# optional
go test -race ./...
golangci-lint run
```

### Frontend changes

```bash
cd apps/web
bun run check
bun run build
```

### Docs site changes

```bash
cd apps/site
bun run check
bun run typecheck:astro
bun run build
```

---

## Production build

```bash
make build-dist    # linux/amd64 tarball with embedded frontend + LK
```

Docker:

```bash
make docker
```

---

## LiveKit placeholder (CI requirement)

CI creates empty placeholder so `go:embed` compiles:

```bash
mkdir -p server/internal/livekit/bin
touch server/internal/livekit/bin/livekit-server
```

`make init` downloads the real binary for local dev.

---

## Release

Tagged releases (`v*`) trigger release builds. Version injected into binary via `-ldflags` in Makefile/Dockerfile.