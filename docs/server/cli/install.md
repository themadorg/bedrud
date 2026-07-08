# `bedrud install`

Interactive Linux installer: copies binary, writes configs, creates init service (systemd / OpenRC / SysV), configures TLS.

**Source:** `server/internal/cli/install.go` → `internal/install.LinuxInstall()`

Requires root for typical paths (`/etc/bedrud`, `/usr/local/bin`).

---

## Usage

```bash
sudo bedrud install
sudo bedrud install --self-signed --behind-proxy
sudo bedrud --json install --no-tls
```

---

## Flags

| Flag | Description |
|------|-------------|
| `--tls` | Enable HTTPS with self-signed cert (alias for `--self-signed`) |
| `--self-signed` | Generate self-signed TLS certificate |
| `--no-tls` | Disable TLS (overrides `--tls`) |
| `--ip` | Override detected public IP |
| `--domain` | Domain for Let's Encrypt |
| `--email` | ACME registration email |
| `--port` | Override HTTP(S) port |
| `--cert` | Existing certificate file path |
| `--key` | Existing private key file path |
| `--lk-port` | LiveKit API port (default 7880) |
| `--lk-tcp-port` | LiveKit RTC TCP port |
| `--lk-udp-port` | LiveKit RTC UDP port |
| `--lk-udp-range` | WebRTC UDP range, e.g. `50000-60000` |
| `--fresh` | Remove existing install before reinstalling |
| `--behind-proxy` | CDN/reverse-proxy mode |
| `--external-livekit` | External LiveKit URL |
| `--livekit-domain` | Separate domain for local LiveKit |
| `--lk-ip` | LiveKit node IP behind CDN |
| `--cert-algorithm` | `ed25519`, `ecdsa256`, `rsa2048`, `rsa4096` |
| `--json` | Result as JSON envelope |

---

## JSON output

```json
{
  "ok": true,
  "message": "✓ Bedrud installed successfully",
  "data": {
    "enableTls": true,
    "selfSigned": true,
    "disableTls": false,
    "behindProxy": false,
    "domain": "meet.example.com"
  }
}
```

---

## Related

- [uninstall.md](./uninstall.md)
- [cert.md](./cert.md)
- [../internal/install.md](../internal/install.md)