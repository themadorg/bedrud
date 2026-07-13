# 12 — UI, RBAC, room lifecycle, ops, deferred APIs

Complements [11](./11-api-schema-and-tickets.md). Feature remains **experimental**.

---

## 1. Meeting UI (web)

### Visibility

| Condition | UI |
|-----------|-----|
| Server `webxdc.enabled === false` (or not Active) | Experimental switch disabled; no Apps |
| Server on, user **Settings → Experimental → WebXDC** off | Hint in room info; no Apps panel |
| Server on **and** user experimental toggle on | Show **Apps (experimental)** (room info / panel) |

Never hide the experimental label in v0. Both **server config** and **user experimental preference** are required.

### Layout (v1)

```
Meeting toolbar
  [… mic cam …] [Apps ▾ experimental]
       │
       ▼
  Apps panel / sheet
    - List packages (icon, name)
    - [Upload .xdc]  (if canUpload)
    - Open instances (document / summary)
    - [Start] / [Focus] / [Close] (mod)
       │
       ▼
  Main stage or side panel
    - iframe (sandbox + allow="" + referrerpolicy)
    - Chrome: name | summary | “Untrusted mini-app · experimental”
    - External link confirm dialog (spec 1.3)
```

### Flows

1. **Upload** → `POST packages` → list refresh.  
2. **Gallery (optional)** → browse global/curated catalog → “Add to room” / Start (see [13](./13-app-gallery.md)).  
3. **Start** → `POST instances` (+ ticket) → set `iframe.src` → inject/listen bridge.  
4. **Focus existing** → `POST ticket` if expired → navigate iframe (or same instance).  
5. **Activity `info`** → line in meeting “system/activity” feed; click → open instance + relative `href`.  
6. **Mod close** → `POST close` + LiveKit control; tear down iframes.  

### Components (expected)

```
apps/web/src/components/meeting/webxdc/
  WebxdcAppsButton.tsx      # toolbar entry + experimental badge
  WebxdcPanel.tsx           # list / upload / start
  WebxdcFrame.tsx           # iframe shell
  useWebxdcHost.ts          # postMessage bridge
  useWebxdcConfig.ts        # public enabled/baseDomain
  webxdc*.ts                # pure helpers (already partially landed)
```

### Accessibility

- iframe `title` includes app name + “experimental untrusted mini-app”.  
- Focus management when opening/closing panel.  
- Confirm dialogs keyboard-accessible.

### i18n

English strings first; keys for later locale packs: `webxdc.apps`, `webxdc.experimental`, `webxdc.untrusted`, `webxdc.externalLinkWarning`.

---

## 2. RBAC matrix

| Action | Owner | Moderator | Member | Guest | Notes |
|--------|-------|-----------|--------|-------|-------|
| See Apps (if enabled) | ✓ | ✓ | ✓ | ✓* | *if guest may join room |
| Upload package | ✓ | ✓ | config | ✗ default | `uploadPolicy: owner_mod` \| `any_member` |
| Create/start instance | ✓ | ✓ | ✓ | ✓ | room access only |
| sendUpdate / realtime | ✓ | ✓ | ✓ | ✓ | if can open |
| Force close instance | ✓ | ✓ | ✗ | ✗ | creator may close own if product allows |
| Delete package | ✓ | ✓ | ✗ | ✗ | |
| Mint ticket | ✓ | ✓ | ✓ | ✓ | requires room access |

Guests: use existing guest join identity; `selfAddr` HMAC includes guest id; no elevation via payload.

Admin **global** superadmin: may disable feature only via config/env (no need for runtime admin toggle in v0). Optional later: system_settings flag mirroring `enabled` for hot toggle without restart.

---

## 3. Room / meeting lifecycle (cleanup)

**Yes — when the meeting/room ends, temporary WebXDC data must be torn down**, not left on disk or in DNS-routable instance hosts forever.

Bedrud already has room **suspend**, **idle cleanup**, and **hard delete** jobs. WebXDC must plug into those.

| Event | Client | Server (required) |
|-------|--------|-------------------|
| **Participant leaves** meeting UI | Destroy iframe(s); drop postMessage listeners; forget tickets in memory | No mandatory server delete (others may still use the app) |
| **Last participant leaves** / room goes idle | Same | Prefer **soft-close all instances** (stop Host serving; optional short grace) |
| **Room suspend** (existing Bedrud) | Force-close iframes on control/nudge | Deny new tickets/opens; **soft-close instances**; purge or freeze status log per policy below |
| **Room hard delete / cleanup job** | n/a | **Full cascade delete** (see below) |
| **Instance soft-close** (mod or room end) | Destroy iframe | `closed_at` set; Host returns 404/410 for that `webxdc-<id>`; tickets invalid |
| **Instance hard delete** | n/a | Delete status rows; stop Host routing; free id for reuse only after purge |
| User ban | Ticket renew fails | Existing authz |

