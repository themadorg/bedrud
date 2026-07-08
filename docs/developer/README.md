# Developer Documentation

Practical guides for contributing to Bedrud. This section is for people who write code — not operators deploying a server.

---

## Quick start (5 minutes)

```bash
git clone https://github.com/themadorg/bedrud.git
cd bedrud
make init
make dev
```

Open `http://localhost:3000` (web dev server, proxies `/api` → `:8090`).

Promote yourself to admin after creating a user:

```bash
cd server
go run ./cmd/bedrud user promote --email you@example.com
```

---

## Documentation map

### Essentials

| Guide | What you'll learn |
|-------|-------------------|
| [Project Tree](../project-tree.md) | **Full repo directory map** — where everything lives |
| [Getting Started](./getting-started.md) | Toolchain, first run, project layout |
| [Daily Workflow](./daily-workflow.md) | Which `make` target to use when |
| [Contributing](./contributing.md) | Branches, commits, PRs, pre-submit checks |
| [Makefile Reference](./makefile.md) | All build/dev/test commands |
| [Debugging](./debugging.md) | Port conflicts, DB, LiveKit, auth issues |
| [CI/CD](./ci.md) | GitHub Actions jobs and local equivalents |

### By application

| Guide | Stack |
|-------|-------|
| [Server Development](./server-development.md) | Go, Fiber, GORM, LiveKit |
| [Web Development](./web-development.md) | React, TanStack Start, Bun, Biome |
| [Server Developer Guide](../server/developer-guide.md) | Recipes: add API endpoint, model, queue job |

### Reference (read-only)

| Section | Content |
|---------|---------|
| [Server docs](../server/) | Full backend architecture reference |
| [AGENTS.md](../../AGENTS.md) | Repo-wide conventions for AI agents |
| [Site docs](../../apps/site/src/content/docs/en/) | User-facing docs (bedrud.org) |

---

## Repository layout

See **[Project Tree](../project-tree.md)** for the full annotated directory map (every major folder and where to edit code).

Quick overview:

```
bedrud/
├── server/              # Go backend (module: bedrud)
├── apps/
│   ├── web/             # React web app (primary UI)
│   ├── site/            # Astro marketing/docs site
│   ├── desktop/       # Rust + Slint desktop app
│   ├── android/         # Kotlin Android app
│   └── ios/             # Swift iOS app
├── packages/
│   └── api-types/       # Shared TypeScript DTOs
├── agents/              # Python LiveKit bots
├── tools/cli/           # Deployment CLI (pyinfra)
├── docs/                # This documentation
│   ├── project-tree.md  # Full repo directory map
│   ├── developer/       # Developer guides (you are here)
│   └── server/          # Server reference docs
├── Makefile             # Build orchestration
└── Dockerfile           # Production container build
```

---

## Minimum toolchain

| Tool | Version | Used for |
|------|---------|----------|
| Go | 1.26+ | Server |
| Bun | 1.0+ | Web, site |
| Make | any | Dev orchestration |
| Git | any | Version control |

Optional depending on what you work on: Rust (desktop), Android Studio, Xcode, Python 3.10+ (agents), `swag` CLI (Swagger gen), Air (server hot-reload), golangci-lint.

---

## Typical dev loops

| Task | Command |
|------|---------|
| Full stack | `make dev` |
| API only (fast) | `make dev-api` |
| API + hot reload | `make dev-server-hot` |
| Frontend only | `make dev-web` |
| Run all checks before PR | See [Contributing](./contributing.md#before-submitting-a-pr) |