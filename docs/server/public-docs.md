# Public Documentation (bedrud.org)

Bedrud maintains **two documentation layers**. Use both: public docs for operators and end users; `docs/server/` for contributors working in the codebase.

| Layer | Location | Audience | URL |
|-------|----------|----------|-----|
| **Public site** | `apps/site/src/content/docs/en/` | Operators, deployers, API consumers | [bedrud.org/en/docs](https://bedrud.org/en/docs) |
| **Internal server docs** | `docs/server/` | Contributors, agents, code auditors | This directory |

Public docs are built with Astro SSG, support 10 locales (English source in `en/`), and deploy to GitHub Pages. Sidebar: `apps/site/src/content/docs/sidebar.ts`.

---

## When to use which

| Need | Start here |
|------|------------|
| Deploy Bedrud on a server | [getting-started/installation](https://bedrud.org/en/docs/getting-started/installation) |
| Full `config.yaml` operator reference | [getting-started/configuration](https://bedrud.org/en/docs/getting-started/configuration) |
| Behind nginx / Cloudflare | [guides/behind-proxy](https://bedrud.org/en/docs/guides/behind-proxy) |
| Admin dashboard walkthrough | [guides/admin-dashboard](https://bedrud.org/en/docs/guides/admin-dashboard) |
| Webhook setup (UI screenshots) | [guides/webhooks](https://bedrud.org/en/docs/guides/webhooks) |
| Authoritative route table from code | [routes.md](./routes.md) |
| Handler method index | [handlers-reference.md](./handlers-reference.md) |
| Queue claim algorithm, job types | [queue-deep-dive.md](./queue-deep-dive.md) |
| Add a new API endpoint | [developer-guide.md](./developer-guide.md) |

**Rule of thumb:** If it ships to bedrud.org and has screenshots or deployment steps, use the public doc. If it traces exact code paths or migration SQL, use `docs/server/`.

---

## Backend section (public ↔ internal)

| Public doc | URL | Internal counterpart |
|------------|-----|----------------------|
| Backend Documentation | [/en/docs/backend/overview](https://bedrud.org/en/docs/backend/overview) | [README.md](./README.md), [architecture.md](./architecture.md) |
| Code Structure | [/en/docs/backend/structure](https://bedrud.org/en/docs/backend/structure) | [structure.md](./structure.md), [project-tree.md](../project-tree.md) |
| Database & Models | [/en/docs/backend/database](https://bedrud.org/en/docs/backend/database) | [database-schema.md](./database-schema.md), [internal/models.md](./internal/models.md) |
| Authentication | [/en/docs/backend/authentication](https://bedrud.org/en/docs/backend/authentication) | [auth-flows.md](./auth-flows.md), [internal/auth.md](./internal/auth.md) |
| API Handlers | [/en/docs/backend/api-handlers](https://bedrud.org/en/docs/backend/api-handlers) | [handlers-reference.md](./handlers-reference.md), [api.md](./api.md) |
| LiveKit Integration | [/en/docs/backend/livekit](https://bedrud.org/en/docs/backend/livekit) | [internal/livekit.md](./internal/livekit.md), [room-lifecycle.md](./room-lifecycle.md) |
| Deployment | [/en/docs/backend/deployment](https://bedrud.org/en/docs/backend/deployment) | [bootstrap.md](./bootstrap.md), [internal/install.md](./internal/install.md) |
| Advanced Topics | [/en/docs/backend/advanced](https://bedrud.org/en/docs/backend/advanced) | [architecture.md](./architecture.md), [backend-patterns.md](./backend-patterns.md) |

**Source files:** `apps/site/src/content/docs/en/backend/*.mdx`

---

## Architecture section (public)

| Public doc | URL | Related internal doc |
|------------|-----|-------------------|
| Architecture Overview | [/en/docs/architecture/overview](https://bedrud.org/en/docs/architecture/overview) | [architecture.md](./architecture.md) |
| Server Architecture | [/en/docs/architecture/server](https://bedrud.org/en/docs/architecture/server) | [README.md](./README.md), [structure.md](./structure.md) |
| WebRTC Connectivity | [/en/docs/architecture/webrtc-connectivity](https://bedrud.org/en/docs/architecture/webrtc-connectivity) | [internal/livekit.md](./internal/livekit.md) |
| TURN Server | [/en/docs/architecture/turn-server](https://bedrud.org/en/docs/architecture/turn-server) | [configuration.md](./configuration.md) (LiveKit TLS) |
| End-to-End Encryption | [/en/docs/architecture/e2ee](https://bedrud.org/en/docs/architecture/e2ee) | [room-lifecycle.md](./room-lifecycle.md) (`settings_e2ee`) |
| Web Frontend | [/en/docs/architecture/web](https://bedrud.org/en/docs/architecture/web) | [docs/developer/web-development.md](../developer/web-development.md) |
| Bot Agents | [/en/docs/architecture/agents](https://bedrud.org/en/docs/architecture/agents) | `agents/` in [project-tree.md](../project-tree.md) |

---

## Getting started (public)

| Public doc | URL | Related internal doc |
|------------|-----|-------------------|
| Quick Start | [/en/docs/getting-started/quickstart](https://bedrud.org/en/docs/getting-started/quickstart) | [docs/developer/getting-started.md](../developer/getting-started.md) |
| Server Installation | [/en/docs/getting-started/installation](https://bedrud.org/en/docs/getting-started/installation) | [cli.md](./cli.md), [internal/install.md](./internal/install.md) |
| Configuration | [/en/docs/getting-started/configuration](https://bedrud.org/en/docs/getting-started/configuration) | [configuration.md](./configuration.md), [settings-system.md](./settings-system.md) |
| CLI Reference | [/en/docs/getting-started/cli-reference](https://bedrud.org/en/docs/getting-started/cli-reference) | [cli.md](./cli.md) |
| CLI Installer | [/en/docs/getting-started/cli-installer](https://bedrud.org/en/docs/getting-started/cli-installer) | [cli.md](./cli.md) |
| Client Installation | [/en/docs/getting-started/clients](https://bedrud.org/en/docs/getting-started/clients) | `apps/desktop`, `apps/android`, `apps/ios` |

---

## Guides (public)

| Public doc | URL | Related internal doc |
|------------|-----|-------------------|
| Development Workflow | [/en/docs/guides/development](https://bedrud.org/en/docs/guides/development) | [development.md](./development.md), [docs/developer/](../developer/) |
| Deployment Guide | [/en/docs/guides/deployment](https://bedrud.org/en/docs/guides/deployment) | [bootstrap.md](./bootstrap.md) |
| Behind a Proxy/CDN | [/en/docs/guides/behind-proxy](https://bedrud.org/en/docs/guides/behind-proxy) | [security.md](./security.md), [configuration.md](./configuration.md) |
| Docker Guide | [/en/docs/guides/docker](https://bedrud.org/en/docs/guides/docker) | Root `Dockerfile` |
| Internal TLS | [/en/docs/guides/internal-tls](https://bedrud.org/en/docs/guides/internal-tls) | [internal/utils.md](./internal/utils.md) |
| Makefile Reference | [/en/docs/guides/makefile](https://bedrud.org/en/docs/guides/makefile) | [docs/developer/makefile.md](../developer/makefile.md) |
| Package Installation | [/en/docs/guides/packages](https://bedrud.org/en/docs/guides/packages) | [cli.md](./cli.md) |
| Appliance Mode | [/en/docs/guides/appliance](https://bedrud.org/en/docs/guides/appliance) | [internal/install.md](./internal/install.md) |
| Admin Dashboard | [/en/docs/guides/admin-dashboard](https://bedrud.org/en/docs/guides/admin-dashboard) | [settings-system.md](./settings-system.md) |
| User Roles | [/en/docs/guides/roles](https://bedrud.org/en/docs/guides/roles) | [auth-flows.md](./auth-flows.md), [security.md](./security.md) |
| Webhooks | [/en/docs/guides/webhooks](https://bedrud.org/en/docs/guides/webhooks) | [email-webhooks.md](./email-webhooks.md) |

---

## API section (public)

| Public doc | URL | Related internal doc |
|------------|-----|-------------------|
| API Reference (Swagger) | [/en/docs/api/api-reference](https://bedrud.org/en/docs/api/api-reference) | [api.md](./api.md), [routes.md](./routes.md) |

The site sidebar also lists `api/admin`, `api/auth`, `api/rooms`, etc. — those pages are planned; until they exist, use [routes.md](./routes.md) and [handlers-reference.md](./handlers-reference.md) as the code-audited reference.

**Live Swagger:** `http://localhost:8090/api/swagger` (dev server)

---

## Contributing (public)

| Public doc | URL | Related internal doc |
|------------|-----|-------------------|
| Contributing | [/en/docs/contributing](https://bedrud.org/en/docs/contributing) | [docs/developer/contributing.md](../developer/contributing.md) |

---

## Editing public docs

Public doc sources live under:

```
apps/site/src/content/docs/en/<section>/<page>.mdx
```

- Add sidebar entry in `apps/site/src/content/docs/sidebar.ts`
- Other locales fall back to English when a translation is missing
- Local preview: `make dev-site` → typically `http://localhost:4321/en/docs/...`
- Build: `make build-site` → output in `site/`

When you change backend behavior, update **both** the relevant `docs/server/` file and the matching `apps/site/.../en/` MDX if the change is user-visible (config keys, API paths, deployment steps).

---

## Quick links

- **Public docs home:** https://bedrud.org/en/docs
- **Backend overview (public):** https://bedrud.org/en/docs/backend/overview
- **Internal server index:** [README.md](./README.md)
- **Monorepo tree:** [project-tree.md](../project-tree.md)