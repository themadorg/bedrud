# 11 — API, database schema, Host routing, capability tickets

Fills implementation gaps for the **experimental** WebXDC host. Complements [10](./10-config-and-installer.md) (config), [02](./02-security-sandbox-csp.md) (security), [12](./12-ui-rbac-lifecycle-ops.md) (UI/RBAC/ops).

**Status:** experimental — route paths may get a version prefix later; treat as the v0 contract.

When `webxdc.enabled` is false, all routes below return **404** (or **403** with a clear body — prefer **404** to avoid feature probing).

---

## 1. Concepts

| Term | Meaning |
|------|---------|
| **Package** | Stored `.xdc` ZIP bytes + manifest metadata (may be reused by multiple instances) |
| **Instance** | One running app in a room (`appId` / `instanceId`); own origin, storage, status log |
| **Ticket** | Short-lived capability token so the browser can load `webxdc-<id>.{baseDomain}` **without** SPA session cookies on that host |
| **Status entry** | One row in the server-assigned serial log for an instance |

```
User (SPA, JWT/cookie on app host)
  → API on app host (auth)
  → mint ticket + open instance
  → iframe src = https://webxdc-{id}.{baseDomain}/?t=… or Cookie set for that host only
  → zipfs + host webxdc.js
  → postMessage ↔ SPA ↔ LiveKit / status API
```

---

## 2. Database schema (GORM)

### ER sketch

```
rooms
  │
  ├── webxdc_packages (room_id)     # optional library of ZIPs in a room
  │         │
  │         └── webxdc_instances (package_id, room_id)
  │                   │
  │                   ├── webxdc_status_updates (instance_id, serial)
  │                   └── (runtime only: open iframes on clients)
  │
  └── (existing room_participants for authz)
```

### `webxdc_packages`

| Column | Type | Notes |
|--------|------|-------|
| `id` | varchar(36) PK | UUID |
| `room_id` | varchar(36) FK → rooms | CASCADE delete with room |
| `content_hash` | varchar(64) | SHA-256 hex of ZIP bytes |
| `storage_key` | varchar(512) | Path/key in disk/S3/inline backend |
| `size_bytes` | bigint | Archive size |
| `name` | varchar(255) | From `manifest.toml` or filename |
| `source_code_url` | varchar(512) | Optional from manifest |
| `icon_path` | varchar(255) | `icon.png` / `icon.jpg` if present |
| `uploaded_by` | varchar(36) | User id (or guest id) |
| `created_at` | timestamp | |

Indexes: `(room_id)`, unique optional `(room_id, content_hash)` to dedupe.

### `webxdc_instances`

| Column | Type | Notes |
|--------|------|-------|
| `id` | varchar(36) PK | **Opaque** id used in hostname: `webxdc-{id without dashes or short form}` |
| `room_id` | varchar(36) FK | |
| `package_id` | varchar(36) FK | |
| `created_by` | varchar(36) | |
| `document` | varchar(64) | Latest `update.document` (chrome) |
| `summary` | varchar(64) | Latest `update.summary` |
| `last_info` | varchar(128) | Latest `update.info` (optional) |
| `closed_at` | timestamp NULL | Soft-close; hard delete optional |
| `created_at`, `updated_at` | timestamp | |

**Hostname id:** prefer 16–32 char lowercase hex/base32 **without** dots (DNS labels). Store as `id` or separate `host_label` column.

Index: `(room_id)`, `(host_label)` unique global (or unique per `baseDomain` generation).

### `webxdc_status_updates`

| Column | Type | Notes |
|--------|------|-------|
| `id` | bigserial / UUID PK | |
| `instance_id` | varchar(36) FK | CASCADE |
| `serial` | bigint | Monotonic **per instance**, starts at 1 |
| `sender_user_id` | varchar(36) | May be empty for guest |
| `sender_identity` | varchar(255) | LiveKit identity at send time |
| `payload_json` | text/blob | Full update object JSON (`payload`, `info`, `href`, …) |
| `byte_size` | int | |
| `created_at` | timestamp | |

