# Project Tree

Complete directory map of the Bedrud monorepo. Generated from the repository layout; build artifacts (`node_modules/`, `dist/`, hashed bundles) are omitted for clarity.

---

## Top level

```
bedrud/                              # Monorepo root
├── AGENTS.md                        # Repo guide for AI agents and developers
├── CONTRIBUTING.md                  # Contribution process
├── DESIGN.md                        # Project-wide design system
├── Dockerfile                       # Multi-stage production container build
├── LICENSE                          # Apache 2.0
├── Makefile                         # Build & dev orchestration (make help)
├── Cargo.toml                       # Rust workspace (desktop app)
├── Cargo.lock
├── bunfig.toml                      # Bun workspace config
│
├── server/                          # Go backend — single binary, embeds frontend + LiveKit
├── apps/                            # Client applications
├── packages/                        # Shared packages & distro packaging
├── agents/                          # Python LiveKit bots
├── tools/                           # Operational / deployment CLI
├── docs/                            # Markdown documentation (this tree)
├── site/                            # Built Astro site output (GitHub Pages deploy)
├── .agents/skills/                  # Cursor agent skill definitions
└── .github/                         # CI/CD workflows, issue templates
```

---

## `server/` — Go backend

```
server/
├── cmd/
│   ├── bedrud/main.go               # Production CLI binary (cobra)
│   └── server/main.go               # Dev API server (Air hot-reload, Swagger)
│
├── config/
│   ├── config.go                    # Config struct, Load(), env overrides
│   └── *_test.go
│
├── internal/
│   ├── auth/                        # JWT, OAuth (Goth), passkeys, email verification
│   ├── cli/                         # Cobra commands (run, install, user, room, cert, db…)
│   ├── database/                    # GORM init + AutoMigrate
│   ├── handlers/                    # HTTP route handlers (Fiber)
│   ├── install/                     # OS installer (systemd, OpenRC, SysV)
│   ├── livekit/
│   │   └── bin/livekit-server       # Embedded LK binary (placeholder OK for CI)
│   ├── lkutil/                      # Shared LiveKit client helpers
│   ├── middleware/                  # Auth, rate limit, email verified, recordings gate
│   ├── models/                      # GORM database models
│   ├── queue/
│   │   ├── handler_*.go             # Async job handlers
│   │   └── templates/               # Cerberus HTML email templates
│   ├── repository/                  # Data access layer
│   ├── roomcli/                     # CLI room management
│   ├── scheduler/                   # Background cron (idle rooms, cleanup, TLS renew)
│   ├── server/server.go             # Production bootstrap (Run)
│   ├── services/                    # Room cleanup, recording service
│   ├── storage/                     # Chat uploads, recording files, S3 deleter
│   ├── templates/                   # Legacy HTML (pre-React, not served)
│   ├── testutil/                    # Test DB + LiveKit mocks
│   ├── usercli/                     # CLI user management
│   └── utils/                       # TLS certs, email, keys, safe I/O
│
├── frontend/                        # (generated) React build → //go:embed
│   ├── index.html                   # SPA entry (SSR homepage)
│   ├── shell.html                   # Client-router shell
│   └── assets/                      # Hashed JS/CSS bundles (from make build)
│
├── docs/                            # Generated Swagger/OpenAPI (swag)
├── ui.go                            # //go:embed all:frontend
├── magefile.go                      # Mage: Build, Swagger, InstallDeps
├── go.mod / go.sum                  # Module: bedrud, Go 1.26
│
├── config.local.yaml.example        # Dev config template
├── config/livekit.yaml.example      # External LiveKit YAML example
├── .air.toml                        # Air hot-reload config
├── .golangci.yml                    # Linter rules
├── .env.example                     # Env var reference
└── .swaggo                          # Swag type overrides
```

See [server/structure.md](./server/structure.md) for per-file detail.

---

## `apps/` — Client applications

```
apps/
├── web/                             # Primary web UI (React + TanStack Start)
│   ├── src/
│   │   ├── routes/                  # File-based routing (TanStack Router)
│   │   │   ├── __root.tsx
│   │   │   ├── index.tsx            # Landing
│   │   │   ├── auth*.tsx            # Login, register, verify, reset
│   │   │   ├── m.$meetId.tsx        # Meeting room
│   │   │   └── dashboard/           # User dashboard + admin
│   │   │       ├── admin/           # Users, rooms, queue, settings, recordings
│   │   │       └── settings/        # Profile, audio, video, security
│   │   ├── components/
│   │   │   ├── ui/                  # shadcn/ui primitives
│   │   │   ├── meeting/             # Live meeting room UI
│   │   │   ├── admin/               # Admin dashboard widgets
│   │   │   ├── auth/                # Auth forms
│   │   │   └── dashboard/           # Dashboard layout
│   │   ├── lib/                     # api.ts, utils, hooks, auth
│   │   ├── locales/                 # i18n strings
│   │   └── types/                   # Local TS types
│   ├── routeTree.gen.ts             # Auto-generated — never edit
│   ├── AGENTS.md                    # Web UI conventions
│   ├── DESIGN.md                    # Web design tokens
│   └── package.json                 # Bun toolchain
│
├── site/                            # Marketing + docs site (Astro 6 SSG)
│   ├── src/
│   │   ├── content/docs/            # MDX docs (10 locales)
│   │   ├── content/blog/            # Blog posts
│   │   ├── components/              # Astro/React components
│   │   ├── pages/                   # Route pages
│   │   ├── i18n/                    # Locale strings
│   │   └── styles/                  # global.css
│   └── public/                      # Static assets, search indexes
│
├── desktop/                         # Desktop app (Rust + Slint)
│   ├── src/                         # Rust logic (api, auth, livekit, store)
│   ├── ui/                          # .slint UI definitions
│   │   ├── meeting/
│   │   ├── dashboard/
│   │   ├── admin/
│   │   └── auth/
│   └── installer/                   # linux, snap, windows packaging
│
├── android/                         # Kotlin Android app
│   └── app/src/                     # Main source, Gradle build
│
├── ios/                             # Swift iOS app
│   ├── Bedrud/                      # App source
│   │   ├── Core/                    # API, Auth, LiveKit, Instance
│   │   ├── Features/                # Screens
│   │   └── Models/
│   ├── BedrudTests/
│   └── project.yml                  # XcodeGen spec
│
└── server/                          # Server installer assets (not Go code)
    └── installer/linux/             # Debian RPM spec, systemd, postinst
```

