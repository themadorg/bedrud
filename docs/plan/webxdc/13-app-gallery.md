# 13 — WebXDC app gallery (global / curated)

**Status:** experimental, **phased after** core host (upload + iframe + status).  
Earlier plan docs only mentioned room-local packages and a vague “shared library”; this doc defines the **gallery** explicitly.

---

## 1. What Delta Chat has (product model)

Delta Chat (and the wider WebXDC ecosystem) expose a **global app gallery / store**:

- Public catalog of mini-apps: [webxdc.org/apps](https://webxdc.org/apps) (curated FOSS apps).
- Submission / tooling ecosystem (e.g. xdcget / store metadata) for listing sources and downloads.
- In the messenger UI: browse / search / start apps from that ecosystem without the user building a `.xdc` themselves.

Bedrud should support a similar **“pick an app”** experience in meetings, without weakening the no-internet mini-app sandbox.

---

## 2. Layers in Bedrud

| Layer | Scope | v0 experimental core | Gallery phase |
|-------|--------|----------------------|---------------|
| **Room packages** | Uploaded `.xdc` stored for one room | **Required** ([11](./11-api-schema-and-tickets.md)) | Still required |
| **Room library** | Reuse packages already in the room | List + Start | Same |
| **Server gallery** | Instance-wide curated catalog (admin or bundled) | Out of scope for first ship | **Planned** |
| **Global remote gallery** | Fetch metadata/ZIPs from public store (webxdc.org / mirror) | Out of scope for first ship | **Optional**, server-side only |

Mini-apps themselves still have **no internet**. Only the **Bedrud server** (trusted) may fetch catalog/ZIP bytes.

---

## 3. Goals

1. In the meeting **Apps** panel: tabs or sections  
   - **This room** (uploaded packages)  
   - **Gallery** (curated / global) — experimental badge  
2. User can **Start** a gallery app into the current room (creates package+instance or instance from cached package).  
3. Operators can run **air-gapped** with gallery disabled or fully local.  
4. Same zip validation, CSP, tickets, and host `webxdc.js` as manual upload.

### Non-goals (gallery v1)

- In-iframe browsing of webxdc.org (would need network in the mini-app — **forbidden**).
- Unreviewed arbitrary URL paste that bypasses size/validation.
- Replacing room upload (always keep upload for custom apps).
- Guaranteeing availability of third-party store (network optional).

---

## 4. Config (extends `webxdc` block)

```yaml
webxdc:
  enabled: true
  baseDomain: "wx.example.com"
  # ...

  gallery:
    # Master switch for gallery UI + APIs (requires webxdc.enabled).
    enabled: false

    # local | remote | both
    # local  = only server-bundled / admin-uploaded gallery packages
    # remote = fetch public catalog metadata (and optional ZIP) server-side
    # both   = merge local + remote
    source: local

    # Public catalog JSON/base URL (only if source is remote|both).
    # Example conceptual: official store API or a self-hosted mirror.
    # Empty = remote disabled even if source says remote.
    remoteCatalogURL: ""

    # If true, server may download .xdc from catalog entry URLs into storage.
    # If false, remote is metadata-only and admin must mirror ZIPs locally.
    allowRemoteDownload: false

    # Max ZIP size for gallery downloads (≤ package maxArchiveBytes).
    maxRemoteArchiveBytes: 10485760

    # Cache TTL for remote catalog JSON (seconds).
    catalogCacheSeconds: 3600
```

Env (suggested): `WEBXDC_GALLERY_ENABLED`, `WEBXDC_GALLERY_SOURCE`, `WEBXDC_GALLERY_REMOTE_URL`.

### Admin System Settings (runtime override)

Operators can configure the gallery **without editing config.yaml** under **Admin → Settings → WebXDC**:

| UI field | SystemSettings JSON | Seeds from config when empty |
|----------|---------------------|------------------------------|
| Enable app gallery | `webxdcGalleryEnabled` | `webxdc.gallery.enabled` |
| Catalog source | `webxdcGallerySource` | `webxdc.gallery.source` |
| **Remote catalog URL** | `webxdcGalleryRemoteCatalogUrl` | `webxdc.gallery.remoteCatalogURL` |
| Allow remote .xdc download | `webxdcGalleryAllowRemoteDownload` | `webxdc.gallery.allowRemoteDownload` |

DB values win after first save. Catalog URL is the primary operator knob for pointing at a mirror or official store API.

**Installer:** do **not** enable gallery by default. Optional later prompt: “Enable experimental app gallery? [y/N]” only when WebXDC is enabled **and** domain is set. Print that remote gallery needs outbound HTTPS from the **server**, not from browsers in the iframe.

---

## 5. Data model

### `webxdc_gallery_entries` (server-global)

| Column | Type | Notes |
|--------|------|-------|
| `id` | varchar(36) PK | |
| `slug` | varchar(128) unique | Stable key |
| `name` | varchar(255) | |
| `description` | text | Optional |
| `source_code_url` | varchar(512) | From manifest / catalog |
| `icon_storage_key` | varchar(512) | Optional cached icon |
| `package_id` | varchar(36) NULL FK | Local `webxdc_packages` row if mirrored globally (room_id NULL or sentinel) |
| `remote_xdc_url` | varchar(1024) | Optional; only used if allowRemoteDownload |
| `remote_content_hash` | varchar(64) | For cache invalidation |
| `origin` | varchar(32) | `bundled` \| `admin` \| `remote` |
| `enabled` | bool | Soft-hide |
| `sort_order` | int | |
| `created_at`, `updated_at` | timestamp | |

**Global packages:** either:

- **A.** `webxdc_packages.room_id` NULL = server-global package (shared blob), or  
- **B.** Separate `webxdc_global_packages` table.

Prefer **A** with clear semantics: `room_id IS NULL` means gallery/global library; room instances copy or reference that package when started in a room.

When user starts a gallery app in room R:

1. Ensure a **room package** row exists (clone metadata + same `storage_key` / content_hash, or reference global package id).  
2. `POST instance` as today.  
3. No change to iframe security model.

---

## 6. API (sketch)

All require `webxdc.enabled` + `gallery.enabled`; else 404.

| Method | Path | Authz | Purpose |
|--------|------|-------|---------|
| GET | `/api/webxdc/gallery` | authenticated user (or room member) | List entries (name, icon, description, origin) |
| GET | `/api/webxdc/gallery/:id` | same | Detail |
| POST | `/api/rooms/:roomId/webxdc/gallery/:id/start` | room access + upload/start policy | Materialize package in room + create instance + ticket |
| POST | `/api/admin/webxdc/gallery` | admin | Add local gallery entry (upload .xdc) |
| DELETE | `/api/admin/webxdc/gallery/:id` | admin | Disable/remove |
| POST | `/api/admin/webxdc/gallery/refresh` | admin | Refresh remote catalog cache (if remote) |

**Remote catalog shape (host-defined adapter):** map third-party JSON into `{ slug, name, description, xdcURL?, sourceCodeURL?, iconURL? }`. Official store format may change — keep an adapter interface.

**Security for remote download (server-side only):**

- SSRF protections (block private IPs, require https, size limit, timeout).  
- Same ZIP validation as upload (`internal/webxdc`).  
- Host always wins for `webxdc.js`.  
- Do not stream untrusted ZIP to the browser as “open URL”; only serve via zipfs after store.

---

## 7. Meeting UI

```
Apps (experimental)
  [ This room ]  [ Gallery ]
       │              │
       │              ├─ search
       │              ├─ cards: icon, name, short description
       │              └─ [Start in this meeting]
       └─ uploads + room packages
```

- Gallery tab hidden if `gallery.enabled` is false.  
- Show origin badge: `Bundled` / `Admin` / `Store`.  
- Remote unavailable → empty state + “Gallery offline” (do not break room uploads).  
- Still show **Untrusted mini-app · experimental** when running.

---

## 8. Relation to webxdc.org store

| Approach | Pros | Cons |
|----------|------|------|
| **Link-only** (open store in new tab) | Simple | User must download/upload manually; weak UX |
| **Server catalog proxy** | Closest to Delta Chat “pick and run” | Needs maintenance, SSRF care, store API stability |
| **Admin-mirrored only** | Air-gap friendly, full control | No automatic updates |
| **Bundled demos** | Works offline out of the box | Few apps |

**Recommended experimental path:**

1. **First:** local/bundled + admin gallery (no outbound required).  
2. **Then:** optional remote catalog + optional download with defaults **off** (`allowRemoteDownload: false`).  
3. Never load the public store **inside** the WebXDC iframe.

Graduation of gallery can lag core WebXDC host.

---

## 9. RBAC

| Action | Who |
|--------|-----|
| Browse gallery | Room members / guests who can open Apps (if gallery on) |
| Start gallery app in room | Same as start instance; upload policy may still apply if materialize counts as upload — **prefer** allow start if user can start instances even when they cannot upload custom ZIPs |
| Admin add/remove gallery entries | Instance admin / superadmin |
| Refresh remote catalog | Admin |

---

## 10. Phasing

| Phase | Deliverable |
|-------|-------------|
| Core host (existing plan) | Room upload, instance, ticket, status — **no gallery required** |
| Gallery A | Config + `webxdc_gallery_entries` + admin upload + GET gallery + Start in room + UI tab |
| Gallery B | Bundled 1–N demo apps from fixtures |
| Gallery C | Remote catalog adapter (metadata) |
| Gallery D | Optional remote ZIP download + cache |

PR order: after core API/UI PRs in [05](./05-implementation-roadmap.md).

---

## 11. Tests

| Test | Notes |
|------|-------|
| Gallery disabled → 404 | |
| Start gallery app → instance on webxdc host with CSP | |
| Remote download blocked to link-local IPs | SSRF |
| Oversize remote ZIP rejected | |
| Air-gap: source=local works without network | |

---

## 12. Summary

| Question | Answer |
|----------|--------|
| Was gallery in the plan before? | **Only vaguely** (“shared library”); not a global store |
| Is it covered now? | **Yes** — this doc |
| Same as Delta Chat global store? | **Same product idea**; implementation is Bedrud-hosted catalog + optional server-side fetch of public store |
| Default on install? | **Off** (even when WebXDC is on) |
