# 10 — Config, wildcard domain, and installer opt-in

WebXDC in Bedrud is **experimental** and **opt-in**.

- **Experimental:** behavior, config schema, host URLs, and security posture may change; not a stable production guarantee until explicitly graduated.
- **Opt-in:** when disabled (default), no mini-app UI, routes, or Host routing for `webxdc-*` subdomains.

Installer copy and any admin/UI chrome should label the feature **Experimental** whenever operators enable it.

Related: origin model [02](./02-security-sandbox-csp.md), Desktop-style hostnames [08](./08-deltachat-desktop-host.md).

---

## 1. Config section (`webxdc`)

Add a top-level YAML block (alongside `server`, `livekit`, …). Suggested shape:

```yaml
webxdc:
  # EXPERIMENTAL. Master switch. false = feature fully off (default).
  # Enabling accepts that the feature may change or have residual risks.
  enabled: false

  # Parent DNS label for per-instance origins.
  # Instance URL becomes:
  #   https://webxdc-<instanceId>.<baseDomain>/
  # Example: baseDomain: wx.example.com
  #   → https://webxdc-a7f3c91e2b0d4e8f.wx.example.com/
  #
  # Operators must publish a DNS wildcard:
  #   *.wx.example.com  → same A/AAAA (or CNAME) as the Bedrud edge
  # And a TLS certificate covering that wildcard:
  #   *.wx.example.com  (or a broader cert if they accept the risk)
  baseDomain: ""

  # Optional override if the public URL scheme/host differs from derived
  # https://webxdc-<id>.<baseDomain> (rare; leave empty to derive).
  # publicURLTemplate: "https://webxdc-{{.InstanceID}}.wx.example.com"

  # Who may upload .xdc packages: owner_mod | any_member
  uploadPolicy: owner_mod

  # Package limits (bytes / counts) — see server/internal/webxdc Limits
  maxArchiveBytes: 10485760        # 10 MiB
  maxUncompressedTotal: 31457280   # 30 MiB
  maxEntries: 500
  maxSingleFileBytes: 5242880      # 5 MiB

  # Status log retention (server-assigned serials)
  statusLogMaxUpdates: 500
  statusLogMaxBytes: 2097152       # 2 MiB per app instance
```

### Environment overrides

| Env | Maps to |
|-----|---------|
| `WEBXDC_ENABLED` | `webxdc.enabled` (`true`/`1`/`yes`) |
| `WEBXDC_BASE_DOMAIN` | `webxdc.baseDomain` |
| `WEBXDC_UPLOAD_POLICY` | `webxdc.uploadPolicy` |

When `enabled: false`, handlers short-circuit (404/403), meeting UI hides Apps entry, installer may still write the block with `enabled: false`.

### Domain required (no IP-only WebXDC)

WebXDC **must not** be enabled on IP-only deployments.

| Condition | WebXDC |
|-----------|--------|
| `server.domain` empty / install is IP-only | **Cannot enable** — do not offer installer prompt (or refuse if forced) |
| `server.domain` set to a real hostname (e.g. `example.com`) | May enable experimental WebXDC |
| `webxdc.enabled: true` without a usable domain | **Config validation error** — refuse to start WebXDC subsystem |
| `webxdc.baseDomain` empty when enabled | **Config validation error** |
| Main public URL is only `http(s)://203.0.113.10` | WebXDC off |

Reasons: per-instance hosts are `webxdc-<id>.{baseDomain}`; wildcard DNS `*.{baseDomain}` and wildcard TLS only make sense with a **domain name**, not a bare IP.

When `enabled: true` but `baseDomain` is empty or domain prerequisites fail, server should **refuse to start WebXDC routes** and log a clear error (or fail config validation).

---

## 2. Wildcard location: `*.yourdomain.com` (recommended form)

### Preferred production layout

| Role | Hostname |
|------|----------|
| Meeting SPA + API | `example.com` or `app.example.com` |
| WebXDC instances | `webxdc-<instanceId>.wx.example.com` |
| DNS | `*.wx.example.com` → Bedrud / reverse proxy |
| TLS | Certificate for `*.wx.example.com` |

**Config:**

```yaml
server:
  domain: "example.com"          # existing: main site / ACME / passkeys

webxdc:
  enabled: true
  baseDomain: "wx.example.com"   # NOT the SPA host; dedicated suffix
```

**Instance URL formula:**

```text
https://webxdc-{instanceId}.{webxdc.baseDomain}/
```

Example: `instanceId = a7f3c91e2b0d4e8f`

```text
https://webxdc-a7f3c91e2b0d4e8f.wx.example.com/
```

### Why not `example.com/something-webxdc`?

Same origin as the SPA breaks isolation (plan [02](./02-security-sandbox-csp.md)). Path-only on the main domain is **not** a supported production mode.

### Why a dedicated suffix (`wx.`) instead of `*.example.com`?

- Auth cookies must stay **host-only** on the SPA host.
- Using `*.example.com` for apps increases the chance of accidental `Domain=.example.com` cookie leakage.
- `*.wx.example.com` keeps WebXDC in a separate cookie jar by construction.

