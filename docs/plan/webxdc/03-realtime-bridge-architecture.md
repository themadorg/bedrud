# 03 — Host Bridge Architecture (Status + Realtime)

## Purpose

WebXDC apps have **no internet**. Collaboration is host-mediated.

Official API has **two** transport styles ([07](./07-official-spec-mapping.md)):

| Mode | App API | Semantics | LiveKit (Bedrud) |
|------|---------|-----------|------------------|
| **Status** | `sendUpdate` / `setUpdateListener` | JSON updates with **serial** catch-up; best-effort; may be reordered/lost | Topic **`webxdc`**, reliable; host assigns serials; keep last-N / snapshot |
| **Realtime** | `joinRealtimeChannel` (experimental) | Ephemeral `Uint8Array` ≤ 128000; **no** late join | Topic **`webxdc-rt`**; no serial log |

Do not mix the two logs.

Security invariants: [02](./02-security-sandbox-csp.md).

---

## Layers

```
┌─────────────────────────────────────────────┐
│  UNTRUSTED iframe  (index.html from .xdc)   │
│  webxdc.js (HOST-PROVIDED, not from ZIP)    │
└──────────────────┬──────────────────────────┘
                   │ postMessage
┌──────────────────▼──────────────────────────┐
│  TRUSTED React bridge                         │
│  serial store · rate limits · RBAC · notify │
└──────────────────┬──────────────────────────┘
                   │
     ┌─────────────┴──────────────┐
     ▼                            ▼
 topic webxdc (status)    topic webxdc-rt (ephemeral)
     └─────────────┬──────────────┘
                   ▼
              LiveKit room
```

---

## Status update flow (`sendUpdate`)

### App → host

1. App calls `window.webxdc.sendUpdate({ payload, info?, href?, document?, summary?, notify? }, "")`.
2. Host `webxdc.js` posts to parent (validated: source, origin, size ≤ `sendUpdateMaxSize`, interval).
3. Bridge `POST`s update to **Go status log** (or queues publish after server assigns serial).
4. Server assigns monotonic **`serial`** per `appId` (gaps allowed if deletes; never client-minted for authority).
5. Bridge updates local chrome (`info` / `document` / `summary`) as plain text; handles `notify` for local `selfAddr` / `*`.
6. Fan-out: LiveKit topic `webxdc` carries full update **or** a nudge; peers pull `GET …/updates?after=` when using nudge mode.
7. Deliver to local `setUpdateListener` with `{ payload, serial, max_serial, … }` (own updates included).

### Peer → app

1. Receive LiveKit `webxdc` (body or nudge); validate size/schema.
2. Ensure local log has updates through `max_serial` (apply body and/or pull from Go).
3. If app open: `postMessage` into iframe → listener.
4. If `info` with `href`: store for activity-line open (`root + relative href` only).

### Serial authority (**locked: server**)

Delta Chat Desktop does **not** invent serials in the UI. Core owns the log. Bedrud mirrors that:

| Approach | Status |
|----------|--------|
| **C. Go assigns serial per `appId`** | **Required for ship** (minimal SQLite/Postgres log) |
| A/B/D peer-only serials | Dev spike only — do not call “spec complete” |

LiveKit is the **fan-out bus**, not the serial authority ([08](./08-deltachat-desktop-host.md)).

### `setUpdateListener(cb, serial)`

- Replay all stored updates with `update.serial > serial` (default `serial = 0`).
- Resolve returned **Promise** when replay finished.
- Subsequent live updates invoke `cb`.
- Second call replaces listener (last wins).

### Limits (official defaults)

Expose on `window.webxdc`:

- `sendUpdateInterval = 10000` (ms) — bridge may coalesce/delay faster calls.
- `sendUpdateMaxSize = 128000` (bytes) — reject oversize serialized update.

---

## Realtime channel flow (`joinRealtimeChannel`)

1. App: `const ch = window.webxdc.joinRealtimeChannel()`.
2. Second join without leave → **throw**.
3. `ch.send(Uint8Array)` → parent → LiveKit topic `webxdc-rt` (payload raw or thin envelope with `appId` only).
4. `ch.setListener(cb)` receives `Uint8Array` from peers (same app only).
5. `ch.leave()` invalidates channel.
6. **No** persistence, **no** snapshot for late joiners (spec).

Feature-detect for apps: `joinRealtimeChannel !== undefined`. Ship as soon as status path works if cost is low.

---

## Instance isolation

| Resource | Keyed by |
|----------|----------|
| Iframe | `appId` |
| web storage | origin / partition per `appId` (messenger MUST) |
| Status log | `appId` |
| Realtime channel | `appId` |
| `selfAddr` | user + `appId` (not shared across apps) |

Same package bytes opened twice ⇒ two `appId`s ([messenger.md](../../../context/webxdc-website/src-docs/spec/messenger.md)).

---

## Bridge module responsibilities

| Duty | Detail |
|------|--------|
| Serve/inject `webxdc.js` | Trusted implementation of full API surface we claim |
| Status validate + serial | Size, interval, JSON payload rules |
| Realtime validate | Max 128000 bytes; appId bind |
| LiveKit pub/sub | Topics `webxdc`, `webxdc-rt` |
| Chrome | info/summary/document; Start; link confirm for external URLs |
| `selfAddr` / `selfName` | Per-app opaque addr; display name |
| RBAC | close / upload; ignore forged control from non-mods |
| Lifecycle | destroy iframe, leave RT channel, drop listeners |

---

## UI placement

- Apps panel: icon (`icon.png`/`jpg`), `manifest.toml` name, summary/document.
- Activity: `info` lines; click → start app + optional `href`.
- Label chrome: “Mini-app (untrusted)”.
- External links inside app: intercept → confirm full URL (spec 1.3).

---

## What must never live in the iframe

- LiveKit room/tokens, JWT, parent stores  
- Ability to publish arbitrary LiveKit topics  
- Host bridge replaced by ZIP’s `webxdc.js`  

---

## Optional later

- Server-side status log (serial authority C)  
- `sendToChat` → meeting chat draft  
- `importFiles` → picker + recent uploads  
- Yjs guidance for app authors (base64 in payload); not required in host core  