### What “temp app things” means (must go away on room end / delete)

| Artifact | On room suspend / idle end | On room hard delete |
|----------|----------------------------|---------------------|
| Open iframes | Force close (LiveKit `control: close` + client leave) | n/a |
| Capability tickets | Invalidate (exp short; deny by closed instance) | n/a |
| `webxdc_instances` | Soft-close all; then **hard-delete** after grace or immediately on hard room delete | CASCADE delete |
| `webxdc_status_updates` | Delete with instance (or purge after grace) | CASCADE delete |
| Room-scoped `webxdc_packages` + ZIP blobs | **Delete** with room (temp by default) | CASCADE + delete storage files |
| Gallery global packages | **Keep** (not room-temp) | Keep |
| Host routing `webxdc-<id>.…` | 404 after close/delete | 404 |

**Default policy (locked):** room-uploaded packages and their instances/status logs are **meeting-scoped temporary data**. When the room is **hard-deleted** or cleaned up by the idle/delete pipeline, they **must** be removed (DB + blob storage).

**Suspend / idle (softer end of meeting):**

1. Soft-close every instance in the room.  
2. Deny new opens/tickets.  
3. **Either** (config, default **A**):  
   - **A. Eager purge (recommended default):** after suspend/idle cleanup, delete room WebXDC packages, instances, status logs, and blobs (treat meeting end as wipe).  
   - **B. Retain until hard delete:** keep blobs for a TTL then purge (only if product wants “resume room later with same apps”).  

Plan default: **A — wipe WebXDC temp data when the room is suspended/cleaned as ended**, so disks and instance hostnames do not accumulate. If Bedrud “suspend” is reversible and product needs resume, switch to B with an explicit TTL (e.g. 24h) still ending in delete.

### Cascade steps (implementation)

Hook existing `RoomCleanupService` / queue `room_delete` / idle cleanup:

1. List `webxdc_instances` for `room_id` → soft-close + optional LiveKit close control.  
2. Delete `webxdc_status_updates` for those instances.  
3. Delete `webxdc_instances`.  
4. Delete room-scoped `webxdc_packages` and **storage blobs** (`storage_key`).  
5. Do **not** delete gallery/global packages (`room_id IS NULL` / gallery origin).

Align with `RoomCleanupService` / queue `room_delete` handlers: add this webxdc cascade as a required step (tests in [11](./11-api-schema-and-tickets.md)).

---

## 4. LiveKit control plane (client)

In addition to status/realtime:

| Message | Who may send | Receivers |
|---------|--------------|-----------|
| `control: close` | mod/owner (receivers re-check RBAC) | destroy iframe |
| `control: open` / presence of instance | optional | show “App running” chip |
| `nudge` maxSerial | anyone after POST updates | pull status log |

Do not open iframes solely from peer messages without local user intent (except optional “follow mod” later).

---

## 5. Ops: reverse proxy & TLS

**When WebXDC is enabled, both are mandatory:**

1. **DNS:** `*.{baseDomain}` (e.g. `*.wx.example.com`) must resolve to the **same edge IP/target** as the main Bedrud domain (this server or the reverse proxy in front of it).  
2. **TLS:** a certificate covering `*.{baseDomain}` (not only the apex/main name).

Installer must tell the admin this when they enable WebXDC — see [10 §3](./10-config-and-installer.md).

### DNS

```text
# Main site (existing)
example.com.            A/AAAA  <edge>

# REQUIRED for WebXDC — same target as main site / proxy
*.wx.example.com.       A/AAAA  <edge>
# optional
wx.example.com.         A/AAAA  <edge>
```

### Caddy (sketch)

```caddyfile
# Main app
app.example.com {
  reverse_proxy 127.0.0.1:8090
}

# Wildcard WebXDC hosts — cert must cover *.wx.example.com (DNS-01)
*.wx.example.com {
  tls {
    dns <provider> ...
  }
  reverse_proxy 127.0.0.1:8090
}
```

### nginx (sketch)

