# 09 — Plan review notes & automated tests

Double-check of the plan (docs 01–08) and the **tests that encode invariants** before the full host ships.

---

## Consistency review (2026-07-10)

| Issue found | Resolution |
|-------------|------------|
| Serial authority said “bridge assigns” in flow but “prefer Go” in table | **Locked: Go assigns serials**; LiveKit is fan-out only ([03](./03-realtime-bridge-architecture.md)) |
| Roadmap “server serial later” vs Desktop core model | Persistence decision updated to **Go status log required** ([05](./05-implementation-roadmap.md)) |
| Sandbox snippet looked like default same-origin-to-SPA | Split **dedicated origin** vs **null-origin** cases; reject SPA+both flags ([02](./02-security-sandbox-csp.md)) |
| `connect-src 'none'` vs Desktop `connect-src 'self'` | Keep Bedrud stricter default; document why ([02](./02-security-sandbox-csp.md), [08](./08-deltachat-desktop-host.md)) |
| Status log “optional” in Phase B | Now **required** for ship |
| `frame-ancestors 'self'` with split hosts | Open decision remains: allowlist SPA origin when webxdc host differs |

### Gaps filled (docs 11–12)

| Former gap | Doc |
|------------|-----|
| HTTP API table | [11](./11-api-schema-and-tickets.md) §3 |
| DB schema | [11](./11-api-schema-and-tickets.md) §2 |
| Capability tickets | [11](./11-api-schema-and-tickets.md) §4 |
| Host-header routing | [11](./11-api-schema-and-tickets.md) §5 |
| Meeting UI / experimental badge | [12](./12-ui-rbac-lifecycle-ops.md) §1 |
| Guest RBAC | [12](./12-ui-rbac-lifecycle-ops.md) §2 |
| Room lifecycle cascade | [12](./12-ui-rbac-lifecycle-ops.md) §3 |
| Proxy / DNS / TLS sketches | [12](./12-ui-rbac-lifecycle-ops.md) §5 |
| Public bootstrap | [11](./11-api-schema-and-tickets.md) §3.1 |
| Deferred APIs / multi-client | [12](./12-ui-rbac-lifecycle-ops.md) §6–7 |
| Graduation criteria | [12](./12-ui-rbac-lifecycle-ops.md) §8 |

### Still open (small product choices — defaults in 12 §12)

1. Side panel vs stage for iframe (default: side panel)  
2. Package delete: force-close vs 409 (default: force-close)  
3. Guest upload ever (default: no)  
4. LiveKit full payload vs nudge-only (default: nudge + GET)  
5. ~~Wildcard cert~~ → **required when enabled** (`*.{baseDomain}`); install must instruct DNS `*.` → same IP as main domain ([10](./10-config-and-installer.md)). Choice left: in-tree ACME DNS-01 vs proxy-held cert only.

### Spec/Desktop alignment — confirmed OK

- Host `webxdc.js`, not ZIP  
- Status vs realtime split  
- Limits 128000 / 10000  
- Per-app `selfAddr`  
- CSP on errors + PDF MIME neutralization  
- Relative `href` only  
- Permission allowlist mindset  

---

## Automated tests (landed with this review)

Pure logic ships **now** so CI enforces plan rules before the full UI/server routes exist.

### Go — `server/internal/webxdc`

| Test area | File | Plan link |
|-----------|------|-----------|
| CSP header completeness (connect-src, no draft-only webrtc) | `headers_test.go` | 02 |
| CSP present on error-style responses helper | `headers_test.go` | XDC-01-002 |
| PDF → non-viewer MIME | `mime_test.go` | XDC-01-005 |
| Host path `webxdc.js` interception rule | `serve_path_test.go` | H4 |
| Zip: require `index.html`, zip-slip reject, size caps | `zip_test.go` | 02 §6, 07 format |
| Safe entry path join | `zip_test.go` | zip-slip |

Run:

```bash
cd server && go test ./internal/webxdc/ -count=1
```

### Web — `apps/web/src/components/meeting/webxdc`

| Test area | File | Plan link |
|-----------|------|-----------|
| Topic constants | `webxdcTopic.test.ts` | 04 |
| Limits defaults | `webxdcLimits.test.ts` | sendUpdate defaults |
| Relative href only | `webxdcHref.test.ts` | 04, Desktop |
| Send-update validation | `webxdcUpdate.test.ts` | 04, 07 |
| Wire envelope parse/encode | `webxdcWire.test.ts` | 04 |
| postMessage channel / appId bind | `webxdcHostMessage.test.ts` | 02 H6 |
| Interval rate gate | `webxdcRateLimit.test.ts` | sendUpdateInterval |
| selfAddr stability / unlinkability helpers | `webxdcSelfAddr.test.ts` | 07 selfAddr |

Run:

```bash
cd apps/web && bun run test src/components/meeting/webxdc
```

### Fixtures (manual / future e2e)

| Path | Purpose |
|------|---------|
| `docs/plan/webxdc/fixtures/demo-echo/` | Minimal get_started-style app sources |
| `docs/plan/webxdc/fixtures/hostile-probe/` | Scripts that attempt network / parent access (for DevTools checklist) |

Package to `.xdc` when host exists:

```bash
(cd docs/plan/webxdc/fixtures/demo-echo && zip -9 -r ../demo-echo.xdc .)
```

Do **not** commit generated `.xdc` blobs unless needed for CI e2e later.

### Not yet automated (need runtime host)

- Real iframe network isolation matrix (browser)  
- LiveKit multi-peer  
- Cookie isolation across origins  
- Full [webxdc-test](https://github.com/webxdc/webxdc-test) suite  

Tracked in [06](./06-verification-checklist.md).

---

## How to extend tests when implementing

1. **Server route tests** — hit zipfs GET, assert CSP on 200 and 404, assert ZIP `webxdc.js` overridden.  
2. **Handler tests** — status log serial monotonic, authz, cross-room 403.  
3. **Bridge tests** — integrate `webxdcWire` with LiveKit mock.  
4. Keep pure validators free of React/Fiber so unit tests stay fast.  
5. **Never** load fixtures or helpers from `context/` in tests — only `docs/plan/webxdc/fixtures/` or in-tree testdata under `server/` / `apps/web/`.  
