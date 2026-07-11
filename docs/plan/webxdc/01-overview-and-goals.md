# 01 — Overview & Goals

**Feature status: experimental** — opt-in only; see [10 — Config & installer](./10-config-and-installer.md).

## Problem

Bedrud meetings already support chat, whiteboard, YouTube, stage, and presence over LiveKit. Users (and Delta Chat–adjacent workflows) also want **small collaborative mini-apps** packaged as **WebXDC** (`.xdc` ZIP of HTML/JS/CSS/assets) that:

- Run **inside** a meeting without tracking or third-party network access.
- Sync state **across all participants** in near real time.
- Reuse Bedrud’s existing realtime stack (no new message broker).

## What is WebXDC (host perspective)

Per the [official WebXDC docs](https://webxdc.org/docs) philosophy:

- Mini apps have **no internet access** and run in a **sandboxed web view**.
- They must **not** track users or leak data to third parties.
- The host unpacks/serves content **from the `.xdc` only**.
- Apps talk to the host via **`window.webxdc`** (postMessage-based), not via raw network APIs.
- Authentication, identity, and transport are **outsourced to the host** (messenger / Bedrud).

Bedrud is a **web host** (browser + React meeting page + Go API + LiveKit), not a native messenger. The security *intent* is the same as Delta Chat; the *mechanism* is **origin isolation + iframe sandbox + CSP + Permissions-Policy + validated zip serve + a JS bridge to LiveKit**. See [02](./02-security-sandbox-csp.md).

## Goals

1. **Upload / attach** a `.xdc` to a room and run it in the meeting UI; later, pick from an **optional app gallery** (curated/global store-style — [13](./13-app-gallery.md)).
2. **Contain** every instance so a malicious mini-app cannot:
   - reach the open internet (best-effort layered browser policy; verified per browser),
   - read SPA cookies/storage/DOM,
   - obtain LiveKit/JWT secrets,
   - use camera/mic/geolocation without explicit future review.
3. **Act as a WebXDC messenger host** per official `spec/messenger.md` (local clone under `context/webxdc-website`): serve ZIP assets, provide **`webxdc.js`**, deny internet, isolate storage per app instance.
4. **Bridge status updates** (`sendUpdate` / `setUpdateListener` with serials) and optionally **ephemeral realtime** (`joinRealtimeChannel`) — see [03](./03-realtime-bridge-architecture.md) and [07](./07-official-spec-mapping.md).
5. **Late joiners** catch up on **status** serials (realtime stays ephemeral).
6. Run third-party `.xdc` apps that only use the public API (e.g. get_started sample) without Bedrud-specific forks.

## Non-goals (v1)

- Full parity with every Delta Chat host API extension (sendToChat, import/export, etc.) unless required for a minimum viable demo set of apps.
- Server-side execution of mini-app logic (mini-apps remain pure client JS).
- Persisting arbitrary WebXDC state in Postgres long-term (v1 may be room-lifetime / sessionStorage / optional Yjs hybrid only).
- Allowing mini-apps to call Bedrud REST or LiveKit APIs directly.
- Nested WebXDC or loading `.xdc` from external URLs **inside the mini-app**.
- Kernel-level network namespace isolation (native webviews may be stronger than pure browser CSP; residual risk is documented in [02 §9](./02-security-sandbox-csp.md)).
- Guaranteeing mini-app authors are benign (they are not trusted).
- **v0 core:** full global gallery parity with webxdc.org/apps (phased in [13](./13-app-gallery.md); room upload ships first).

## Success criteria

| Criterion | Measure |
|-----------|---------|
| No network from iframe | Malicious test app: `fetch`, XHR, WS, `sendBeacon` fail; **WebRTC** attempt fails or is documented residual; Network panel shows only app-origin assets |
| Origin isolation | Mini-app origin ≠ SPA origin **or** sandbox lacks `allow-same-origin`; cannot read `parent.document` / SPA `localStorage` |
| No sandbox escape | With chosen flags, iframe cannot remove sandbox / become SPA-origin script |
| Serve from container | All app assets from validated `.xdc`; **`webxdc.js` from host** only |
| Storage isolation | `localStorage` / IDB work and are **not** shared across app instances or with SPA |
| Sandbox attributes | Minimal allow-list; **no** popups/top-navigation/forms unless reviewed |
| Secrets isolation | JWT / LiveKit token never appear in iframe JS heap / URL / storage |
| Bridge hygiene | Forged `postMessage` ignored; official size/interval limits enforced |
| Multi-user sync | Two+ browsers share status updates; serial catch-up works |
| Spec sample app | get_started `index.html` pattern runs without modification |
| No new infra | Only LiveKit + existing Go API + React meeting UI |

## Stakeholders / surfaces

| Surface | Role | Trust |
|---------|------|-------|
| `apps/web` meeting UI | Iframe host, bridge, topic listener, UX | Trusted |
| Go server | Auth, store/serve `.xdc`, CSP headers | Trusted |
| LiveKit | Reliable broadcast of updates | Trusted transport; peers untrusted |
| `.xdc` package | App UI/logic | **Untrusted** |
| Peer `webxdc` packets | Collaborative updates | **Untrusted** |

## Document map

- Security: [02](./02-security-sandbox-csp.md) (**read first for implementers**)
- Bridge: [03](./03-realtime-bridge-architecture.md)
- Payloads: [04](./04-payloads-and-edge-cases.md)
- Build plan: [05](./05-implementation-roadmap.md)
- Verify: [06](./06-verification-checklist.md)
- Spec mapping: [07](./07-official-spec-mapping.md)
- Delta Chat Desktop host: [08](./08-deltachat-desktop-host.md)
- Review & tests: [09](./09-review-and-tests.md)
- Config & installer: [10](./10-config-and-installer.md)
- API, schema, tickets: [11](./11-api-schema-and-tickets.md)
- UI, RBAC, lifecycle, ops: [12](./12-ui-rbac-lifecycle-ops.md)
- App gallery (global/curated): [13](./13-app-gallery.md)
- Local clones: [`context/README.md`](../../../context/README.md)
