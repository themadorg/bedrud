# 06 — Verification Checklist

Confirm Bedrud’s WebXDC host matches the **official security spirit** and this plan’s threat model ([02](./02-security-sandbox-csp.md)).

Use a **malicious fixture** `.xdc` (intentionally hostile scripts) in addition to a friendly demo app.

---

## A. Network isolation (no phone-home)

- [ ] `fetch('https://example.com')` fails in iframe.
- [ ] `XMLHttpRequest` to external host fails.
- [ ] `new WebSocket('wss://example.com')` fails.
- [ ] `navigator.sendBeacon('https://example.com', 'x')` fails or does not deliver.
- [ ] `<img src="https://example.com/pixel.png">` does not load (CSP).
- [ ] External `<script src="https://…">` does not execute.
- [ ] `new RTCPeerConnection()` / offer flow cannot be used as an open network channel (or residual risk signed off per browser matrix).
- [ ] CSP asset responses include `connect-src 'none'` (no draft-only `webrtc` directive — avoids console noise).
- [ ] Network panel (iframe context): only webxdc-origin document + same-origin ZIP assets (+ host `webxdc.js`).
- [ ] Clicking an in-app `https://` link does **not** navigate silently; confirm UI shows **full URL** + privacy warning (or link is blocked).

---

## B. Origin / sandbox isolation (no SPA breakout)

- [ ] Mini-app origin is **not** the meeting SPA origin **or** sandbox omits `allow-same-origin`.
- [ ] `parent.document` / `top.document` access fails.
- [ ] SPA `localStorage` / `sessionStorage` / cookies **not** readable from iframe.
- [ ] Iframe cannot remove its `sandbox` attribute effectively against the parent.
- [ ] `blob:` / `srcdoc` untrusted HTML is **not** used on SPA origin with both allow flags.
- [ ] Sandbox string contains **no** `allow-popups`, `allow-top-navigation*`, `allow-forms`, `allow-modals` (unless explicitly reviewed exception).
- [ ] `allow=""` (or equivalent) denies camera, microphone, geolocation, display-capture.
- [ ] Permissions-Policy header on app origin denies powerful features.

---

## C. Secrets & authz

- [ ] JWT / session token never appears in iframe URL, storage, or JS globals.
- [ ] LiveKit token never injected into iframe.
- [ ] Asset GET without room auth returns 401/403.
- [ ] Asset GET for room A package using room B credentials fails.
- [ ] Upload denied for unauthorized roles (default: non-mod cannot upload).
- [ ] No `Access-Control-Allow-Origin: *` on private app assets.
- [ ] Long-lived secrets not passed as query parameters in iframe `src`.

---

## D. Host API / bridge (spec surface)

- [ ] App only uses `window.webxdc.*` (no LiveKit in iframe).
- [ ] Request for `webxdc.js` is **host** implementation even if ZIP contains `webxdc.js`.
- [ ] get_started-style sample: `sendUpdate` + `setUpdateListener(..., 0)` works.
- [ ] `selfAddr` differs across two app instances for the same user; stable on reopen of same instance.
- [ ] `selfName` is display name; `selfAddr` not shown as primary UI identity.
- [ ] `sendUpdateMaxSize` / `sendUpdateInterval` exposed (defaults 128000 / 10000).
- [ ] Oversize update rejected; rapid updates delayed/coalesced per interval policy.
- [ ] Received updates include `serial` and `max_serial`; listener Promise resolves after catch-up.
- [ ] Optional: `joinRealtimeChannel` isolated per app; leave required before re-join; max 128000 bytes.
- [ ] `postMessage` ignored when `event.source` ≠ iframe window or wrong origin/channel.
- [ ] Cross-`appId` publish attempt from iframe A dropped.
- [ ] Parent renders `info` as plain text only (XSS attempt in `info` fails).
- [ ] `targetOrigin` is never `*` in production builds.
- [ ] `href` open navigates only relative paths under app root.

---

## E. Peer / LiveKit abuse

- [ ] Spoofed JSON `sender` does not override LiveKit participant identity in trusted UI.
- [ ] Random participant’s `control: close` is ignored if not mod/owner.
- [ ] Oversized `snapshot` dropped.
- [ ] Peer cannot force-open an app iframe without local UX policy.
- [ ] Topic confusion: packets on other topics do not enter webxdc bridge.
- [ ] Realtime (`webxdc-rt`) traffic does not enter the status serial log.

---

## F. Package handling

- [ ] Valid `.xdc` with `index.html` runs.
- [ ] Optional `manifest.toml` name / `source_code_url` and `icon.png`/`icon.jpg` used in UI when present.
- [ ] Zip-slip (`../`, absolute paths) rejected.
- [ ] Oversize archive / entry / entry-count rejected.
- [ ] Missing `index.html` rejected.
- [ ] Unknown extension served as non-executable type (`nosniff`).
- [ ] Response headers: CSP, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`.
- [ ] Two instances of the same package do not share storage or update streams.

---

## G. Collaboration functional

- [ ] Client A `sendUpdate` → Client B listener receives same payload.
- [ ] Topic is `webxdc`, reliable (status).
- [ ] Local apply is single (no double UI glitch).
- [ ] Second `appId` isolation holds.
- [ ] Late joiner status snapshot or empty state works; realtime stays empty until new RT messages.
- [ ] Mod close tears down iframes for authorized close only.
- [ ] Reconnect re-binds bridge.

---

## H. Regression

- [ ] Meeting chat works.
- [ ] Whiteboard works.
- [ ] Presence / stage unaffected.
- [ ] `bun run check` and `go test` pass for touched packages.

---

## I. Manual DevTools recipe (hostile)

1. Load **malicious fixture** app in a real meeting.
2. Console (iframe): attempt `fetch`, `WebSocket`, `parent.document`, `localStorage` of parent origin, `RTCPeerConnection`.
3. Application panel: confirm cookie jar isolation.
4. From a second tool or console on parent: `window.postMessage(fake, '*')` — bridge must ignore.
5. Flood `sendUpdate` — rate limit; SPA remains responsive.
6. Friendly demo: two browsers still sync.

---

## J. Browser matrix (record results)

| Browser | fetch blocked | WS blocked | RTCPeerConnection blocked | parent DOM blocked | Notes |
|---------|---------------|------------|---------------------------|--------------------|-------|
| Chromium | | | | | |
| Firefox | | | | | |
| Safari | | | | | |

Any “no” under network columns → residual risk entry in [02 §9](./02-security-sandbox-csp.md) before calling v1 complete.

---

## K. Desktop-parity host checks (from [08](./08-deltachat-desktop-host.md))

- [ ] `webxdc.js` path never returns ZIP bytes.
- [ ] 404/500 asset responses still include full CSP + `nosniff`.
- [ ] `application/pdf` not used for zip entries (no PDF viewer CSP bypass).
- [ ] Asset URL for app A cannot fetch blob of app B.
- [ ] Second Start focuses existing instance (no duplicate iframe).
- [ ] Relative `href` navigation works; absolute `href` rejected.
- [ ] Status serials come from host log (or documented interim gap).
- [ ] [webxdc-test](https://github.com/webxdc/webxdc-test) exercised or explicitly deferred with ticket.

---

## L. Spec alignment summary

| Expectation | Pass? | Evidence |
|-------------|-------|----------|
| No internet from mini-app | | |
| Origin / sandbox isolation | | |
| Serve only from `.xdc` + host `webxdc.js` | | |
| Communication via webxdc API + host | | |
| No secrets in iframe | | |
| Status serials + optional realtime channel | | |