```nginx
server {
  server_name ~^webxdc-[a-z0-9]+\.wx\.example\.com$;
  # ssl_certificate for *.wx.example.com
  location / {
    proxy_pass http://127.0.0.1:8090;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

### ACME / cert when WebXDC is enabled

| Method | Wildcard `*.wx.example.com` |
|--------|----------------------------|
| HTTP-01 | **No** — cannot issue `*` |
| DNS-01 | **Yes** — preferred for in-app or Caddy ACME |
| External cert (proxy) | **Yes** — proxy holds `*.` cert; Bedrud may be HTTP upstream |
| Self-signed install | **Yes** — SANs must include `*.{baseDomain}` |

**Plan requirement:** enabling WebXDC implies obtaining or attaching a **`*.{baseDomain}` cert**, not only the main domain cert. If install cannot do DNS-01, admin must supply wildcard TLS at the proxy and is told so during install.

### Dev without wildcard

| Mode | How |
|------|-----|
| `webxdc-<id>.localhost` | Some browsers resolve `*.localhost` → 127.0.0.1 |
| `/etc/hosts` single label | Limited testing |
| Path mode | **Not for prod**; optional `WEBXDC_DEV_SAME_ORIGIN=1` only in dev builds |

---

## 6. Deferred host APIs (meeting mapping)

| Spec API | v0 | Later mapping |
|----------|----|---------------|
| `sendToChat` | stub reject or “not supported” | Open meeting chat composer with text/file draft; user sends |
| `importFiles` | may use `<input type=file>` only | + recent chat uploads picker |
| `mailto:` links | intercept → confirm open OS mail / copy | |
| `getAllUpdates` | deprecated → empty / omit | |
| Integrated internet apps (maps) | **out of scope** | never for arbitrary packages |

---

## 7. Multi-client scope

| Client | v0 experimental | Notes |
|--------|-----------------|-------|
| **Web meeting** (`apps/web`) | **In scope** | Primary |
| Desktop (Rust/Slint) | Out of scope | Could embed WebView later |
| Android / iOS | Out of scope | Same |
| Site (marketing) | Docs only | Operator page when shipping |

Plan docs assume **web host** only until graduation.

---

## 8. Graduation criteria (leave “experimental”)

All must be true before removing experimental labeling:

1. Security gates G0–G5 ([05](./05-implementation-roadmap.md)) + checklist [06](./06-verification-checklist.md) green on Chromium + Firefox + Safari sample.  
2. Ticket + Host routing + CSP-on-404 reviewed.  
3. Status serial log under concurrent multi-user load tested.  
4. Room delete cascade verified.  
5. Operator docs published (config, DNS, TLS).  
6. At least one real third-party `.xdc` from webxdc store runs without fork.  
7. Explicit product decision + changelog “WebXDC: stable experimental → supported”.  

Until then: UI badge, config comments, installer wording stay **experimental**.

---

## 9. Admin / settings

| Surface | v0 |
|---------|----|
| `config.yaml` / env | **Source of truth** |
| Installer | Opt-in + baseDomain |
| Admin dashboard toggle | Optional later; if added, must not weaken security defaults |
| Superadmin “kill switch” | Prefer env `WEBXDC_ENABLED=false` + restart |

Public settings endpoint exposes only `{ enabled, experimental: true, baseDomain? }`.

---

## 10. Privacy / retention

| Data | Retention |
|------|-----------|
| Room package ZIP / instances / status | **Deleted when room ends** (suspend wipe default or hard delete) — temporary |
| Gallery global packages | Until admin removes |
| Tickets | Stateless JWT until exp; invalid once instance closed |
| Logs | No ticket secrets, no full payloads in info logs by default |

Self-hosters responsible for backups of `/var/lib/bedrud/webxdc` (or S3).

---

## 11. Sequence: upload → open → update (happy path)

```
User          SPA              API/Go              LiveKit           iframe
 |--upload--->|---POST package-->|                   |                 |
 |            |<--package id-----|                   |                 |
 |--start---->|---POST instance->|                   |                 |
 |            |<--id+ticket+url--|                   |                 |
 |            |---set iframe.src--------------------------------------->|
 |            |                  |                   |                 |--GET / ?t=
 |            |                  |<--ticket ok-------|                 |--webxdc.js
 |            |                  |                   |                 |--index.html
 |            |<======== postMessage ready ============================|
 |--type----- |                  |                   |                 |
 |            |<--sendUpdate-----|                   |                 |
 |            |---POST updates-->| assign serial     |                 |
 |            |                  |---nudge---------->|                 |
 |            |<--DC nudge-------|<------------------|                 |
 |            |---GET ?after=N-->|                   |                 |
 |            |---webxdcUpdate---------------------------------------->|
 |            |                  |                   |                 | listener
```

---

## 12. Open product choices (small)

Still choosable without blocking schema:

1. Side panel vs stage takeover for iframe.  
2. Whether package delete force-closes instances or 409.  
3. Guest upload ever allowed (default no).  
4. Status nudge-only vs full payload on LiveKit.

Defaults if unspecified: **side panel**, **force-close on package delete**, **no guest upload**, **nudge + GET pull** (Desktop-like).