### DNS + TLS requirements (mandatory when enabled)

When `webxdc.enabled: true`, production **must** have:

| Requirement | Example | Notes |
|-------------|---------|--------|
| **Wildcard DNS** | `*.wx.example.com` → **same A/AAAA (or CNAME target) as the main Bedrud/public edge** | Without this, `webxdc-<id>.wx.example.com` will not resolve |
| **Optional apex** | `wx.example.com` → same edge | Optional health/debug; not required for instances |
| **Wildcard TLS cert** | Certificate that includes `*.wx.example.com` | Single-name cert for `example.com` alone is **not** enough |
| **Reverse proxy** (if used) | `Host: webxdc-*.wx.example.com` → Bedrud | Same process is fine |

**TLS how-to (required outcome = `*.` cert exists):**

1. **Preferred when Bedrud does ACME:** if WebXDC is enabled, ACME/cert pipeline **must also obtain or attach a cert covering `*.{baseDomain}`** (and optionally `{baseDomain}`). Wildcards need **ACME DNS-01** (HTTP-01 cannot issue `*`). Installer must document DNS API / manual DNS challenge if in-tree ACME supports DNS-01; if not yet, fail soft with clear “provide cert files that include the wildcard” and refuse to mark WebXDC production-ready.
2. **Preferred when TLS is at the edge:** operator installs a wildcard cert on Caddy/nginx (DNS-01 or purchased) and points both main and `*.wx` traffic at Bedrud.
3. **Self-signed install:** generate a self-signed cert that includes SAN `*.{baseDomain}` (and main domain if same cert) so local/dev-style installs still match Host routing.

**Hard rule:** Enabling WebXDC without a path to `*.{baseDomain}` TLS is incomplete — installer and docs must not imply that the main site cert alone is enough.

### Operator checklist (always print when enabling)

```text
REQUIRED — do this in your DNS provider before WebXDC works:

  1. Add a wildcard DNS record:
       *.wx.example.com   →  same IP/target as your Bedrud domain
         (same A/AAAA as example.com / app.example.com, or your reverse proxy)

  2. Ensure TLS covers:
       *.wx.example.com
     (Let's Encrypt DNS-01, or cert on reverse proxy, or self-signed SAN)

  3. Instance URLs will look like:
       https://webxdc-<id>.wx.example.com/
```

---

## 3. Installer (`bedrud install`) — opt-in + mandatory admin messaging

Extend `InstallConfig` and interactive `promptConfig` (see `server/internal/install/`).

### New fields

```go
// InstallConfig additions (plan — not landed yet)
EnableWebxdc     bool   // default false
WebxdcBaseDomain string // e.g. wx.example.com; empty if disabled
```

### Interactive flow (after domain / TLS questions)

**Gate: only ask about WebXDC if a domain is configured.**

- If `cfg.Domain` is empty (IP-only install): **skip** WebXDC entirely; write `webxdc.enabled: false` (or omit). Print once:

```text
  WebXDC: skipped (requires a domain name; not available on IP-only installs)
```

- If domain is set, then:

```text
➜ Enable experimental WebXDC mini-apps in meetings? [y/N]:
  (Requires extra DNS: *.<baseDomain> pointing at this server, plus a wildcard TLS cert)
```

- **No / empty** → write `webxdc.enabled: false` (or omit section); no further WebXDC questions.
- **Yes** →:

```text
➜ WebXDC is experimental and needs wildcard DNS + TLS. Continue? [y/N]:
➜ WebXDC base domain for per-app hosts
  (instances: https://webxdc-<id>.<baseDomain>/ )
  Example: wx.example.com
  [default: wx.<server.domain> if domain set]:
```

Then the installer **MUST print a clear, hard-to-miss block** (not a one-line hint), e.g.:

```text
╔══════════════════════════════════════════════════════════════════╗
║  WebXDC — REQUIRED DNS (do this now)                             ║
╠══════════════════════════════════════════════════════════════════╣
║  Add a wildcard record in your DNS provider:                     ║
║                                                                  ║
║    *.wx.example.com  →  SAME IP / target as your main domain     ║
║                         (point it at this machine or your        ║
║                          reverse proxy in front of Bedrud)       ║
║                                                                  ║
║  Example: if example.com is 203.0.113.10, then:                  ║
║    *.wx.example.com  A  203.0.113.10                             ║
║                                                                  ║
║  Also ensure TLS includes:  *.wx.example.com                     ║
║  (ACME DNS-01, reverse-proxy cert, or self-signed with SAN)      ║
║                                                                  ║
║  Without this, mini-apps will not load (hostname will not resolve║
║  or TLS will fail).                                              ║
╚══════════════════════════════════════════════════════════════════╝
➜ Press Enter after you understand the DNS requirement...
```

**Confirm before finishing install:**

```text
➜ Confirm: I will (or already did) point *.wx.example.com at this server/proxy [y/N]:
```