Unique: `(instance_id, serial)`. Index: `(instance_id, serial)`.

Trim policy: when exceeding `statusLogMaxUpdates` / `statusLogMaxBytes`, delete oldest serials (or compact — v1 delete).

### `webxdc_tickets` (optional table vs signed JWT)

**Preferred v1: signed JWT** (no table) — see §4.  
Optional DB if you need instant revoke:

| Column | Type | Notes |
|--------|------|-------|
| `jti` | varchar(36) PK | |
| `instance_id` | varchar(36) | |
| `user_id` | varchar(36) | |
| `expires_at` | timestamp | |
| `revoked_at` | timestamp NULL | |

### Room / meeting-end cascade (required)

Room-scoped WebXDC data is **temporary meeting data**. It must not survive room teardown.

**On room hard-delete / idle cleanup job** (align with existing room delete queue):

1. Soft-close instances; best-effort LiveKit `control: close`.  
2. Delete status updates → instances.  
3. Delete room-scoped packages **and** storage blobs.  
4. Host routes for those `webxdc-<id>` labels must 404.  
5. Never delete gallery/global packages.

**On room suspend / “meeting ended” (default policy A — eager wipe):** same purge as hard delete for WebXDC room artifacts (packages, instances, logs, blobs), so temp apps do not linger. Alternative retain-until-TTL is opt-in only ([12 §3](./12-ui-rbac-lifecycle-ops.md)).

**On participant leave only:** no server purge (other participants may still use the app).

---

## 3. HTTP API (app host — authenticated)

Base: existing Fiber `/api` group. All require normal JWT/session **unless noted**.

Authz helpers:

- `canAccessRoom(user, roomId)` — member or valid guest join  
- `canUploadWebxdc` — per `uploadPolicy` + room role  
- `canModerateRoom` — owner/mod  

### 3.1 Feature bootstrap (public-ish)

| Method | Path | Auth | Response |
|--------|------|------|----------|
| GET | `/api/auth/settings` (extend existing public settings) **or** `/api/webxdc/config` | none or any | `{ "webxdc": { "enabled": bool, "experimental": true, "baseDomain": "wx.example.com" \| null } }` |

Never expose secrets. If disabled, `baseDomain` may be null/omitted.

### 3.2 Packages

| Method | Path | Authz | Status | Body |
|--------|------|-------|--------|------|
| POST | `/api/rooms/:roomId/webxdc/packages` | upload policy | 201 | multipart `file` (.xdc) → package JSON |
| GET | `/api/rooms/:roomId/webxdc/packages` | room access | 200 | list `{ id, name, iconUrl?, sizeBytes, createdAt }[]` |
| GET | `/api/rooms/:roomId/webxdc/packages/:packageId` | room access | 200 | metadata |
| DELETE | `/api/rooms/:roomId/webxdc/packages/:packageId` | mod/owner | 204 | fails if instances still open? prefer force-close instances or 409 |

**POST validation:** zip limits, `index.html`, zip-slip ([`internal/webxdc`](../../../server/internal/webxdc)).

### 3.3 Instances

| Method | Path | Authz | Status | Body |
|--------|------|-------|--------|------|
| POST | `/api/rooms/:roomId/webxdc/instances` | room access | 201 | `{ packageId }` → instance + optional ticket |
| GET | `/api/rooms/:roomId/webxdc/instances` | room access | 200 | open/recent instances (document/summary/icon) |
| GET | `/api/rooms/:roomId/webxdc/instances/:instanceId` | room access | 200 | metadata |
| POST | `/api/rooms/:roomId/webxdc/instances/:instanceId/ticket` | room access | 200 | `{ ticket, expiresAt, iframeOrigin, iframeUrl }` |
| POST | `/api/rooms/:roomId/webxdc/instances/:instanceId/close` | mod/owner (or creator) | 204 | soft-close + LiveKit control |
| DELETE | `/api/rooms/:roomId/webxdc/instances/:instanceId` | mod/owner | 204 | hard delete log optional |

