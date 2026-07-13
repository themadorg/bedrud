# 07 — Official WebXDC Spec Mapping (from `context/webxdc-website`)

Source: cloned [webxdc/website](https://github.com/webxdc/website) → `context/webxdc-website/src-docs/`  
(see [`context/README.md`](../../../context/README.md)).

This document maps **messenger/host requirements and JS API** to Bedrud decisions so we implement a real host, not a guess.

---

## 1. Product model (instances)

From `spec/messenger.md`:

| Spec | Bedrud mapping |
|------|----------------|
| A `.xdc` is attached to a **chat message**; starting it creates a webxdc app instance | A **room app instance** (`appId`) created when a package is opened/shared in a meeting |
| Same container file shared **twice** ⇒ **two isolated apps** (no cross-talk) | Two `appId`s even if same package bytes/hash |
| Instances must not know about each other | Separate iframe, storage origin key, LiveKit sub-routing by `appId` |

---

## 2. Container format (`spec/format.md`)

| Requirement | Host action |
|-------------|-------------|
| ZIP with extension `.xdc` | Accept upload; validate ZIP |
| Compression Deflate or Store (RFC 1950 defaults) | Reject exotic methods if library allows |
| MUST contain `index.html` | Reject otherwise |
| MAY contain `manifest.toml` | Parse `name`, `source_code_url` for UI |
| MAY contain `icon.png` / `icon.jpg` (square ~128–512) | Show in Apps panel; fallback icon |
| Initial open: root URL with `index.html` | `…/index.html` (or rewrite `/` → index) |

Example app packaging (docs): zip directory contents so `index.html` is at ZIP root.

---

## 3. `webxdc.js` is provided by the host (`spec/api.md`)

> `webxdc.js` **must not** be added to your `.xdc` file as they are provided by the messenger.

| Spec | Bedrud |
|------|--------|
| Apps include `<script src="webxdc.js"></script>` | When serving the package, **also** serve host-controlled `webxdc.js` at the app origin path (e.g. virtual file not from ZIP, or ZIP path overridden by host) |
| Host implements full JS API behind that script | Trusted bootstrap only ([02](./02-security-sandbox-csp.md) H4) |
| Dev simulators: hello / webxdc-dev | Optional local dev; production never trusts ZIP’s copy if present (prefer host override) |

**Implementation note:** If a malicious ZIP ships its own `webxdc.js`, the host **must win** (serve host file first / strip entry / inject before app scripts).

---

## 4. Messenger MUST list (`spec/messenger.md`) → Bedrud checklist

| MUST | Bedrud plan |
|------|-------------|
| Serve **all** resources from container | Zipfs only ([02](./02-security-sandbox-csp.md)) |
| Open with `index.html` | Asset router |
| Implement API + serve `webxdc.js` | Trusted bridge |
| **Deny all forms of Internet access** | Layered isolation; see residual WebRTC risk |
| If allowing click-out to `http(s)` links | **Confirm dialog** with **full URL** + privacy warning ([CHANGELOG 1.3](../../../context/webxdc-website/src-docs/spec/CHANGELOG.md)) — never silent open |
| Support `localStorage`, `sessionStorage`, `indexedDB` | Requires stable origin per app instance → pushes toward dedicated origin or careful null-origin alternatives |
| **Isolate storage/state per webxdc app** | Per-`appId` origin path or partitioned storage key |
| `visibilitychange` | Browser default in iframe |
| `window.navigator.language` | Browser default |
| `window.location.href` (scheme/domain unspecified) | Do not hardcode assumptions in apps; host may use opaque URL |
| Local HTML links `<a href="localfile.html">` | Zipfs relative paths |
| `mailto:` links | Parent intercept or allow with OS handler — decide; no network to web |
| `<meta name="viewport">` | Passthrough |
| `<input type="file">` | Allowed; pairs with `importFiles` |

### UI in “chat” (meeting chrome)

| Spec SHOULD/MUST | Bedrud meeting UI |
|------------------|-------------------|
| Show `update.info` text; tap opens app (+ `href` navigation) | Meeting activity / system line; open panel to app + navigate iframe |
| Show latest `document` + `summary` beside icon | Apps list row / stage chip |
| **Start** button; no cookie consent needed (no implicit net) | Open / Start control |

---

## 5. Status updates API (durable-ish shared state)

### `sendUpdate(update, descr?)` — `spec/sendUpdate.md`

| Field | Type / rules | Host behavior |
|-------|----------------|---------------|
| `payload` | JSON-serializable; **not** `undefined`; no raw binary (use base64) | Validate serializable; size ≤ max |
| `info` | Optional short chat line; no linebreaks; ~50 chars truncate; don’t spam | Show as text in meeting; strip/truncate |
| `href` | Optional **relative** URL (e.g. `index.html#about`) | On open-from-info, load root + href |
| `document` | Optional doc title; ~20 chars | Chrome only |
| `summary` | Optional aggregate; ~20 chars | Chrome only |
| `notify` | Optional map `address → text`; `"*"` catch-all | Map to desktop/browser notifications for matching `selfAddr` only |
| `descr` arg | **Deprecated**; apps pass `""` | Accept and ignore |

**Limits (host SHOULD expose; defaults if not):**

| Property | Default | Bedrud |
|----------|---------|--------|
| `webxdc.sendUpdateInterval` | **10000** ms | Expose on `window.webxdc`; bridge may delay faster callers |
| `webxdc.sendUpdateMaxSize` | **128000** bytes | Enforce on serialized update; expose to apps |

These defaults replace earlier ad-hoc 64 KiB ideas where they conflict — **prefer official 128000** unless LiveKit forces lower (then expose the true max).

### `setUpdateListener(cb, serial?)` — `spec/setUpdateListener.md`

| Spec | Bedrud |
|------|--------|
| Callback receives own + remote updates | Local echo + LiveKit |
| `serial` > 0, increasing; **gaps allowed** | Host assigns monotonic serial per app instance |
| `max_serial` on each update; equal ⇒ caught up for now | Set correctly on delivery |
| Returns **Promise** resolved when all known updates at subscribe time processed | Implement |
| Multiple `setUpdateListener` = undefined; only last works | Document; last wins |

### Delivery semantics (`faq/storage.md`)

- **No** guaranteed delivery; **no** delivery ACKs to the app.
- Updates may be **reordered** or **lost**.
- Safe pattern for apps: send full state or CRDT merges (Yjs + base64) — see `shared_state/practical.md`.
- Host persists updates “like messenger” for multi-device; Bedrud v1 may be room-session + peer snapshot with honest “best effort” (same as chat DC).

---

## 6. Realtime channel vs status updates (important correction)

Earlier plan drafts used LiveKit primarily for `sendUpdate`. The official model has **two** layers:

| API | Semantics | Natural Bedrud transport |
|-----|-----------|---------------------------|
| `sendUpdate` / `setUpdateListener` | Application **status** updates; intended to be **queued/persisted** by messenger; catch-up via serial | LiveKit **reliable** topic `webxdc` **or** server-backed log later; must support serial replay |
| `joinRealtimeChannel()` (**experimental**) | **Ephemeral** binary (`Uint8Array` ≤ 128000); only currently connected peers; no late join | LiveKit data channel topic e.g. `webxdc-rt` (reliable or lossy TBD); perfect fit |

From `spec/joinRealtimeChannel.md`:

- Private to chat, isolated per app, ephemeral.
- Second `joinRealtimeChannel` without `leave()` throws.
- Feature-detect: `window.webxdc.joinRealtimeChannel !== undefined`.
- `setListener` / `send` / `leave`.

**Bedrud recommendation:**

1. **v1:** Implement full `sendUpdate` path (what most simple apps need).
2. **v1.1 or same PR if cheap:** Implement `joinRealtimeChannel` on a separate LiveKit topic for games/cursors-style traffic without polluting the serial log.
3. Do **not** treat realtime messages as status updates (no serial, no snapshot into status log).

---

## 7. Identity (`spec/selfAddr_and_selfName.md`)

| Property | Spec | Bedrud |
|----------|------|--------|
| `selfName` | Display nick | Meeting display name |
| `selfAddr` | **Unique per user per webxdc app instance**; stable across restarts/devices for **that app**; **different** for different apps (anti-linkability); **not** shown in UI | Derive opaque id: e.g. `hmac(server_secret, userId + appId)` or `hash(roomId + appId + userId)` — **not** raw email; **not** global LiveKit identity alone if that links apps |

`notify` keys use these addresses. Host notifications must only fire for the local user’s `selfAddr` or `"*"`.

---

## 8. Optional APIs (phase later)

| API | Spec role | Bedrud mapping |
|-----|-----------|----------------|
| `sendToChat` | User confirms; may draft file+text to a chat; may exit app | Open meeting chat composer with attachment; user sends |
| `importFiles` | Picker + recent chat attachments | File input + optional recent chat uploads |
| `getAllUpdates` | **Deprecated** | Stub or omit |

---

## 9. Yjs / shared state (`shared_state/practical.md`)

- Yjs updates are `Uint8Array` → must **base64** (or similar) inside `sendUpdate` payload (status path).
- Existing [y-webxdc](https://codeberg.org/webxdc/y-webxdc) provider is for messengers; Bedrud can later offer a LiveKit-backed provider **outside** the iframe or document how apps use stock `sendUpdate`.
- Bedrud’s own whiteboard already uses Yjs over LiveKit in the **trusted** parent — keep that separate from untrusted mini-apps.

---

## 10. Example minimal app (from `get_started.md`)

Hosts must run apps shaped like:

```html
<script src="webxdc.js"></script>
<script>
  window.webxdc.sendUpdate({ payload: msg }, "");
  window.webxdc.setUpdateListener((update) => { /* update.payload */ }, 0);
</script>
```

Compatibility target: this pattern + common store apps without Bedrud-specific forks.

---

## 11. Spec version awareness

| Doc version notes | Impact |
|-------------------|--------|
| 1.2: `notify`, `href`, limits, per-app `selfAddr` | Implement for modern apps |
| 1.3: external link confirmation | Host link policy |
| `webxdc.d.ts` in website repo | May lag; prefer markdown + CHANGELOG over stale d.ts |

Track implemented surface in code comments / `packages` types later (`@bedrud/webxdc-types` or re-export upstream `@webxdc/types` if used).

---

## 12. Implications that revise earlier plan bullets

| Earlier plan idea | Spec-informed revision |
|-------------------|------------------------|
| LiveKit only as generic “webxdc updates” | Split **status** (serial, catch-up) vs **realtime** (ephemeral binary) |
| Max payload ~64 KiB | Prefer **128000** serialized update size (official default) |
| `descr` in API examples | Deprecated; use `""` |
| Identity = LiveKit id in iframe | Use **per-app** `selfAddr`; LiveKit id only for transport auth on parent |
| Storage optional | Host **MUST** support web storage APIs + **per-app isolation** |
| Block all navigations forever | Allow **confirmed** external http(s) opens only |
| `webxdc.js` injection optional style | **Required** host-provided module at `webxdc.js` |

Full security model remains in [02](./02-security-sandbox-csp.md); this file is the **compatibility** contract with upstream docs.