If they answer **N**, either disable WebXDC in written config with a warning, or abort WebXDC enablement (prefer: write `enabled: false` + message “WebXDC left disabled until DNS confirmed”). Do not silently enable WebXDC when the admin refuses the DNS step.

Validate:

- **`server.domain` (or install Domain) must be non-empty** before `enabled: true` is written.
- Reject baseDomain that looks like a bare IPv4/IPv6 address.
- Non-empty `baseDomain` when enabled.
- Prefer dedicated suffix (e.g. `wx.`) not equal to bare SPA host alone.
- Show the concrete `*.{baseDomain}` string filled in (no placeholders only).

### TLS / cert generation when WebXDC is enabled

| Install mode | Required installer behavior |
|--------------|----------------------------|
| **Self-signed** | Generate cert with SANs including main domain **and** `*.{baseDomain}` (and optionally bare `{baseDomain}`). Tell the admin the cert is self-signed for both. |
| **ACME / Let's Encrypt** | Request or attach coverage for `*.{baseDomain}` via **DNS-01** when supported; if DNS-01 is not available in this install path, print that they must put a wildcard cert on the reverse proxy and set cert paths accordingly. |
| **User-supplied cert/key** | Validate or warn: cert should include `*.{baseDomain}`; if not, print a loud warning that WebXDC HTTPS will fail. |
| **Behind reverse proxy (`--behind-proxy`)** | Still require DNS `*.{baseDomain}` → proxy; tell admin the **proxy** must terminate TLS for `*.{baseDomain}` (same IP as main site is typical). |

### Non-interactive / flags (suggested)

```bash
bedrud install --domain example.com --webxdc --webxdc-base-domain wx.example.com --webxdc-dns-ack
bedrud install --no-webxdc   # explicit default
```

- **`--webxdc` without `--domain` (or empty domain)** → **error** and do not enable WebXDC:  
  `WebXDC requires --domain (not available for IP-only installs)`.
- When `--webxdc` succeeds, **still print the full DNS wildcard block** (and include fields in `--json`).
- Prefer requiring `--webxdc-dns-ack` for non-interactive enable.

### Generated `config.yaml` snippet

When enabled:

```yaml
webxdc:
  enabled: true
  baseDomain: "wx.example.com"
  uploadPolicy: owner_mod
```

When disabled:

```yaml
webxdc:
  enabled: false
```

### Install success banner (when enabled)

```text
  WebXDC:          enabled (experimental)
  WebXDC base:     wx.example.com
  WebXDC DNS:      *.wx.example.com  →  MUST point to this server / reverse proxy
  WebXDC TLS:      cert MUST cover *.wx.example.com
  WebXDC URL form: https://webxdc-<id>.wx.example.com/
```

When disabled: `WebXDC: disabled`.

### CLI / flags surface (ops-cli later)

Document in install help and `docs/server/cli/install.md` when implemented.

---

## 4. Runtime behavior matrix

| `enabled` | `server.domain` | `baseDomain` | Behavior |
|-----------|-----------------|--------------|----------|
| `false` | any | any | Feature off |
| `true` | empty / IP-only | any | **Invalid** — refuse WebXDC; log error |
| `true` | set | empty | **Invalid** — refuse WebXDC |
| `true` | set | `wx.example.com` | Host routing `webxdc-*.wx.example.com`; SPA builds iframe URLs |

Frontend should read a public settings/bootstrap field, e.g. `GET /api/settings` or existing public config:

```json
{ "webxdc": { "enabled": true, "baseDomain": "wx.example.com" } }
```

so the meeting UI does not show Apps when disabled.

---

## 5. Implementation checklist (for code PRs)

- [ ] `config.WebxdcConfig` + env overrides + validation
- [ ] `config.local.yaml.example` commented `webxdc:` block
- [ ] Installer `InstallConfig` + `promptConfig` + YAML emit
- [ ] Installer: **WebXDC only if domain is set** (hide/skip/error on IP-only)
- [ ] Installer **mandatory DNS wall** when enabling: tell admin to add `*.{baseDomain}` → same IP as main domain; confirm/ack
- [ ] Self-signed path: SANs include `*.{baseDomain}`; ACME/proxy path: document or request wildcard cert
- [ ] Config validation: `enabled` implies non-empty `server.domain` + `webxdc.baseDomain`
- [ ] Install success summary: DNS + TLS lines for WebXDC
- [ ] Public “feature flags” for SPA
- [ ] Docs: `docs/server/configuration.md` section
- [ ] Site/docs operator page when shipping (bedrud.org configuration MDX)
- [ ] Host header parse: `webxdc-<id>.` + `baseDomain` suffix match

---

## 6. Example full config (production sketch)

```yaml
server:
  domain: "example.com"
  enableTLS: true
  # … existing …

webxdc:
  enabled: true
  baseDomain: "wx.example.com"   # requires DNS + TLS for *.wx.example.com
  uploadPolicy: owner_mod
```

**Do not** enable WebXDC on path-only same origin as the SPA for production.