**201 instance response (sketch):**

```json
{
  "id": "a7f3c91e2b0d4e8f",
  "roomId": "…",
  "packageId": "…",
  "name": "Poll",
  "iframeOrigin": "https://webxdc-a7f3c91e2b0d4e8f.wx.example.com",
  "iframeUrl": "https://webxdc-a7f3c91e2b0d4e8f.wx.example.com/?t=eyJ…",
  "ticket": "eyJ…",
  "expiresAt": "2026-07-10T12:00:00Z",
  "experimental": true
}
```

### 3.4 Status log (serial authority)

| Method | Path | Authz | Status | Notes |
|--------|------|-------|--------|-------|
| POST | `/api/rooms/:roomId/webxdc/instances/:instanceId/updates` | room access + not closed | 201 | Body: sendUpdate object; server assigns `serial`; returns received update |
| GET | `/api/rooms/:roomId/webxdc/instances/:instanceId/updates?after=N` | room access | 200 | Updates with `serial > N`, ordered; include `maxSerial` |

**POST 201:**

```json
{
  "serial": 12,
  "maxSerial": 12,
  "update": { "payload": {…}, "info": "…", "href": "…" },
  "ts": 1710000000000
}
```

Enforce `sendUpdateMaxSize` (128000 default) and server-side rate limit (in addition to client interval).

After POST, server **or** client publishes LiveKit topic `webxdc` nudge:

```json
{ "v": 1, "kind": "nudge", "appId": "<instanceId>", "maxSerial": 12 }
```

Peers GET `?after=` (Desktop pull model). Optional: include full update in DC for low-latency if under size budget.

### 3.5 Errors

| Code | When |
|------|------|
| 404 | Feature disabled or unknown ids |
| 403 | Authz |
| 400 | Validation (zip, payload, href absolute) |
| 413 | Oversize |
| 429 | Rate limit |
| 409 | Conflict (e.g. delete package with active instances) |
| 503 | `enabled` but `baseDomain` misconfigured |

---

## 4. Capability tickets (subdomain auth)

### Problem

SPA cookies/JWT live on `app.example.com`. The iframe is on `webxdc-<id>.wx.example.com`. Cross-site cookies are undesirable; sharing the session cookie domain is **forbidden** (plan security).

### Solution: short-lived signed ticket

**JWT (or HMAC token)** minted only after room authz check.

**Claims:**

```json
{
  "jti": "uuid",
  "inst": "a7f3c91e2b0d4e8f",
  "room": "room-uuid",
  "sub": "user-or-guest-id",
  "exp": 1710000300,
  "iat": 1710000000,
  "scope": "webxdc-assets"
}
```

| Rule | Value |
|------|--------|
| TTL | 5–15 minutes (config later); SPA refreshes ticket while app open |
| Signature | Dedicated secret or derived from `auth.jwtSecret` with different `kid`/purpose |
| Binding | Host handler requires `inst` match Host label `webxdc-{inst}` |
| Transport | Query `?t=` on first navigation **or** `Set-Cookie` **Host-only** on `webxdc-{inst}.{baseDomain}` via redirect endpoint |

**Recommended flow:**

1. SPA `POST …/ticket` with JWT.  
2. Response includes `iframeUrl` with `?t=`.  
3. First asset request validates `t`, optionally sets `HttpOnly; Secure; SameSite=None; Path=/` cookie scoped to that exact host (not parent domain).  
4. Subsequent relative navigations use cookie; SPA renews before expiry via `postMessage` “ticket-expiring” or timer + reload ticket into iframe.

**Never** put long-lived SPA refresh tokens in the iframe URL.

