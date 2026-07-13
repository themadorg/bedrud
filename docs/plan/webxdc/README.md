# WebXDC in Bedrud — Implementation Plan

Planning docs for embedding **WebXDC** mini-apps in Bedrud meeting rooms.

**Status: experimental.** Opt-in (`webxdc.enabled`, default off). Not a stable production guarantee until graduated ([12 §8](./12-ui-rbac-lifecycle-ops.md)). Self-hosters should treat enablement as a deliberate beta.

WebXDC packages (`.xdc` ZIP) run in a **sandboxed web view with no internet access**, talk to the host only via `window.webxdc`, and sync through Bedrud (Go status log + LiveKit fan-out).

**Security is first-class.** Treat every `.xdc` and peer update as **untrusted**. Start with [02](./02-security-sandbox-csp.md).

**Spec / Desktop context:** [`context/README.md`](../../../context/README.md) · mapping [07](./07-official-spec-mapping.md) · Desktop [08](./08-deltachat-desktop-host.md).

---

## Documents

| # | File | Contents |
|---|------|----------|
| 01 | [Overview & goals](./01-overview-and-goals.md) | Why, scope, non-goals, success criteria |
| 02 | [Security model](./02-security-sandbox-csp.md) | Threat model, origin, CSP, tickets constraints |
| 03 | [Bridge architecture](./03-realtime-bridge-architecture.md) | Status + realtime over LiveKit |
| 04 | [Payloads & edge cases](./04-payloads-and-edge-cases.md) | Envelopes, limits, notify/href |
| 05 | [Implementation roadmap](./05-implementation-roadmap.md) | Phases, gates, PRs |
| 06 | [Verification checklist](./06-verification-checklist.md) | Spec + security tests |
| 07 | [Official spec mapping](./07-official-spec-mapping.md) | Messenger MUST → Bedrud |
| 08 | [Delta Chat Desktop host](./08-deltachat-desktop-host.md) | Reference host lessons |
| 09 | [Review & tests](./09-review-and-tests.md) | Consistency + unit test map |
| 10 | [Config & installer](./10-config-and-installer.md) | `enabled`, `baseDomain`, install opt-in |
| 11 | [API, schema, tickets](./11-api-schema-and-tickets.md) | Routes, DB, Host routing, capability tickets |
| 12 | [UI, RBAC, lifecycle, ops](./12-ui-rbac-lifecycle-ops.md) | Meeting UI, guests, room delete, proxy, graduation |
| 13 | [App gallery](./13-app-gallery.md) | Global/curated gallery (Delta Chat–style store); phased, default off |

Fixtures: [`fixtures/`](./fixtures/).

---

## Locked decisions (summary)

| Topic | Decision |
|-------|----------|
| Feature flag | `webxdc.enabled` default **false**, **experimental** |
| Domain prerequisite | **Domain required** — WebXDC **cannot** be enabled on IP-only installs |
| Production origin | `https://webxdc-<instanceId>.{baseDomain}/` |
| DNS when enabled | **`*.{baseDomain}` → same IP/target as main Bedrud domain** (installer must tell admin) |
| TLS when enabled | Cert **must** cover `*.{baseDomain}` (not main name only) |
| SPA same-origin path | **Rejected** for production |
| Serial authority | **Go status log** (not peer-minted) |
| Fan-out | LiveKit `webxdc` (nudge/status), optional `webxdc-rt` |
| Host script | `webxdc.js` always from host, never ZIP |
| Subdomain auth | Short-lived **capability ticket** (not SPA cookie Domain) |
| Upload default | Owner/mod only |
| Primary client | **Web meeting** only in experimental phase |
| App gallery | Optional ([13](./13-app-gallery.md)); default **off**; not required for core host |
| Meeting end | Room-scoped WebXDC packages/instances/logs/blobs **deleted** with room cleanup ([12 §3](./12-ui-rbac-lifecycle-ops.md)) |

---

## Core constraints (do not dilute)

1. Zero network from mini-app (layered; external links only with confirm).  
2. Origin + storage isolation per instance.  
3. Host `webxdc.js`; CSP + nosniff on **every** response including errors.  
4. No secrets in the iframe.  
5. Status vs realtime separation; server serials.  
6. Per-app opaque `selfAddr`.  
7. Permission allowlist (default deny powerful APIs).  

---

## Architecture (end-to-end)

```
┌─ app.example.com (SPA + /api, JWT) ─────────────────────┐
│  Apps panel → POST package / instance / ticket           │
│  Bridge: postMessage ↔ LiveKit ↔ GET status log         │
└────────────────────────────┬────────────────────────────┘
                             │ iframe src + ticket
┌─ webxdc-<id>.wx.example.com ────────────────────────────┐
│  Host parse + ticket → zipfs + webxdc.js + CSP           │
│  Untrusted mini-app only                                 │
└──────────────────────────────────────────────────────────┘
```

Detail: [11](./11-api-schema-and-tickets.md), [12](./12-ui-rbac-lifecycle-ops.md).

---

## Security / ship gate

- [x] Origin strategy preferred: per-instance subdomain ([10](./10-config-and-installer.md), [02](./02-security-sandbox-csp.md))  
- [x] API + DB + tickets documented ([11](./11-api-schema-and-tickets.md))  
- [x] UI / RBAC / lifecycle / ops documented ([12](./12-ui-rbac-lifecycle-ops.md))  
- [ ] Config + installer implemented  
- [ ] Zipfs Host routing + tickets implemented  
- [ ] Checklist [06](./06-verification-checklist.md) A–B on hostile fixture  
- [ ] Graduation criteria ([12 §8](./12-ui-rbac-lifecycle-ops.md)) before dropping “experimental”  

---

## Related Bedrud code

| Concern | Pattern |
|---------|---------|
| Reliable DC | Chat, presence |
| Yjs (trusted whiteboard) | `whiteboard-yjs` — not mini-app host |
| Public settings | Extend `/api/auth/settings` or `/api/webxdc/config` |
| Room delete | Queue + cascade webxdc tables |
| Pure validators | `server/internal/webxdc`, `apps/web/.../webxdc` |

---

## Code rule: no `context/` in product code

Local `context/` checkouts (upstream WebXDC docs, Delta Chat Desktop, etc.) exist **only for humans/agents to learn from**.

| Allowed | Forbidden |
|---------|-----------|
| Read upstream ideas and re-implement in Bedrud | `import` / `require` / `embed` / `go:embed` from `context/` |
| Copy **patterns** into `server/`, `apps/web/`, etc. as first-party code | Relative or absolute paths to `context/` in source, tests, or build scripts |
| Cite public URLs in comments if useful | CI, Makefile, or runtime depending on `context/` existing |

All WebXDC behavior must live under normal package trees (e.g. `server/internal/webxdc`, `apps/web/src/components/meeting/webxdc`). If something useful is learned from upstream, **write it into Bedrud code or this plan** — do not link the tree.

---

## Branch

Work targets branch **`webxdc`**.
