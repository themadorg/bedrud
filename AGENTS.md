# Bedrud — Repo Guide

Single binary vid meeting. Go (Fiber, GORM) + React + embedded LiveKit.

---

## Architecture

```
bedrud/
├── server/               Go backend (module: bedrud)
│   ├── cmd/bedrud/       CLI entrypoint (run, install, user, --livekit)
│   ├── cmd/server/       API-only entrypoint (Air hot-reload target)
│   ├── internal/
│   │   ├── auth/         AuthService, JWT, session store, OAuth (Goth)
│   │   ├── database/     GORM init (SQLite/Postgres), auto-migrations
│   │   ├── handlers/     HTTP route handlers (auth, rooms, users, admin, prefs)
│   │   ├── install/      Debian systemd installer
│   │   ├── livekit/      Embedded LK binary (embed.FS + subprocess mgmt)
│   │   ├── lkutil/       Shared LiveKit helpers (NewClient, AuthContext, SendSystemMessage)
│   │   ├── middleware/    JWT auth + RBAC middleware
│   │   ├── models/       GORM models (User, Room, Passkey, ChatUpload, Settings, etc.)
│   │   ├── repository/   Data access layer (6 repos)
│   │   ├── queue/        Job queue: async task processing (room_delete, user_delete, etc.)
│   │   ├── scheduler/    Background idle-room cleanup + job cleanup (gocron)
│   │   ├── server/       Bootstrap: Run() wires all subsystems
│   │   ├── services/     RoomCleanupService (cascade delete, suspend)
│   │   ├── storage/      Chat image upload (disk/inline/S3) + ChatUploadTracker (Record/DeleteByRoom)
│   │   ├── templates/    HTML templates (login, index)
│   │   ├── testutil/     Test db utilities
│   │   ├── usercli/      CLI user management (promote/demote/create/delete)
│   │   └── utils/        Self-signed TLS cert generation
│   ├── migrations/       SQL migrations
│   ├── config.local.yaml Dev config template (SQLite, local LK)
│   └── ui.go             //go:embed all:frontend → embed.FS
├── apps/
│   ├── web/              React frontend (TanStack Start, TailwindCSS v4, Bun, Biome)
│   ├── site/             Astro SSG marketing/docs site (bedrud.org, 10-locale i18n)
│   ├── desktop/          Rust + Slint desktop app
│   ├── server/           Server installer assets
│   ├── android/          Android app
│   └── ios/              iOS app
├── site/                 Built static output (deployed to GitHub Pages)
├── agents/               Python LiveKit bots (music, radio, video_stream)
├── packages/
│   └── api-types/        Shared TS types (@bedrud/api-types)
├── tools/cli/            Deployment CLI (pyinfra + Click)
├── .agents/skills/       Custom agent skills (bedrud-server, bedrud-frontend, bedrud-api, + bundled)
```

**Entrypoints:**
- `cmd/bedrud/main.go` → prod CLI: run, install, uninstall, user, --livekit
- `cmd/server/main.go` → dev API (Air). No CLI. Swagger here.

---

## Development

**Setup:**
```bash
make init      # LK + config + bun + go mod tidy
make dev       # LK + hot-reload server + web (concurrent)
```

**Commands:**
| Command | What |
|---------|------|
| `make dev-web` | Frontend only (TanStack Start :3000, proxy /api → :8090) |
| `make dev-server` | Backend + LK (no reload) |
| `make dev-server-hot` | Backend + Air hot-reload |
| `make dev-api` | Backend only (no LK) |
| `make dev-livekit` | LK only |
| `make dev-site` | Astro site dev server |

**Config:** `config.local.yaml` → `config.yaml`. Override: `CONFIG_PATH=/path bedrud run`. Queue: `QueueConfig` in config (pollInterval, maxAttempts, concurrency). Env: `QUEUE_POLL_INTERVAL`, `QUEUE_MAX_ATTEMPTS`, `QUEUE_CONCURRENCY`.

---

## Build & Deploy

```bash
make build         # Frontend → server/frontend/ → Go embed → single binary
make build-dist    # Compressed linux/amd64 tarball
```

**Build order:** Frontend first. `make build` copies `apps/web/build/*` → `server/frontend/` → `//go:embed all:frontend`.

**LK placeholder:** `internal/livekit/bin/livekit-server` must exist (even empty). CI: `mkdir -p internal/livekit/bin && touch internal/livekit/bin/livekit-server`.

**Docker:** Multi-stage cross-compile (`tonistiigi/xx`). See `Dockerfile`.

---

## Verification & Testing

**Server:**
```bash
make test-back     # cd server && go test -v -count=1 ./...
cd server && go vet ./...
cd server && go build ./...
```

**Web:**
```bash
cd apps/web && bun run check    # Biome lint + tsc
cd apps/web && bun run build    # Prod build
```

