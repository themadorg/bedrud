# 08 — Lessons from Delta Chat Desktop (host reference)

Source: local clone [`context/deltachat-desktop`](../../../context/README.md)  
([github.com/deltachat/deltachat-desktop](https://github.com/deltachat/deltachat-desktop))

Primary references in that tree:

| Path | Role |
|------|------|
| `docs/WEBXDC.md` | Official desktop architecture write-up |
| `packages/target-electron/src/deltachat/webxdc.ts` | Host controller (~1.3k lines): session, CSP, protocol, IPC |
| `packages/target-electron/static/webxdc-preload.js` | `window.webxdc` API + realtime listener |
| `packages/frontend/src/system-integration/webxdc.ts` | Core events → open / status / realtime / delete |
| `packages/target-electron/src/deltachat/link-clicks.ts` | External URL handling patterns |
| `packages/target-electron/src/index.ts` | `host-resolver-rules`, `webxdc` scheme privileges |

Electron/Tauri specifics do not transfer 1:1 to Bedrud’s browser+iframe model, but **security layering, protocol design, and API bootstrap** are highly transferable.

---

## 1. Architecture (Desktop) vs Bedrud

```
Delta Chat Desktop                         Bedrud (planned)
─────────────────                          ────────────────
Main process (Node)                        Go API + React meeting SPA
  BrowserWindow per app instance             iframe per app instance
  session.fromPartition(...)                 dedicated origin / partition strategy
  protocol webxdc://account.msg.webxdc/      HTTP zipfs under webxdc host
  preload + contextBridge                    host webxdc.js + postMessage
  IPC → JSON-RPC core                        LiveKit (+ optional Go status log)
  Core persists status updates               Room session / peer snapshot / Go log
```

**Instance id:** Desktop uses `{accountId}.{messageId}` as hostname and open-app key.  
**Bedrud:** `{roomId}.{appInstanceId}` (message ↔ room app instance).

---

## 2. What Desktop does well (adopt)

### 2.1 Custom origin scheme with instance in the hostname

```
webxdc://{accountId}.{messageId}.webxdc/index.html
webxdc://{accountId}.{messageId}.webxdc/webxdc.js   ← host-generated, not from ZIP
```

- Hostname encodes **which app instance** may be served.
- Protocol handler **refuses** requests if that instance is not in `open_apps`.
- Blobs loaded only via core `getWebxdcBlob(accountId, msgId, filename)` — no raw FS path to the ZIP.

**Bedrud analogue:**  
`https://webxdc.<host>/r/{roomId}/a/{appId}/…` with auth + appId binding on every asset request. Never serve package A bytes under package B’s URL.

### 2.2 Host-owned `webxdc.js` is a tiny bootstrap

Desktop does **not** embed the full API in the script tag response. Flow:

1. Preload (`contextIsolation: true`) defines `webxdc_internal.setup` + full API via Electron IPC.
2. Request for `webxdc.js` returns only:

   ```js
   window.webxdc_internal.setup("<base64 selfAddr>", "<base64 selfName>", interval, maxSize, isAppSender, isBroadcast)
   ```

3. Setup exposes `window.webxdc` once.

**Bedrud analogue:**

- Serve trusted `webxdc.js` that talks to parent via `postMessage` (not IPC).
- Optional: short bootstrap URL with config, or inject config on first parent→iframe `init` message.
- **Always** intercept path `webxdc.js` so ZIP cannot supply the bridge (Desktop pattern + our H4).

### 2.3 Multi-layer network kill-switch (not CSP alone)

Desktop stacks:

| Layer | Mechanism |
|-------|-----------|
| Chromium DNS | Global `--host-resolver-rules=MAP * ^NOTFOUND` (blocks DNS for apps; careful: process-wide on Electron) |
| Proxy blackhole | Per webxdc session SOCKS5 to local TCP listener that **destroys** sockets |
| webRequest | Cancel all URLs except `webxdc://` (and optional `https` for integrated apps, `devtools` if enabled) |
| CSP | Includes `webrtc 'block'`; `connect-src 'self' data: blob:` (self = custom protocol only) |
| WebRTC policy | `setWebRTCIPHandlingPolicy('disable_non_proxied_udp')` so WebRTC would need the dead proxy |
| Permissions | Allowlist: **only** `pointerLock`, `fullscreen` |

**Pentest notes hard-coded in source:**

- **XDC-01-002:** CSP must be on **every** response (including 404). Missing CSP on error responses → iframe without CSP → parent breakout.  
  → Bedrud: middleware must set CSP on **404/500** asset responses too (`makeResponse` pattern).
- **XDC-01-005:** PDF MIME opens Chromium PDF viewer → CSP bypass path. Desktop clears/overrides `application/pdf`.  
  → Bedrud: never serve `application/pdf` as navigable type from zipfs (force `application/octet-stream` or download).

**Bedrud browser stack (best effort):**

1. Strict CSP on **all** status codes (`default-src` tight; `webrtc 'block'`).
2. Permissions-Policy / iframe `allow=""`.
3. Separate origin from SPA.
4. Optional Service Worker / parent navigation interceptor for external URLs.
5. Automated tests with [webxdc-test](https://github.com/webxdc/webxdc-test) (Desktop’s recommended suite).

We **cannot** ship process-wide DNS MAP in a multi-tenant browser tab; document residual risk vs native hosts.

### 2.4 Status updates are core-persisted; UI only notifies

- `sendUpdate` → `rpc.sendWebxdcStatusUpdate(accountId, msgId, update, '')`
- `getAllUpdates` / listener path → `rpc.getWebxdcStatusUpdates(accountId, msgId, serial)`
- When core gets a new status: frontend event `WebxdcStatusUpdate` → main sends `webxdc.statusUpdate` to the open window → preload pulls updates since `last_serial`

**Implications for Bedrud:**

- Serial authority lives in **core** (not each peer inventing global serials).
- Open windows are **prodded** to pull; pull is authoritative.
- Prefer **Go (or single room authority) status log** for correct `serial` / `max_serial` rather than pure peer gossip long-term ([03](./03-realtime-bridge-architecture.md)).

### 2.5 Realtime is separate IPC + core

- `joinRealtimeChannel` → advertise + `sendRealtimeData` / `leaveRealtimeChannel`
- Core pushes `WebxdcRealtimeData` → `webxdc.realtimeData` to window
- Preload enforces `Uint8Array`, single active listener, trash after leave

Matches our `webxdc` vs `webxdc-rt` split.

### 2.6 Single window per instance; focus + href navigation

- Re-open focuses existing window.
- Info-message `href`: resolve as **relative** against dummy base, then `appURL + relative` (pathname+search+hash only).

**Bedrud:** one iframe per `appId`; activity-line click focuses panel and navigates iframe to relative `href` only.

### 2.7 Permissions allowlist (not denylist)

Desktop allows **only** pointer lock + fullscreen. Everything else denied and rate-limited in logs.

**Bedrud:** empty iframe `allow` + Permissions-Policy deny-all; only add features with explicit product need.

### 2.8 Lifecycle cleanup

- Instance deleted / chat deleted → close window, clear session storage data, mark partition for cleanup.
- TODO in Desktop: close on message deletion + wipe DOM storage (still tracked).

**Bedrud:** on room leave / app delete / force-close → destroy iframe, drop logs, clear partitioned storage if any.

### 2.9 Link handling

- Relative `webxdc://` navigations allowed inside app.
- External http(s): confirm / open externally (`openExternalHttpOrPromptToCopy`).
- Scheme whitelist for other schemes; else prompt copy.

**Bedrud:** parent intercepts `window.open` / top navigations / click on absolute URLs → confirm dialog (spec 1.3).

### 2.10 Use official types package

Desktop documents [`@webxdc/types`](https://www.npmjs.com/package/@webxdc/types). Prefer that over stale `webxdc.d.ts` in the website repo.

---

## 3. Desktop CSP (actual string)

```text
default-src 'self';
style-src 'self' 'unsafe-inline' blob: ;
font-src 'self' data: blob: ;
script-src 'self' 'unsafe-inline' 'unsafe-eval' blob: ;
connect-src 'self' data: blob: ;
img-src 'self' data: blob: ;
media-src 'self' data: blob: ;
webrtc 'block'
```

Notes for Bedrud:

| Desktop choice | Our earlier plan | Recommendation |
|----------------|------------------|----------------|
| `connect-src 'self' data: blob:` | `connect-src 'none'` | Prefer **`'none'`** in browser host if apps don’t need blob XHR; Desktop’s `'self'` is safe only because custom protocol + request cancel. In HTTP zipfs, `'self'` allows fetch to own origin (OK for assets) but not third parties if CSP holds. **`none` is stricter** for XHR/WS. |
| `unsafe-eval` | avoided | Avoid unless a real app needs it; Desktop allows for compat — track as residual. |
| `blob:` in script/style | allowed | Match Desktop for offline blob workers/URLs used by apps; keep `worker-src` tight if we set it. |
| CSP on **all** responses | we said assets | **Must include error bodies** (XDC-01-002). |

---

## 4. What Desktop has that Bedrud should phase

| Feature | Desktop | Bedrud phase |
|---------|---------|--------------|
| Core status persistence + multi-device | Yes | v1 peer/Go log; harden toward server serials |
| Realtime channel | Yes | P1 after status |
| `sendToChat` / `importFiles` | Yes | P3 |
| Integrated internet apps (maps) | Proxy via core `getHttpResponse` | **Out of scope v1** — do not open internet for arbitrary `.xdc` |
| Window bounds persistence | Yes | N/A (panel size optional) |
| `isAppSender` / `isBroadcast` | Core flags | If needed for API parity later |
| Drag-file-out | Desktop-only | Skip |
| Tauri host | Separate implementation | Out of scope |

---

## 5. Concrete Bedrud design adjustments (from this review)

1. **`makeResponse` discipline** — one helper sets CSP + `nosniff` + safe MIME for every zipfs response including 404/500.
2. **PDF / dangerous MIME denylist** — force non-viewer types.
3. **Instance-bound asset URLs** — room + appId in path; 403 if mismatch.
4. **Host `webxdc.js` bootstrap** — Desktop-style setup with `selfAddr`/`selfName`/limits; API implementation in trusted JS.
5. **Pull model for status** — after LiveKit nudge (“status available”), client may fetch from Go log `GET …/updates?after=serial` **or** apply DC payload; long-term prefer Go log like core.
6. **Test with webxdc-test** — add to [06](./06-verification-checklist.md).
7. **Do not** implement “internet access for arbitrary apps”; maps-style proxy is a separate trusted product.
8. **Permission allowlist mindset** — games may later need fullscreen inside iframe; default deny.
9. **Relative href only** — same dummy-base URL parse as Desktop.
10. **Open-once** — focus existing iframe/panel instead of second instance.

---

## 6. Mapping Desktop IPC → Bedrud channels

| Desktop IPC / event | Bedrud |
|---------------------|--------|
| `webxdc.sendUpdate` | postMessage → bridge → LiveKit `webxdc` and/or `POST` status log |
| `webxdc.getAllUpdates` | parent serial store / `GET` status log |
| `webxdc.statusUpdate` (push) | LiveKit notify or poll; then deliver to listener |
| `webxdc.sendRealtimeData` | LiveKit `webxdc-rt` |
| `webxdc.realtimeData` | bridge → iframe |
| `webxdc.sendToChat` | open meeting chat composer |
| `webxdc:instance-deleted` | destroy iframe + clear state |
| Core `WebxdcInfoMessage` + href | meeting activity line + navigate |

---

## 7. Residual: browser host vs Electron

| Capability | Electron Desktop | Bedrud browser |
|------------|------------------|----------------|
| Separate OS process | Yes | No (iframe only) |
| Session partition files | Yes | Origin / Storage keys |
| DNS poison entire renderer | Yes | No |
| Blackhole proxy | Yes | No |
| contextBridge | Yes | postMessage + no shared JS heap if cross-origin |
| Preload without page script | Yes | Host script as first resource |

Therefore Bedrud’s **origin isolation + CSP-on-all-responses + no secrets** remain mandatory; Desktop’s extra OS layers are an upper bound we document as residual risk ([02 §9](./02-security-sandbox-csp.md)).

---

## 8. Suggested reading order for implementers

1. `context/deltachat-desktop/docs/WEBXDC.md`
2. `webxdcProtocolHandler` + `makeResponse` in `webxdc.ts`
3. `createSessionIfNotExists` (proxy + webRequest)
4. `static/webxdc-preload.js` (API surface)
5. This plan’s [02](./02-security-sandbox-csp.md), [03](./03-realtime-bridge-architecture.md), [07](./07-official-spec-mapping.md)
