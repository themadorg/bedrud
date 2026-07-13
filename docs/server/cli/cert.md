# `bedrud certificate` (alias: `cert`)

TLS certificate management for self-hosted Bedrud.

**Source:** `server/internal/cli/cert.go`

Default cert paths from config: `/etc/bedrud/cert.pem`, `/etc/bedrud/key.pem`.

```bash
bedrud certificate <subcommand>
bedrud cert <subcommand>          # short alias
```

---

## `bedrud certificate regenerate`

Regenerate (or create) the self-signed TLS certificate from the **current** config SANs.

When `webxdc.enabled` is true (and not path-mode), SANs include:

- `webxdc.baseDomain`
- `*.{webxdc.baseDomain}` — required for `webxdc-<id>.{baseDomain}` instance hosts

```bash
bedrud certificate regenerate
bedrud certificate regenerate --algo ed25519
bedrud certificate regenerate --force   # allow when useACME is true (still self-signed only)
bedrud --json certificate regenerate
```

| Flag | Description |
|------|-------------|
| `--algo` | `ed25519`, `ecdsa256`, `rsa2048`, `rsa4096` (default: config `certAlgorithm`, existing cert, or ed25519) |
| `--force` | Allow overwrite even when `server.useACME` is enabled |
| `--config` | Bedrud config (root) |
| `--json` | JSON result |

**Always SANs:** `server.domain`, non-loopback `server.host`, outbound IP (if any), `localhost`, `127.0.0.1`, `::1`.

**JSON `data`:** `certFile`, `keyFile`, `sans`, `validDays`, `algorithm`, `algorithmSource`, `created`, `webxdcWildcard`

Creates the pair if missing; otherwise atomic renew (`.new` + rename). Restart services after:

```bash
sudo systemctl restart livekit bedrud
```

---

## `bedrud certificate renew` / `bedrud cert renew`

Same SAN rebuild as regenerate (including WebXDC wildcard). Historically named “renew”; prefer `regenerate` for new docs/scripts.

```bash
bedrud cert renew
bedrud cert renew --algo ed25519
bedrud --json cert renew
```

| Flag | Description |
|------|-------------|
| `--algo` | `ed25519`, `ecdsa256`, `rsa2048`, `rsa4096` |
| `--config` | Bedrud config (root) |
| `--json` | JSON result |

---

## `bedrud certificate info` / `bedrud cert info`

Show certificate status (subject, issuer, expiry, SANs). Also reports expected SANs and any missing ones (e.g. WebXDC wildcard after enabling mini-apps).

```bash
bedrud certificate info
bedrud --json cert info
```

If TLS is disabled in config:

```json
{
  "ok": true,
  "message": "TLS: not enabled",
  "data": { "enabled": false }
}
```

When enabled, `data` includes `subject`, `issuer`, `notBefore`, `notAfter`, `daysRemaining`, `status`, `sans`, `expectedSans`, `missingSans`, `webxdcWildcard`, `certFile`, `keyFile`.

---

## Related

- [../configuration.md](../configuration.md) — `server.enableTls`, cert paths, webxdc
- [../internal/utils.md](../internal/utils.md) — cert generation helpers
