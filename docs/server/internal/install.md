# OS Installer (`internal/install`)

Automated installation for Linux systems. Invoked via `bedrud install` and `bedrud uninstall`.

---

## Files

| File | Purpose |
|------|---------|
| `init.go` | Init system detection and dispatch |
| `linux.go` | Debian/Ubuntu systemd installer |
| `openrc.go` | OpenRC init installer |
| `sysv.go` | SysV init installer |
| `config.go` | Installer config file generation |
| `secrets.go` | Cryptographic secret generation |

---

## Init System Detection

`init.go` detects the running init system and dispatches to the appropriate installer:

| Init system | File | Service manager |
|-------------|------|-----------------|
| systemd | `linux.go` | `systemctl` |
| OpenRC | `openrc.go` | `rc-update` |
| SysV | `sysv.go` | `update-rc.d` |

---

## Debian Install (`linux.go`)

Interactive installer that:

1. Copies binary → `/usr/local/bin/bedrud`
2. Writes `/etc/bedrud/config.yaml` + `/etc/bedrud/livekit.yaml`
3. Creates init service units (`bedrud.service`, optionally `livekit.service`)
4. Generates secrets (JWT, session, LiveKit API key/secret)
5. Configures TLS:
   - ACME (Let's Encrypt) with `useACME` + `domain` + `email`
   - Self-signed (Ed25519 default)
   - Manual cert/key paths
6. Supports modes:
   - Embedded LiveKit (default)
   - External LiveKit (`livekit.external: true`)
   - Separate LiveKit domain
   - Reverse-proxy mode (`behindProxy: true`)

```bash
sudo bedrud install
```

---

## Uninstall

`DebianUninstall()` / equivalent for other init systems:

1. Stop and disable services
2. Remove service unit files
3. Remove binary from `/usr/local/bin/`
4. Optionally remove `/etc/bedrud/` config and data directories

```bash
sudo bedrud uninstall
```

---

## Generated Config

Installer writes production-ready `config.yaml` with:

- Generated JWT and session secrets
- LiveKit API key/secret pair
- Database path (SQLite default) or PostgreSQL DSN
- TLS configuration based on user choices
- Appropriate `livekit.host` and `livekit.internalHost` URLs

LiveKit YAML written to `/etc/bedrud/livekit.yaml` when using external config.