**Desktop:**
```bash
cargo test -p bedrud-desktop
cargo build -p bedrud-desktop
```

**Site:**
```bash
cd apps/site && bun run check           # Biome lint + format
cd apps/site && bun run typecheck:astro  # Astro type checking
cd apps/site && bun run build            # Prod build
```

**CI order:** Server: `go vet` → `go build` → `go test -race`. Web: `bun run check` → `bun run build`. Site: `bun run check` → `bun run typecheck:astro` → `bun run build`.

---

## Web Frontend Conventions

- **Toolchain:** Bun (not npm/yarn). Biome (not ESLint/Prettier).
- **Path alias:** `#/*` → `./src/*`. Never `../src/*`.
- **Routing:** TanStack Router file-based. `routeTree.gen.ts` auto-gen — never edit.
- **Styling:** TailwindCSS v4. CSS var tokens. See `apps/web/AGENTS.md`.
- **Components:** shadcn/ui in `components/ui/`. Add: `bunx shadcn@latest add <name>`.
- **Shadcn/ui compliance overhauled** (2026-05-16, 4 phases). Key rules:
  - Prefer `Button`, `Input`, `Label`, `Select`, `Switch`, `Badge`, `Card`, `Dialog`, `Tabs`, `RadioGroup` from `@/components/ui/` over raw HTML
  - No inline `style={}` for static values — use Tailwind
  - Use `cn()` from `@/lib/utils` for dynamic classNames
  - No gradient text (`bg-clip-text text-transparent` — banned)
  - No animated aurora blobs — max one static radial glow per page
  - Meeting room uses Tailwind classes now (was 100% inline), shared styles in `components/meeting/meeting.css`

| Command | Purpose |
|---------|---------|
| `bun run lint` | Biome check |
| `bun run lint:fix` | Biome auto-fix |
| `bun run format` | Biome format |
| `bun run typecheck` | tsc only |
| `bun run check` | Biome + tsc (CI) |

---

## Site Frontend (apps/site)

Astro 6 SSG → `site/` (GitHub Pages). Landing, docs, blog. 10-locale i18n.

- **Toolchain:** Bun. Biome. Not npm/yarn.
- **Path alias:** `~/*` → `./src/*`, `@/content/*` → `./src/content/*`.
- **Styling:** TailwindCSS v4. `src/styles/global.css`.
- **Components:** shadcn/ui (new-york style) in `components/ui/`. Add: `cd apps/site && bunx shadcn@latest add <name>`.
- **Output:** Static (`output: "static"`). Build → `apps/site/dist/`, deployed from root `site/`.

**i18n:** 10 locales: en, de, fr, es, zh, ja, tr, fa, ar, ru. Default locale prefixed (`/en/...`). Locale strings in `src/i18n/locales/{lang}.ts`.

**Content:**
- Docs: `src/content/docs/{locale}/*.mdx`. Schema in `src/content.config.ts`.
- Blog: `src/content/blog/{locale}/*.mdx`. Only `en/` exists currently.
- Sidebar: `src/content/docs/sidebar.ts` — manually defined, not auto-generated.
- Fallback: Missing locale doc falls back to `en/` version.

**Search index:** `scripts/generate-search-index.ts` builds per-locale MiniSearch JSON → `public/search-index-{locale}.json`. Runs automatically before `dev` and `build`. Do not edit generated files.

| Command | Purpose |
|---------|---------|
| `make dev-site` | Dev server |
| `make build-site` | Prod build |
| `bun run check` | Biome lint + format |
| `bun run typecheck` | tsc only |
| `bun run typecheck:astro` | Astro type checking |

**CI order:** `bun run check` → `bun run typecheck:astro` → `bun run build`.

**Deploy:** GitHub Pages via `withastro/action`. Triggers after CI success on master. `deploy-site.yml`.

---

## API Docs

Swagger in Go handlers. Gen at build.

- Swagger UI: `http://localhost:8090/api/swagger`
- Scalar UI: `http://localhost:8090/api/scalar`

Regen: `make swagger-gen` (needs `swag` CLI).

---

## Common Gotchas