---

## `packages/` — Shared & packaging

```
packages/
├── api-types/                       # Shared TypeScript DTOs (@bedrud/api-types)
│   └── src/index.ts
├── aur/                             # Arch Linux AUR packaging
├── chocolatey/                      # Windows Chocolatey package
├── flatpak/                         # Flatpak manifest (desktop)
└── homebrew/                        # Homebrew formula (desktop)
```

---

## `agents/` — LiveKit bots (Python)

```
agents/
├── music_agent/                     # Play audio into a room
├── radio_agent/                     # Stream radio into a room
└── video_stream_agent/              # Stream video file into a room
    ├── main.py
    ├── pyproject.toml
    └── uv.lock
```

Each agent authenticates against a running Bedrud server.

---

## `tools/` — Deployment CLI

```
tools/cli/
├── bedrud.py                        # pyinfra + Click deployment CLI
└── pyproject.toml
```

---

## `docs/` — Documentation (Markdown)

```
docs/
├── README.md                        # Documentation index
├── project-tree.md                  # This file
├── images/                          # Architecture diagrams (SVG)
│
├── developer/                       # Contributor / developer guides
│   ├── README.md                    # Start here for development
│   ├── getting-started.md
│   ├── daily-workflow.md
│   ├── contributing.md
│   ├── makefile.md
│   ├── server-development.md
│   ├── web-development.md
│   ├── debugging.md
│   └── ci.md
│
└── server/                          # Go backend reference
    ├── README.md
    ├── developer-guide.md           # Recipes: add endpoint, model, queue job
    ├── routes.md                    # Authoritative HTTP route table
    ├── structure.md
    ├── configuration.md
    ├── api.md
    └── internal/                    # Per-package reference docs
```

---

## `site/` — Deployed docs output

```
site/                                # Built from apps/site/ → GitHub Pages
```

Do not edit manually; produced by `make build-site`.

---

## `.agents/skills/` — Agent context

```
.agents/skills/
├── bedrud-server/                   # Full Go backend map
├── bedrud-frontend/                 # Full React frontend map
├── bedrud-api/                      # Complete API endpoint reference
├── bedrud-api-auth/                 # Auth endpoints
├── bedrud-api-admin/                # Admin endpoints
├── bedrud-api-rooms/                # Room endpoints
├── bedrud-auth/                     # Auth service + middleware
├── bedrud-data/                     # Models, repos, DB
├── bedrud-http/                     # HTTP bootstrap, handlers
├── bedrud-jobs/                     # Queue, scheduler, cleanup
├── bedrud-realtime/                 # Embedded LiveKit
├── bedrud-fe-meeting/               # Meeting room UI
├── bedrud-fe-admin/                 # Admin dashboard
├── bedrud-fe-platform/              # Routing, API client
├── bedrud-fe-state/                 # Zustand stores
└── …                                # See .agents/skills/ for full list
```

---

## `.github/` — CI/CD

```
.github/
├── workflows/
│   ├── ci.yml                       # Main CI (server, web, site, mobile, desktop)
│   ├── release.yml                  # Release builds on version tags
│   ├── deploy-site.yml              # GitHub Pages deploy
│   ├── codeql.yml                   # Security scanning
│   └── …                            # apt-repo, dnf-repo, pr-beta, dev-nightly
├── ISSUE_TEMPLATE/                  # Bug, feature, security, etc.
├── pull_request_template.md
└── dependabot.yml
```

---

## Data & runtime directories (created at runtime, not in git)

| Path | Created by | Purpose |
|------|------------|---------|
| `server/bedrud-local.db` | Dev server | SQLite database |
| `server/data/uploads/chat/` | Chat uploads | Disk-backed chat images |
| `server/data/recordings/` | Recordings | Recording files (when enabled) |
| `server/tmp/` | Air | Hot-reload build output |
| `server/frontend/` | `make build-embed` | Embedded React assets |
| `apps/web/dist/` | `bun run build` | Web production build |
| `apps/web/node_modules/` | `bun install` | Web dependencies |
| `site/` | `make build-site` | Deployed static site |

---

## Quick navigation by task

| I want to… | Go to |
|------------|-------|
| Change API / auth / rooms | `server/internal/handlers/` |
| Change database schema | `server/internal/models/` + `database/migrations.go` |
| Change web UI | `apps/web/src/` |
| Change admin dashboard | `apps/web/src/routes/dashboard/admin/` |
| Change meeting room | `apps/web/src/components/meeting/` |
| Change public docs site | `apps/site/src/content/docs/` |
| Change shared API types | `packages/api-types/src/` |
| Change CI | `.github/workflows/ci.yml` |
| Read developer guides | `docs/developer/README.md` |
| Read server reference | `docs/server/README.md` |