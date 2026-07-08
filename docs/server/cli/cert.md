# `bedrud cert`

TLS certificate management for self-hosted Bedrud.

**Source:** `server/internal/cli/cert.go`

Default cert paths from config: `/etc/bedrud/cert.pem`, `/etc/bedrud/key.pem`.

---

## `bedrud cert renew`

Renew the self-signed TLS certificate. Preserves key algorithm from existing cert unless `--algo` is set.

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

**JSON `data`:** `certFile`, `keyFile`, `sans`, `validDays`, `algorithm`

---

## `bedrud cert info`

Show certificate status (subject, issuer, expiry, SANs).

```bash
bedrud cert info
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

When enabled, `data` includes `subject`, `issuer`, `notBefore`, `notAfter`, `daysRemaining`, `status`, `sans`, `certFile`, `keyFile`.

---

## Related

- [../configuration.md](../configuration.md) — `server.enableTls`, cert paths
- [../internal/utils.md](../internal/utils.md) — cert generation helpers