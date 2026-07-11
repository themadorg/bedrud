# 02 — Security Model (Threat Model, Sandbox, CSP)

**Status:** Security-critical. Implementation must not ship without the **hard requirements** in §0 and the verification items in [06](./06-verification-checklist.md).

---

## Official expectation

From the WebXDC get-started philosophy ([webxdc.org/docs](https://webxdc.org/docs/get_started.html)):

> Mini apps have **no internet access** and run in a **sandboxed web view**, which means they can never track users or leak data to third parties.

Native hosts (Delta Chat, etc.) enforce **network-isolated webviews**. Audits (e.g. Open Tech Fund / Delta Chat WebXDC pentest, 2023) emphasize CSP **plus** platform-specific network blocks; Chromium historically has gaps (notably **WebRTC** not fully governed by classic CSP).

Bedrud is a **browser-hosted** web host. We must achieve the **same privacy promise** with layered controls, not a single CSP line.

---

## 0. Hard requirements (non-negotiable for v1 ship)

| # | Requirement | Why |
|---|-------------|-----|
| H1 | **Cross-origin (or opaque) isolation** between meeting SPA and mini-app document | `sandbox="allow-scripts allow-same-origin"` on a **same-origin** iframe is **not a sandbox**: the frame can remove `sandbox` and access parent DOM/storage/cookies. |
| H2 | **No network from mini-app JS** | Core WebXDC promise; blocks tracking and exfiltration. |
| H3 | **Assets only from validated `.xdc` ZIP** (+ host `webxdc.js`) | Spec: all resources from container; host additionally serves `webxdc.js`. No CDN. |
| H4 | **Host provides trusted `webxdc.js` only** | Spec: apps load `<script src="webxdc.js">` but **must not** ship it in the ZIP; messenger provides it. Host file **always wins** over any ZIP entry. |
| H5 | **No secrets in the iframe** | No JWT, LiveKit token, refresh token, admin cookie, or parent store. |
| H6 | **Strict message authentication at the bridge** | Origin allowlist, channel marker, size limits, schema validation; never `eval` / `new Function` on payloads. |
| H7 | **Defense in depth** | Sandbox attributes + CSP + Permissions-Policy + separate origin + authz + size/rate limits. One layer failing must not equal full breakout. |

**Do not ship** a prototype that serves `.xdc` from the **same origin** as the React SPA with `allow-scripts allow-same-origin`. That configuration is equivalent to running untrusted meeting code as first-party XSS.

---

## 1. Threat model

### Assets

| Asset | Sensitivity |
|-------|-------------|
| Meeting JWT / session cookie | Critical |
| LiveKit join tokens | Critical |
| Other participants’ media / chat / whiteboard | High |
| User identity (email, display name) | Medium–high |
| Room membership & private room `.xdc` packages | Medium–high |
| Mini-app collaborative state | App-dependent (may include user-entered secrets) |

### Trust boundaries

```
┌──────────────────────────────────────────────────────────┐
│  Trusted: Bedrud SPA + Go API + LiveKit (after auth)     │
├──────────────────────────────────────────────────────────┤
│  UNTRUSTED: entire contents of any uploaded .xdc          │
│  (HTML/JS/CSS may be fully malicious)                     │
├──────────────────────────────────────────────────────────┤
│  UNTRUSTED: peer-sourced webxdc update payloads           │
│  (any room member can craft envelopes)                    │
└──────────────────────────────────────────────────────────┘
```

Assume **every uploaded mini-app is hostile**. Assume **any participant may send hostile `webxdc` data-channel messages**.

### Adversaries & goals

| Adversary | Goals |
|-----------|--------|
| Malicious `.xdc` author / uploader | XSS into meeting, steal tokens, track users, phone home, use cam/mic, mine crypto, phishing UI |
| Malicious room participant | Flood updates DoS, spoof identity in app, inject HTML if host mishandles `info`, force-close abuse |
| External network attacker | SSRF via host if host proxies; cache poisoning of app assets |
| Curious peer | Read another room’s `.xdc` if authz broken |

### Explicit non-goals of the sandbox

- Stopping a **malicious peer** from sending garbage *into* a correctly isolated app (apps must tolerate bad payloads).
- Stopping social-engineering UI *inside* the iframe that only affects that iframe’s pixels (mitigate with chrome UI that labels “untrusted mini-app”).
- Providing E2E encryption of WebXDC updates beyond whatever LiveKit/room transport already provides (host relay model).

---

## 2. Origin isolation strategy (H1) — pick one before build

### Ranked options (security-first)

| Rank | Strategy | Isolation quality | Notes |
|------|----------|-------------------|-------|
| **1 (preferred)** | **Dedicated origin** for WebXDC assets, e.g. `webxdc.<host>` or path on a separate cookie-less host | Strong | Cookie jar separate if cookie `Domain` not shared; SPA cannot be reached via same-origin DOM. Serve only ZIP assets + CSP. |
| **2** | **Null-origin sandbox**: `sandbox="allow-scripts"` **without** `allow-same-origin`, load via `src` that ends up unique/opaque origin | Strong for parent | localStorage/origin storage limited; postMessage still works with careful origin checks (`ev.origin === "null"` cases). Harder for apps that assume origin storage. |
| **3** | Unique per-app origin (hash subdomain / path worker) | Strong | More ops complexity. |
| **Reject** | Same origin as SPA + `allow-scripts allow-same-origin` | **Broken** | Sandbox removable; full XSS. |
| **Reject** | `blob:` / `srcdoc` of untrusted HTML on parent origin + both allow flags | **Broken** | blob inherits creator origin. |

### Locked guidance for Bedrud

1. **Production target (preferred):** per-instance host  
   `https://webxdc-<instanceId>.{webxdc.baseDomain}/`  
   with DNS wildcard `*.{webxdc.baseDomain}` (e.g. `*.wx.example.com`).  
   Config + installer: [10](./10-config-and-installer.md). Feature gated by `webxdc.enabled`.
2. **Alternative:** single dedicated host + path isolation (still ≠ SPA origin).
3. **`allow-same-origin` is allowed only if** the iframe document’s origin is **not** the SPA origin. Then the flag stabilizes storage on the webxdc host only.
4. Same-origin path on the SPA host (`example.com/…-webxdc`) is **dev-only / rejected for production**.

### Cookie / storage isolation checklist

- [ ] WebXDC origin does **not** receive session cookies (`SameSite`, host-only cookies on SPA host, no parent Domain=).
- [ ] Mini-app cannot read `localStorage` / `IndexedDB` of the SPA origin.
- [ ] Mini-app cannot call `parent.document` / access parent JS heaps.

---

## 3. Network isolation (H2) — layered

CSP is **necessary but not sufficient** in Chromium-class engines.

### 3.1 Known gap: WebRTC

Delta Chat / WebXDC security writeups note that **classic CSP does not fully disable WebRTC**, and WebRTC can be abused as a network channel. W3C later defined a CSP approach (`webrtc` directive / block); **browser support is incomplete**.

| Control | Action |
|---------|--------|
| CSP | **Do not emit** `webrtc 'block'` in Bedrud — draft directive; many engines log *Unrecognized Content-Security-Policy directive 'webrtc'* and ignore it. |
| Permissions-Policy | Disable `camera`, `microphone`, `display-capture`, etc. on iframe (`allow` attribute empty / explicit deny). |
| Feature detection tests | Automated + manual: attempt `new RTCPeerConnection(...)` from iframe; document result per browser. |
| Future | If browsers ship a stable CSP webrtc (or equivalent) without console noise, re-evaluate; consider COOP/COEP only if it doesn’t break LiveKit on parent. |

**Honesty in product docs:** On pure web hosts, network isolation is “best-effort layered browser policy,” not a kernel network namespace. Aim for parity with messenger webviews; track residual risk in §9 residual risks.

### 3.2 Other exfil channels to block or reduce

| Channel | Mitigation |
|---------|------------|
| `fetch` / XHR / WebSocket / EventSource | `connect-src 'none'` |
| `<img src=https://…>`, `<link>`, `<script src=…>` | `default-src` / `img-src` / `script-src` `'self'` only; no https: |
| `navigator.sendBeacon` | Covered by connect-src in modern browsers — verify |
| CSS `url(https://…)` | `style-src` without remote; prefer no `*` |
| Fonts / workers / objects | Explicit `font-src`, `worker-src`, `object-src 'none'` |
| Forms | `form-action 'none'`; no `allow-forms` in sandbox |
| Top navigation / phishing | No `allow-top-navigation*`; `frame-ancestors` on assets |
| Prefetch / speculation | Avoid; CSP default restrictive |
| DNS prefetch | `<meta http-equiv>` / browser defaults; CSP helps for subresources |
| Service Worker | `worker-src 'none'`; do not grant SW control over webxdc origin without review |

---

## 4. Iframe element hardening

### Recommended attributes (webxdc origin **must ≠ SPA origin**)

```html
<iframe
  title="Untrusted WebXDC mini-app"
  sandbox="allow-scripts allow-same-origin"
  allow=""
  referrerpolicy="no-referrer"
  loading="lazy"
  src="https://webxdc.example.invalid/rooms/{roomId}/apps/{appId}/"
></iframe>
```

If there is **no** dedicated origin yet, use `sandbox="allow-scripts"` only (no `allow-same-origin`). Never combine both flags on the SPA origin.

| Attribute | Rule |
|-----------|------|
| `sandbox` | Minimal set. `allow-same-origin` **only** when iframe origin ≠ SPA. |
| `allow=""` | Empty Permissions Policy container: deny powerful features by default. |
| `referrerpolicy="no-referrer"` | Don’t leak room URLs/tokens via Referer. |
| **Forbidden sandbox tokens (v1)** | `allow-top-navigation`, `allow-top-navigation-by-user-activation`, `allow-popups`, `allow-popups-to-escape-sandbox`, `allow-forms`, `allow-modals`, `allow-pointer-lock`, `allow-orientation-lock`, `allow-presentation`, `allow-downloads` (unless later reviewed). |

### Permissions-Policy (response header on app origin **and** iframe `allow`)

```http
Permissions-Policy:
  accelerometer=(),
  autoplay=(),
  camera=(),
  display-capture=(),
  encrypted-media=(),
  fullscreen=(),
  geolocation=(),
  gyroscope=(),
  magnetometer=(),
  microphone=(),
  midi=(),
  payment=(),
  picture-in-picture=(),
  publickey-credentials-get=(),
  screen-wake-lock=(),
  usb=(),
  web-share=(),
  xr-spatial-tracking=()
```

Adjust only with explicit product need and security review (e.g. if a demo needs autoplay for local media blobs — still no mic/cam).

---

## 5. CSP for WebXDC asset responses

Apply on **every** document and subresource response from the WebXDC origin (prefer header over meta; meta cannot set `frame-ancestors`).

### Baseline policy (v1)

Inspired by Delta Chat Desktop’s CSP (`context/deltachat-desktop` / [08](./08-deltachat-desktop-host.md)), tightened where the browser host can afford it:

```http
Content-Security-Policy:
  default-src 'none';
  base-uri 'none';
  form-action 'none';
  frame-ancestors 'self';
  frame-src 'none';
  child-src 'none';
  object-src 'none';
  worker-src 'none';
  manifest-src 'none';
  media-src 'self' data: blob:;
  font-src 'self' data: blob:;
  img-src 'self' data: blob:;
  style-src 'self' 'unsafe-inline' blob:;
  script-src 'self' 'unsafe-inline' blob:;
  connect-src 'none'
```

Desktop also allows `script-src 'unsafe-eval'` and `connect-src 'self' data: blob:` because isolation is enforced outside CSP. **Bedrud defaults avoid `unsafe-eval`** and use `connect-src 'none'`. Revisit only if a required demo app fails.

| Directive | Rationale |
|-----------|-----------|
| `default-src 'none'` | Deny by default (stricter than `'self'` alone). |
| `script-src 'self' 'unsafe-inline'` | Compatibility with many `.xdc` apps; **cost:** inline XSS inside app only (contained if origin-isolated). Prefer nonces later if we rewrite HTML. |
| `connect-src 'none'` | Kill fetch/XHR/WS/EventSource. |
| *(no `webrtc`)* | Draft CSP only; unsupported browsers warn and ignore — see §3.1 residual WebRTC risk. |
| `frame-ancestors <SPA origin(s)>` | Only the meeting SPA embeds (not `'self'` on the webxdc host). |
| `base-uri 'none'` | Stronger than `'self'` against `<base>` attacks. |
| `object-src` / `worker-src` / plugins | Closed. |

**Do not** add `https:` or `*` to any fetch directive “for convenience.”

### Companion headers

```http
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
Cross-Origin-Resource-Policy: same-origin
X-Frame-Options: SAMEORIGIN
```

(Prefer CSP `frame-ancestors` as primary; XFO as legacy backup.)

### Content-Type discipline

| Rule | Detail |
|------|--------|
| Sniff-safe types | Map extensions → safe MIME; unknown → `application/octet-stream` (not executed). |
| **PDF never as `application/pdf`** | Desktop XDC-01-005: PDF viewer can bypass CSP. Force octet-stream / download. |
| HTML only for allowlisted paths | e.g. `index.html` and maybe html entries; never serve `.html` from user path traversal. |
| **CSP on 404/500 too** | Desktop XDC-01-002: missing CSP on error pages → iframe escape. Use a single `makeResponse`-style helper. |
| No `Access-Control-Allow-Origin: *` | Assets stay same-site to app origin. |
| Cache | `Cache-Control: private, no-store` for auth-gated apps (or short private cache with auth). |

### Desktop network layers we cannot fully replicate in-browser

Delta Chat Desktop also uses: SOCKS blackhole proxy per session, `webRequest` cancel-all, Chromium `host-resolver-rules=MAP * ^NOTFOUND`, and `WebRTCIPHandlingPolicy('disable_non_proxied_udp')`. Document residual risk in §9; compensate with origin isolation + tests ([08](./08-deltachat-desktop-host.md)).

---

## 6. Package validation (H3)

| Check | Limit (suggested defaults; tune in config) |
|-------|--------------------------------------------|
| Max archive size | e.g. 5–10 MiB |
| Max uncompressed total | e.g. 30 MiB |
| Max entry count | e.g. 500 |
| Max single file | e.g. 5 MiB |
| Zip-slip | Reject `..`, absolute paths, backslashes tricks, symlink entries if present |
| Required | `index.html` at package root (or documented WebXDC layout) |
| Banned | Nested `.xdc` execution, `service-worker` registration files if detectable |
| Optional static scan | Reject `http://` / `https://` in `src`/`href` of HTML at upload (heuristic; CSP remains authority) |

Store **original bytes** + server-side content hash. Serve only through the validated reader (zipfs), never extract to a world-readable directory with path control.

---

## 7. Host API & bridge security (H4–H6)

### Trusted bootstrap only (matches `spec/api.md` + `messenger.md`)

Official apps include:

```html
<script src="webxdc.js"></script>
```

| Approach | Security |
|----------|----------|
| Host **serves** `webxdc.js` from trusted Bedrud code at the app origin (zipfs interceptor: path `webxdc.js` never read from ZIP) | **Required** |
| Optionally rewrite `index.html` if script tag missing (compat) | OK |
| Using `webxdc.js` **from inside the ZIP** as the bridge | **Forbidden** — ZIP can ship a trojan `webxdc.js` |

Parent must treat **all** iframe messages as untrusted input even after providing the API.

### External links (spec CHANGELOG 1.3)

If the host allows opening `http:` / `https:` links from the mini-app (e.g. user clicked `<a href>`):

- MUST show the **full URL**
- MUST require **explicit confirmation**
- MUST warn that the link is external and may compromise privacy  
Silent navigation to the open web is a **spec + security** failure.

### Storage isolation (messenger MUST)

- Support `localStorage`, `sessionStorage`, `indexedDB` in the webview.
- Isolate storage **per webxdc app instance** (not shared with SPA or other `appId`s).
- This is a hard argument for a **stable dedicated origin per instance** (or equivalent partitioning).

### postMessage rules

```ts
// Parent
function onMessage(ev: MessageEvent) {
  if (ev.source !== iframe.contentWindow) return
  if (ev.origin !== expectedWebxdcOrigin) return // exact match; handle "null" if used
  if (!isWebxdcChannel(ev.data)) return
  if (!isValidSendUpdate(ev.data)) return // size, types, appId binding
  // never: eval, innerHTML of info string without escape, Function(payload)
  publishBounded(ev.data)
}
```

| Rule | Detail |
|------|--------|
| `event.source` | Must be the iframe’s `contentWindow`. |
| `event.origin` | Exact expected origin (not startsWith). |
| `targetOrigin` | Exact when posting **to** iframe; never `*` in production. |
| Size limits | e.g. max JSON 64–256 KiB per update (config). |
| `appId` binding | Message `appId` must match the iframe instance; client cannot open app A and publish as app B. |
| `info` / status strings | Treat as untrusted text in parent UI (React text nodes only). |
| Rate limit | **Required** (not optional): per-sender and per-app publish caps. |
| No capability escalation | Bridge methods are only WebXDC update/status/identity display fields — no `fetchProxy`, no `getToken`. |

### Identity fields (`selfAddr` / `selfName`)

| Prefer | Avoid |
|--------|--------|
| Stable LiveKit identity or opaque room participant id | Raw email if not already visible to all peers |
| Display name already shown in meeting roster | Auth provider subject, phone, IP |

Mini-apps inherit **room-visible** identity, not full account PII.

### LiveKit envelope trust

- Any room member can publish `topic: webxdc`.
- **Do not** trust `sender` / `senderName` fields inside JSON for security decisions; prefer LiveKit `participant.identity` from the data packet metadata.
- Overwrite or ignore spoofable identity inside payload when displaying “who sent”.
- Control actions (`close`) must check **moderator/owner** on the **receiving client** using meeting RBAC, not a boolean in the payload alone. Prefer also server-mediated control later.

---

## 8. Authorization (HTTP)

| Endpoint class | Authz |
|----------------|-------|
| Upload `.xdc` | Room member + policy (default: owner/mod; config for open rooms) |
| List / start apps | Room member / valid guest |
| GET assets on webxdc host | **Capability ticket** bound to instance (not SPA session cookie on parent domain) |
| Status log POST/GET | Room member / valid guest |
| Delete package / force close | Owner/mod/admin |

Additional:

- Guest tokens must be room-scoped per existing join rules.
- **Do not** put long-lived JWT/refresh tokens in iframe URLs. Use short-lived instance tickets ([11 §4](./11-api-schema-and-tickets.md)).
- CSP on all asset responses including 401/404.
- Log access denials; do not log full ZIP bodies or raw tickets.
- Full route + RBAC tables: [11](./11-api-schema-and-tickets.md), [12](./12-ui-rbac-lifecycle-ops.md).

---

## 9. Residual risks (accept & document)

| Risk | Severity | Status |
|------|----------|--------|
| WebRTC or future browser API bypasses CSP | High if present | Test per browser; track Chromium CSP webrtc |
| `'unsafe-inline'` scripts inside app origin | Medium (contained) | Accept for compatibility; origin isolation is the real boundary |
| Malicious UI in iframe (phishing “login”) | Medium | Label chrome: “Mini-app (untrusted)”; never ask password in-frame via host |
| Peer DoS via update flood | Medium | Rate limits + max log size |
| Snapshot contains sensitive app data visible to all who can open app | Medium | Product: who can open apps |
| LiveKit not E2E for data channels depending on deployment | Medium | Same as chat/whiteboard today |
| Shared cookie parent domain misconfig | Critical if mis-set | Deploy checklist |

---

## 10. Expectation matrix (updated)

| Aspect | Official WebXDC | Bedrud web host |
|--------|-----------------|-----------------|
| Internet access | None | CSP + Permissions-Policy + sandbox + **origin isolation**; WebRTC tested; external links only with confirm (1.3) |
| Sandbox | Isolated webview | iframe sandbox **without** same-origin-to-SPA footgun |
| Resources | From `.xdc` only | Zipfs + validation + CSP; host **`webxdc.js`** |
| Storage | Per-app isolation | Dedicated origin / partition per `appId` |
| Communication | Host API only | Trusted `webxdc.js` + LiveKit status/RT; no tokens in frame |
| Untrusted code | Assumed | Explicit threat model §1 |

---

## 11. Auth middleware & headers placement

- JSON API routes: existing JWT/RBAC middleware.
- Asset routes: authz + **security headers middleware dedicated to WebXDC** (do not reuse loose static-file middleware).
- Meeting SPA: unchanged CSP of the app; must **not** weaken SPA CSP to accommodate mini-apps.

---

## 12. References (external)

- [WebXDC get started](https://webxdc.org/docs/get_started.html) — no internet, sandboxed webview  
- [Delta Chat: WebXDC security audit notes](https://delta.chat/en/2023-05-22-webxdc-security) — network isolation, WebRTC gap  
- HTML iframe sandbox: `allow-scripts` + `allow-same-origin` **same origin as embedder** removes sandbox protection (MDN / HTML spec warning)  
- MDN Content-Security-Policy; Permissions-Policy  

Internal: [03 bridge](./03-realtime-bridge-architecture.md), [04 payloads](./04-payloads-and-edge-cases.md), [06 checklist](./06-verification-checklist.md).
