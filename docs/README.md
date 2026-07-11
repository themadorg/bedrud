# Bedrud Documentation

Documentation for the Bedrud monorepo. Use this index to find what you need.

---

## Project tree

**[Full Project Tree](./project-tree.md)** — complete directory map of the monorepo with annotations. Start here if you need to know where something lives.

---

## For developers (start here)

| Document | Audience | Description |
|----------|----------|-------------|
| [Developer Guide](./developer/README.md) | Contributors | Setup, workflow, how to extend each app |
| [Contributing](./developer/contributing.md) | Contributors | PR process, commits, CI, code style |
| [Daily Workflow](./developer/daily-workflow.md) | Contributors | `make` commands, dev loops per app |
| [Debugging](./developer/debugging.md) | Contributors | Common issues and fixes |
| [Whiteboard Yjs sync](./developer/whiteboard-yjs.md) | Contributors | Excalidraw + Yjs over LiveKit in meeting rooms |
| [Whiteboard multiplayer](./developer/whiteboard-multiplayer.md) | Contributors | Shared canvas, cursors, follow, element locks |
| [CI/CD](./developer/ci.md) | Contributors | What runs in GitHub Actions |

---

## Plans (in progress)

| Document | Description |
|----------|-------------|
| [WebXDC plan](./plan/webxdc/README.md) | **Experimental** mini-apps: [config](./plan/webxdc/10-config-and-installer.md), [API](./plan/webxdc/11-api-schema-and-tickets.md), [UI/ops](./plan/webxdc/12-ui-rbac-lifecycle-ops.md), [gallery](./plan/webxdc/13-app-gallery.md) |
| [Context clones](./../context/README.md) | Local refs: webxdc website + deltachat-desktop |

---

## Server (Go backend)

| Document | Description |
|----------|-------------|
| [Server Overview](./server/README.md) | Architecture, package index |
| [Server Developer Guide](./server/developer-guide.md) | **How to add endpoints, models, queue jobs** |
| [Route Table](./server/routes.md) | Authoritative HTTP routes |
| [Configuration](./server/configuration.md) | `config.yaml` + env vars |
| [API Reference](./server/api.md) | REST API summary |

### Backend deep dives

| Document | Description |
|----------|-------------|
| [Architecture](./server/architecture.md) | Request lifecycle, startup, dependencies |
| [Database Schema](./server/database-schema.md) | Tables, indexes, migrations |
| [Auth Flows](./server/auth-flows.md) | JWT, OAuth, passkeys, verification |
| [Room Lifecycle](./server/room-lifecycle.md) | Rooms, moderation, LiveKit |
| [Handlers Reference](./server/handlers-reference.md) | All handler methods by file |
| [Queue Deep Dive](./server/queue-deep-dive.md) | Async jobs, retries, handlers |
| [Email & Webhooks](./server/email-webhooks.md) | SMTP, outbound webhooks |
| [Settings System](./server/settings-system.md) | YAML vs DB settings |
| [Backend Patterns](./server/backend-patterns.md) | Conventions and recipes |

Full server reference: [docs/server/](./server/)

### Public documentation (bedrud.org)

Operator- and user-facing docs built from [`apps/site/src/content/docs/en/`](../apps/site/src/content/docs/en/). Cross-reference to internal docs: [server/public-docs.md](./server/public-docs.md).

| Section | Public URL |
|---------|------------|
| Backend | [bedrud.org/en/docs/backend/overview](https://bedrud.org/en/docs/backend/overview) |
| Configuration | [bedrud.org/en/docs/getting-started/configuration](https://bedrud.org/en/docs/getting-started/configuration) |
| Deployment | [bedrud.org/en/docs/guides/deployment](https://bedrud.org/en/docs/guides/deployment) |
| API (Swagger) | [bedrud.org/en/docs/api/api-reference](https://bedrud.org/en/docs/api/api-reference) |
| Webhooks | [bedrud.org/en/docs/guides/webhooks](https://bedrud.org/en/docs/guides/webhooks) |
| All docs | [bedrud.org/en/docs](https://bedrud.org/en/docs) |

---

## Other resources

| Resource | Location |
|----------|----------|
| Repo agent guide | [`AGENTS.md`](../AGENTS.md) |
| Contributing (root) | [`CONTRIBUTING.md`](../CONTRIBUTING.md) |
| Public docs site (sources) | [`apps/site/src/content/docs/en/`](../apps/site/src/content/docs/en/) |
| Public ↔ internal doc map | [`docs/server/public-docs.md`](./server/public-docs.md) |
| Web UI conventions | [`apps/web/AGENTS.md`](../apps/web/AGENTS.md) |
| Design system | [`DESIGN.md`](../DESIGN.md) |
| API types package | [`packages/api-types/`](../packages/api-types/) |