- **Wrong entrypoint:** `cmd/server/` skips CLI. Use `cmd/bedrud/` for prod.
- **Missing LK placeholder:** Build fails without `internal/livekit/bin/livekit-server` (even empty).
- **Frontend not embedded:** `go build` without `make build` → no frontend → 404.
- **Hot reload:** Only `make dev-server-hot` (Air). `make dev-server` no reload.
- **Path aliases:** Use `#/*` not `../src/*`. TanStack Start resolves via tsconfig paths.
- **Queue polling:** Worker polls every 500ms. SQLite needs serialized writes. Postgres uses `SKIP LOCKED`. If queue jobs stay `pending`, check DB connection limits.
- **Queue handlers are async:** Room delete/suspend, user delete, chat upload S3 all enqueue jobs. Frontend sees 202 Accepted, not immediate result.
- **Config:** Dev config auto-copied. Override: `CONFIG_PATH` env var. LiveKit config override: `LIVEKIT_CONFIG_PATH` env var or `livekit.configPath` in config.yaml.
- **LiveKit webhook:** Embedded LiveKit auto-configures webhook URL to `http://localhost:<httpPort>/api/livekit/webhook` for disconnect detection. For **external LiveKit** (Cloud or self-hosted), manually configure webhook URL in your LiveKit dashboard → `https://<your-domain>/api/livekit/webhook`. Uses same API key/secret for JWT signing.
- **Embedded LiveKit TLS:** When server TLS is enabled (`enableTLS: true`), embedded LiveKit process auto-generates temp config with TURN/TLS (port 5349) using server's certificate. TURN `domain` auto-set from `server.host`, UDP port 3478 configured, relative `certFile`/`keyFile` paths resolved to absolute. Set `livekit.nodeIP` / `LIVEKIT_NODE_IP` for explicit RTC node IP (disables STUN). For custom LiveKit YAML, set `livekit.configPath` or `LIVEKIT_CONFIG_PATH`.
- **Chat message retention:** Config `chat.maxMessageCount` (default 10000) and `chat.messageTTLHours` (default 2160 = 90 days) control frontend-side trimming of chat messages in memory and sessionStorage. LiveKit doesn't persist data channel messages server-side — these are advisory limits enforced client-side. Env: `CHAT_MAX_MESSAGE_COUNT`, `CHAT_MESSAGE_TTL_HOURS`.
- **Privileged ports:** HTTP listener defaults to `:80`. Non-root can't bind. Fix: set `httpPort: "8080"` in config / `SERVER_HTTP_PORT=8080` env, or `sudo setcap 'cap_net_bind_service=+ep' $(which bedrud)` (re-run after each binary update).
- **Site search index:** Auto-generated before dev/build. Don't edit `public/search-index-*.json`.
- **Site sidebar:** Manual in `src/content/docs/sidebar.ts`. Adding doc page? Add sidebar entry too.

---

## Skill Dispatch Guide

Load skill by task. Injects full ctx.

| Task | Load Skill | Provides |
|------|-----------|----------|
| Go backend (handler, model, repo, auth, middleware, DB, queue, scheduler) | `bedrud-server` | Every pkg → file → fn/struct/route. Full dep graph. |
| React/UI (component, route, state, hook, store) | `bedrud-frontend` | Every route → path+purpose. Every component → props+exports. Every lib. Component hierarchy. |
| API endpoints (add/modify/debug) | `bedrud-api` | Complete endpoint table: method, path, auth, handler, req/res shapes. Auth flow. |

**Load:** say skill name or describe task. Auto-dispatches.

---

## Design System (`DESIGN.md`)

`DESIGN.md` — visual design system source of truth. Read before UI work.

**Covers:** brand color tokens (rose primary, teal accent, status colors), foreground/chrome tokens, dark mode overrides, semantic mapping (token → UI element), accessibility rules (color never sole signal, WCAG AA min), typography, spacing, responsive breakpoints, component patterns (buttons, navigation, cards, inputs), hard rules: zero border-radius, no hardcoded hex outside `theme.css`, destructive reserved for irreversible actions.

**Load when:** adding/modifying UI components, layouts, pages; changing colors, themes, dark mode; self-hosting customization / rebranding; accessibility review/audit.

Per-app design docs: `apps/web/DESIGN.md`, `apps/desktop/DESIGN.md`, `apps/site/DESIGN.md`, `apps/android/DESIGN.md`, `apps/ios/DESIGN.md`.

---

## Related Files

- `DESIGN.md` — Project-wide design system (colors, tokens, accessibility, component patterns)
- `apps/web/DESIGN.md` — Web-specific design details
- `apps/web/AGENTS.md` — Frontend design system (shadcn tokens, no gradients, minimal animations)
- `apps/site/README.md` — Site dev/build commands
- `apps/site/src/content.config.ts` — Content collection schemas
- `apps/site/src/content/docs/sidebar.ts` — Manual sidebar definition
- `server/config.local.yaml` — Dev config template
- `Makefile` — All build/dev cmds
- `Dockerfile` — Multi-stage prod build
- `.agents/skills/bedrud-server/SKILL.md` — Full Go backend map
- `.agents/skills/bedrud-frontend/SKILL.md` — Full React frontend map
- `.agents/skills/bedrud-api/SKILL.md` — Complete API endpoint ref

---

## Commit Messages

Fmt: `<action> <what> for <why>`

Actions: `add`, `delete`, `update`.

```
add passkey model for WebAuthn auth
delete unused OAuth helpers for cleaner auth flow
update room handler for guest join support
add invite token repo for gated registration
delete legacy migration for schema v2 migration
update error handling for clearer debug logs
```
