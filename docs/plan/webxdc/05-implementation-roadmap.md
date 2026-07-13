# 05 — Implementation Roadmap

## Key decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Trust model | `.xdc` + peer updates **untrusted** | XSS/exfil risk |
| Network isolation | Layered CSP + Permissions-Policy + sandbox + tests | Official promise; WebRTC CSP gap |
| Origin isolation | **Required** (dedicated origin **or** no `allow-same-origin`) | Avoid sandbox escape |
| Transport | Status: LiveKit `webxdc`; RT: `webxdc-rt` | Spec splits status vs ephemeral realtime |
| Host API | Trusted host `webxdc.js` (not from ZIP) | `spec/api.md` |
| Limits | `sendUpdateMaxSize=128000`, interval 10s | Official defaults |
| `selfAddr` | Opaque per user+appId | Anti-linkability (`selfAddr` docs) |
| Secrets | Never in iframe | Token theft |
| Serve assets | Authz + zipfs + security headers + host `webxdc.js` | Messenger MUST |
| Upload default | Owner/mod only | Highest-impact action |
| Persistence | **Go status log assigns serials** (Desktop-like); LiveKit notifies/fanout | faq/storage honesty + serial fidelity |
| Yjs | App-side over sendUpdate; not host core | shared_state docs |
| Rate limits | Official interval + host caps | DoS + messenger SHOULD |

## Open decisions (must resolve before / during Phase B–C)

1. **Origin strategy (preferred / locked for prod):** per-instance `webxdc-<id>.{baseDomain}` with DNS `*.{baseDomain}` — [10](./10-config-and-installer.md). Null-origin only for constrained dev.
2. **Feature flag:** `webxdc.enabled` default **false**; **experimental**; **domain required** (no IP-only); installer opt-in only when domain is set ([10](./10-config-and-installer.md)).
3. **Serial authority:** Go status log (Desktop/core-like) ([03](./03-realtime-bridge-architecture.md), [08](./08-deltachat-desktop-host.md)).
4. **Who may upload** — default `uploadPolicy: owner_mod`.
5. **Storage backend** for `.xdc` bytes — chat upload vs dedicated table.
6. **Whether v1 ships `joinRealtimeChannel`** — recommended soon after status path.
7. **`frame-ancestors`** — SPA origin only when webxdc host is `baseDomain` suffix.
8. **mailto: / external link** UX (Desktop confirm / open external).
9. **MIME denylist + CSP on 404** — non-optional (XDC-01-002 / 005).

---

## Security gates (cannot skip)

| Gate | When | Pass criteria |
|------|------|----------------|
| G0 | End of plan | Threat model reviewed ([02](./02-security-sandbox-csp.md)) |
| G1 | After origin spike | Parent isolation proven (`parent.document` throws / blocked) |
| G2 | After asset serve | CSP + Permissions-Policy present; zip-slip tests green |
| G3 | After iframe shell | Malicious sample app cannot fetch / read SPA storage |
| G4 | After LiveKit bridge | Forged postMessage + spoofed sender ignored; rate limit works |
| G5 | Before “done” | Full [06](./06-verification-checklist.md) |

---

## Phases

### Phase A — Plan & security spike

- [x] Plan docs + threat model
- [x] Official website/docs cloned to `context/webxdc-website` + [07](./07-official-spec-mapping.md)
- [x] Desktop host review [08](./08-deltachat-desktop-host.md) + consistency review [09](./09-review-and-tests.md)
- [x] Pure invariant tests: `server/internal/webxdc`, `apps/web/.../webxdc`
- [ ] Spike A1: origin isolation + **per-instance storage** (document winner)
- [ ] Spike A2: CSP + `RTCPeerConnection` / `fetch` from iframe (per target browser)
- [ ] Spike A3: postMessage origin/`source` checks
- [ ] Spike A4: host-served `webxdc.js` overrides ZIP entry

**Exit:** G1 notes written into 02 or a short `spikes.md` appendix if needed.