### Reject

- Ticket for instance A used on host for instance B  
- Expired / bad signature  
- Feature disabled  

Return 401/403 with CSP still applied (XDC-01-002).

---

## 5. Host-header routing (webxdc origin)

Single Bedrud process (or edge) receives TLS for `*.wx.example.com`.

### Parse

```
Host: webxdc-a7f3c91e2b0d4e8f.wx.example.com
```

1. Normalize host (lowercase, strip port).  
2. Require suffix `.{baseDomain}` (config).  
3. Require prefix `webxdc-`.  
4. `instanceLabel = host[len("webxdc-"):len(host)-len("."+baseDomain)]`  
5. Reject if label empty, contains `.`, or fails charset `[a-z0-9-]+` / hex.  
6. Lookup instance by label; 404 if missing/closed.  
7. Validate ticket (§4).  
8. Map path → zip entry (`SafeJoinEntry`); `webxdc.js` → **host file**.  
9. `makeResponse` headers always.

### Non-matching hosts

Requests to SPA host `/webxdc/...` for **assets** should not serve untrusted app HTML in production (optional admin-only debug behind flag).

### ACME / health

`wx.example.com` apex may 404; not required for instances.

---

## 6. Identity: `selfAddr` / `selfName`

| Field | Source |
|-------|--------|
| `selfName` | Meeting display name |
| `selfAddr` | `HMAC-SHA256(server_secret, "webxdc-addr\|" + roomId + "\|" + instanceId + "\|" + userId)` → hex truncate (e.g. 32 chars) |

Stable across reloads/devices for same user+instance; **different** across instances (anti-linkability). Guests: use guest id in the HMAC input.

---

## 7. Storage backend

Reuse patterns from chat uploads where possible:

| Option | When |
|--------|------|
| Disk under `/var/lib/bedrud/webxdc/{packageId}.xdc` | Default self-host |
| S3-compatible | If chat uploads already use S3 |
| Inline DB | Only tiny packages; not preferred |

`storage_key` opaque to clients.

---

## 8. Quotas (defaults)

| Quota | Default | Config key (later) |
|-------|---------|---------------------|
| Packages per room | 20 | `webxdc.maxPackagesPerRoom` |
| Instances open per room | 5 | `webxdc.maxOpenInstancesPerRoom` |
| Concurrent open iframes per client | 1–3 | client only |
| Status log | see [10](./10-config-and-installer.md) | |

---

## 9. Observability (minimal)

Log (no ZIP bodies, no full tickets):

- package upload (room, user, size, hash)  
- instance open/close  
- ticket mint (instance, user, exp)  
- ticket reject (reason code)  
- status post rate-limit hits  

Metrics (optional): `webxdc_enabled`, uploads_total, instances_open, status_updates_total.

---

## 10. Implementation file map (expected)

| Area | Path |
|------|------|
| Config | `server/config/config.go` → `WebxdcConfig` |
| Models | `server/internal/models/webxdc_*.go` |
| Repo | `server/internal/repository/webxdc_*.go` |
| Zip/CSP | `server/internal/webxdc/` (exists) |
| Handlers | `server/internal/handlers/webxdc_*.go` |
| Host middleware | Host parse + ticket on webxdc hosts |
| FE | `apps/web/src/components/meeting/webxdc/` |
| Install | `server/internal/install/` |

**Do not** put any of this under or import from `context/`. Upstream trees are study-only; ship only first-party code in the paths above.

---

## 11. Test additions (beyond 09)

| Test | Layer |
|------|-------|
| Config: enabled without baseDomain fails validation | Go |
| Host parse: valid / invalid labels | Go |
| Ticket wrong instance → 403 | Go |
| Status serial monotonic under concurrent posts | Go |
| Room delete cascades packages | Go |
| Public config never leaks jwt secret | Go |
| Wire + ticket expiry helpers | Web unit |