### Phase B — Server: config + schema + tickets + zipfs

Follow [10](./10-config-and-installer.md) + [11](./11-api-schema-and-tickets.md).

- Config + installer experimental opt-in  
- Models: packages, instances, status_updates; room cascade  
- API: packages, instances, ticket, updates POST/GET  
- Host routing + capability tickets  
- Zipfs + host `webxdc.js` + CSP on all statuses  
- Public `{ enabled, experimental, baseDomain }`  

**Exit:** G2.

### Phase C — Frontend shell

Follow [12](./12-ui-rbac-lifecycle-ops.md).

- Apps panel (experimental badge), upload/start, iframe shell  
- Bridge: sendUpdate → POST updates; setUpdateListener; ticket renew  
- External link confirm; untrusted chrome  

**Exit:** G3 + demo-echo single-user.

### Phase D — Multi-peer

- LiveKit nudge + pull; optional `webxdc-rt`  
- info/summary/document; href; notify; mod close + RBAC  
- Guest matrix as in [12](./12-ui-rbac-lifecycle-ops.md)  

**Exit:** G4 + two-browser demo.

### Phase E — Hardening & graduation path

- [06](./06-verification-checklist.md), webxdc-test, proxy docs  
- Room lifecycle cascade verified  
- Graduation criteria checklist ([12 §8](./12-ui-rbac-lifecycle-ops.md)) — keep experimental until met  

**Exit:** G5 (still experimental until graduation).

### Phase F — App gallery (optional, after core)

See [13](./13-app-gallery.md).

- Gallery A: local/admin catalog + Start in room + UI tab (default **off**)  
- Gallery B: bundled demos  
- Gallery C–D: optional remote catalog / download (server-side only, SSRF-safe)  

Not required to call the core host “usable.”

---

## Suggested PR breakdown

| PR | Title | Depends on | Scope | Gate |
|----|-------|------------|-------|------|
| 1 | `add webxdc plan docs for meeting mini-apps` | — | `docs/plan/webxdc/**` + `context/README` | G0 |
| 2 | `add webxdc origin spike notes for isolation` | 1 | Spike results | G1 |
| 3 | `add webxdc config models and status API` | 2 | Config, installer, packages/instances/updates, tickets | G2 |
| 4 | `add webxdc wildcard host zipfs and webxdc.js` | 3 | Host parse, ticket, CSP zipfs | G2 |
| 5 | `add webxdc meeting UI shell experimental` | 4 | Apps panel, iframe, bridge local | G3 |
| 6 | `add webxdc LiveKit nudge and multi-peer status` | 5 | Topic + pull | G4 |
| 7 | `add webxdc realtime chrome and room cascade` | 6 | RT optional, close, cleanup | |
| 8 | `add webxdc hardening verification and ops docs` | 7 | Checklist, proxy, still experimental | G5 |
| 9 | `add webxdc experimental app gallery local catalog` | 8 | [13](./13-app-gallery.md) Gallery A–B | |

Commit style: `<action> <what> for <why>`.

---

## Testing strategy

| Layer | What |
|-------|------|
| Go unit | Zip-slip, oversize, MIME, CSP header presence, authz deny |
| Web unit | Envelope validation, origin filter, rate limiter, identity overwrite |
| Security fixture | Checked-in **malicious** mini-app attempts (fetch, parent access, webrtc) |
| Manual | Two browsers + DevTools isolation checks |
| Regression | Chat / whiteboard / presence |

```bash
make test-back
cd apps/web && bun run check
```

---

## Code rule (mandatory)

**Never reference `context/` from product code** (imports, embeds, tests, Makefile, CI). That directory is learning-only. Re-implement needed logic under `server/` and `apps/web/`. See [README](./README.md#code-rule-no-context-in-product-code).

## Out of scope reminders

See [01](./01-overview-and-goals.md) non-goals. Do not block v1 on Yjs or full Delta Chat API parity.  
**Do** block v1 on failing G1–G4.
