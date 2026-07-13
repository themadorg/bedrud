---
name: bedrud-ops-cli
description: Operational tooling — cobra CLI surface, Linux installer, user/room management, TLS certs, config/settings, invite tokens.
license: Apache License
---

# Bedrud Ops & CLI

Go module `bedrud`. Production entrypoint `cmd/bedrud` → `internal/cli` (cobra). Domain logic in `install/`, `usercli/`, `roomcli/`, `utils/`, `clioutput/`.

---

## Entrypoint

### `cmd/bedrud/main.go`

```go
var version = "dev"  // -ldflags by mage
func main() { cli.Execute(version) }
```

### `internal/cli/` — Cobra surface

| File | Purpose |
|------|---------|
| `root.go` | `NewRootCmd()`, `Execute()`, `resolveConfigPath()`, version cmd |
| `legacy.go` | Pre-cobra: `--version`/`-v`, `--livekit`, `--run` (systemd compat) |
| `run.go` | `run` / `server` — start meeting server |
| `livekit.go` | `livekit` — start embedded LiveKit |
| `install.go` | `install` / `uninstall` |
| `update.go` | `update` / `upgrade` (in-place binary + migrations + restart) |
| `cert.go` | `certificate`/`cert` → `regenerate` / `renew` / `info` (WebXDC wildcard SANs) |
| (install) | WebXDC + `--cloudflare-token` → ACME DNS-01 free `*.domain` via Cloudflare |
| `user.go` | `user *` → `usercli` |
| `room.go` | `room *` → `roomcli` |
| `config.go` | `config path|show|get|set|validate` |
| `settings.go` | `settings show|set|reset` (DB runtime settings) |
| `invite.go` | `invite-token` / `invite` list|create|delete |
| `db.go` | `db migrate` / `db status` |
| `runtime.go` | empty placeholder |
| `cli_test.go` | version/config/user/db JSON tests |

### Root flags / config resolution

| Flag / env | Purpose |
|------------|---------|
| `--config` | Config path (persistent) |
| `--json` | Machine-readable JSON via `clioutput` |
| `BEDRUD_CONFIG` / `CONFIG_PATH` | Env overrides (viper) |

`resolveConfigPath(fallback)` order: flag → viper `config` → `CONFIG_PATH` → fallback → `config.yaml`.

| Context | Default config |
|---------|----------------|
| `run` / legacy `--run` | `config.yaml` |
| Management cmds (user, room, cert, config, settings, invite, db) | `/etc/bedrud/config.yaml` |

---

## CLI command map

```
bedrud [--config PATH] [--json]
├── run | server [--skip-migrate]     → server.Run
├── livekit [--config LK_YAML]        → livekit.RunLiveKit
├── install [flags...]                → install.LinuxInstall
├── uninstall                         → install.LinuxUninstall
├── update | upgrade [flags...]       → install.LinuxUpdate
├── certificate | cert
│   ├── regenerate [--algo ALGO] [--force]   # SANs + WebXDC *.baseDomain
│   ├── renew [--algo ALGO]                  # same SAN rebuild as regenerate
│   └── info
├── user
│   ├── create --email --password --name [--admin]
│   ├── delete --email
│   ├── promote --email               → role superadmin
│   ├── demote --email                → remove superadmin
│   ├── list [--page] [--page-size]
│   ├── info --email
│   ├── password --email --password
│   ├── reset-password --email        → generate random pwd
│   ├── enable --email
│   └── disable --email
├── room
│   ├── list [--page] [--page-size] [--active]
│   ├── info <id-or-name>
│   ├── close <id-or-name> --yes
│   ├── suspend <id-or-name>
│   ├── reactivate <id-or-name>
│   └── kick <id-or-name> --identity ID
├── config
│   ├── path | show | get <key> | set <key> <value> | validate
├── settings
│   ├── show [--effective] | set <jsonField> <value> | reset --yes
├── invite-token | invite
│   ├── list [--page] [--page-size]
│   ├── create [--email] [--created-by] [--ttl-hours=168]
│   └── delete <id>
├── db
│   ├── migrate | status
└── version
```

### Legacy (pre-cobra, still supported)

| Invocation | Behavior |
|------------|----------|
| `bedrud --version` / `-v` | Print version |
| `bedrud --livekit --config <path>` | Run LiveKit |
| `bedrud --run --config <path> [--skip-migrate]` | Run server |
| `--json` on any of above | JSON envelope |

### Install flags (`bedrud install`)

| Flag | Field / effect |
|------|----------------|
| `--tls` | Enable TLS (alias self-signed path) |
| `--self-signed` | Generate self-signed cert |
| `--no-tls` | Disable TLS (overrides --tls/--self-signed) |
| `--ip` | Override detected IP |
| `--domain` / `--email` | Domain + Let's Encrypt email → ACME when both set |
| `--port` | Override (default 443 TLS / 8090 HTTP) |
| `--cert` / `--key` | Existing cert pair |
| `--lk-port` / `--lk-tcp-port` / `--lk-udp-port` | LK ports (7880/7881/7882) |
| `--lk-udp-range start-end` | WebRTC UDP range (default 50000-60000) |
| `--fresh` | Uninstall first |
| `--behind-proxy` | CDN/reverse-proxy mode |
| `--external-livekit URL` | External LK (no local livekit service) |
| `--livekit-domain` | Separate domain for local LK |
| `--lk-ip` | LiveKit NodeIP when server behind CDN |
| `--cert-algorithm` | `ed25519` (default), `ecdsa256`, `rsa2048`, `rsa4096` |

### Cert key algorithms

`ed25519` | `ecdsa256` | `rsa2048` | `rsa4096`

---

## `internal/clioutput/` — JSON output envelope

| Export | Purpose |
|--------|---------|
| `SetJSON(bool)` / `JSON()` | Toggle machine mode (root `--json`) |
| `Success(message, data)` | JSON `{ok,message,data}` or print message |
| `Printf` / `Println` | No-op when JSON mode |
| `Emit` / `EmitResult` / `EmitError` | Raw/result/error JSON |
| `Result{OK, Message, Data}` | Envelope struct |
| `SetWriters` / `ResetWriters` | Test hooks |

---

## `internal/install/` — Linux installer

Entry APIs: **`LinuxInstall` / `LinuxUninstall` / `LinuxUpdate`** (not Debian*).

| File | Purpose |
|------|---------|
| `linux.go` | Install/uninstall flow, interactive prompt, binary copy, config write, user creation |
| `update.go` | In-place upgrade: binary replace, version + DB migrations, service restart |
| `version.go` | `/var/lib/bedrud/version` + ordered versioned install migrations |
| `binary.go` | Self-binary copy, package-managed detection, chown helpers |
| `services.go` | Shared service unit rewrite + enable/start (`refreshServices`) |
| `paths.go` | Standard path constants (`/etc/bedrud`, `/usr/local/bin`, …) |
| `config.go` | `InstallConfig` struct + `SetDefaults()` (+ `Version` for version file) |
| `init.go` | Init detection (systemd / OpenRC / SysV / none=container), service enable/start, stop/disable |
| `openrc.go` | OpenRC init scripts for bedrud + livekit |
| `sysv.go` | SysV init scripts |
| `secrets.go` | `generateSecret(n)` — crypto/rand → base64 URL |

### `LinuxUpdate(opts)` flow

1. Require existing config (`/etc/bedrud/config.yaml` or `--config`)
2. Stop bedrud/livekit
3. Replace binary (skip package-managed `/usr/bin` when self is already that path; else `/usr/local/bin`)
4. Run versioned install migrations (`versionMigrations`) when previous → new crosses them
5. `database.RunMigrations` (unless `--skip-migrate`)
6. `refreshServices` — rewrite units from LiveKit topology, enable+start
7. Write `/var/lib/bedrud/version`

Flags: `--skip-binary`, `--skip-migrate`, `--skip-restart`. CLI: `update` ≡ `upgrade`.

### `InstallConfig`

```
EnableTLS, DisableTLS, SelfSigned, OverrideIP, Domain, Email, Port,
CertPath, KeyPath, LKPort, LKTcpPort, LKUdpPort,
LKUDPPortRangeStart, LKUDPPortRangeEnd, Fresh, BehindProxy,
ExternalLKURL, LKDomain, LKIP, CertAlgorithm
```

Defaults: port 443/8090, LK 7880/7881/7882, UDP range 50000–60000.

### `LinuxInstall(cfg)` flow

1. Optional `--fresh` → `LinuxUninstall`
2. Linux-only; interactive `promptConfig` when stdin is a TTY
3. `SetDefaults`; detect outbound IP if unset
4. Mkdir `/etc/bedrud`, `/var/lib/bedrud{,/certs}`, `/var/log/bedrud`
5. Create system user `bedrud` (nologin, home `/var/lib/bedrud`)
6. Stop bedrud/livekit; copy self binary → `/usr/local/bin/bedrud`
7. Generate secrets (LK apiKey/apiSecret, JWT, session)
8. Write `/etc/bedrud/config.yaml` (sqlite at `/var/lib/bedrud/bedrud.db`)
9. Write `/etc/bedrud/livekit.yaml` unless external LK (TURN, NodeIP, UDP range)
10. Self-signed cert if TLS + no provided cert (`GenerateSelfSignedCertWithAlgo`)
11. Detect init → write + enable + start services (skip in containers)

Paths: binary `/usr/local/bin/bedrud`, config `/etc/bedrud/config.yaml`, LK yaml `/etc/bedrud/livekit.yaml`, certs `/etc/bedrud/cert.pem` + `key.pem`.

### Init systems (`init.go`)

| Detected | Behavior |
|----------|----------|
| Container (dockerenv / containerd PID1 / cgroup) | `none` — print manual start instructions |
| `systemctl` present | systemd units under `/etc/systemd/system/` |
| `/sbin/openrc` | OpenRC scripts in `/etc/init.d/` |
| else | SysV scripts in `/etc/init.d/` |

Services: `livekit` (if not external) + `bedrud`. Systemd uses legacy `bedrud --livekit --config ...` for LK unit; bedrud unit uses `bedrud run --config ...` + `LIVEKIT_MANAGED=true` when local LK.

### `LinuxUninstall()`

Stop/disable all init systems → remove service files → remove binary + tmp binaries → remove `/etc/bedrud`, `/var/lib/bedrud`, `/var/log/bedrud` → `userdel -r bedrud`.

---

## `internal/usercli/usercli.go` — CLI user management

All ops: load config → `database.Initialize` → `RunMigrations` → act. Output via `clioutput`.

| Fn | Purpose |
|----|---------|
| `PromoteUser(configPath, email, role)` | Set accesses via `roleAccessSlice(role)` (CLI always passes `superadmin`) |
| `DemoteUser(configPath, email, role)` | Remove role (default `superadmin`); ensure at least `user` remains |
| `CreateUser(configPath, email, password, name, admin)` | bcrypt via `auth.HashPassword`; skip if exists; `--admin` → superadmin accesses |
| `DeleteUser(configPath, email)` | Cascade: rooms via `RoomCleanupService.DeleteUserRooms` → passkeys → prefs → user |
| `ListUsers(configPath, page, pageSize)` | Paginated (default page=1, size=50) |
| `ShowUser(configPath, email)` | Detail dump / JSON |
| `SetUserPassword(configPath, email, newPassword)` | Empty password → generate 20-char; clears refresh token |
| `SetUserActive(configPath, email, active)` | Enable/disable + clear token on change |
| `withUser(configPath, email, fn)` | Shared load+lookup helper |

`roleAccessSlice`: `superadmin|admin|moderator` → `{role,"user"}`; `user` → `{"user"}`; `guest` → `{"guest"}`.

---

## `internal/roomcli/roomcli.go` — CLI room management

| Fn | Purpose |
|----|---------|
| `ListRooms(configPath, page, pageSize, activeOnly)` | Paginated or all active (`--active` ignores pagination) |
| `ShowRoom(configPath, roomID)` | Detail + active participant count (ID or name) |
| `CloseRoom(configPath, roomID)` | `CascadeDeleteRoom` (system event `room_deleted`) — requires CLI `--yes` |
| `SuspendRoom(configPath, roomID)` | `SuspendRoom` (disconnect, keep DB) |
| `ReactivateRoom(configPath, roomID)` | `IsActive=true`, `ExpiresAt=+24h` |
| `KickParticipant(configPath, roomID, identity)` | LiveKit remove + DB kick |

Helpers: `withRepo`, `getRoomByIDOrName`, `buildCleanupService` (LK client + chat upload tracker / optional S3 deleter).

---

## `internal/utils/` — Shared utilities

### `net.go`

| Fn | Purpose |
|----|---------|
| `OutboundIP() net.IP` | UDP dial `8.8.8.8:80` → local IP; fallback `127.0.0.1` |
| `DisplayAddr(host, port)` | Format display address (resolves `0.0.0.0`/empty via OutboundIP) |

### `keys.go`

| Fn | Purpose |
|----|---------|
| `GenerateLiveKitKeypair() (key, secret, err)` | `gen-` + 16B hex key, 32B hex secret |

> Note: older `GenerateAPIKey` / `GenerateSecret` APIs are gone; install uses private `generateSecret` in `install/secrets.go`.

### `safeio.go`

| Fn | Purpose |
|----|---------|
| `SafeCreate(path, perm)` | O_EXCL create; rejects existing; validates parent chain (no symlink escapes) |
| `SafeOpenAppend(path, perm)` | Append or create; regular file only |

### `tls.go`

| Export | Purpose |
|--------|---------|
| `CertWarnDays = 30` | Expiry warn / auto-renew threshold |
| `SelfSignedCertDays = 1825` | ~5 year validity |
| `KeyAlgorithm` | `ed25519` (default), `ecdsa256`, `rsa2048`, `rsa4096` |
| `GenerateSelfSignedCert(cert, key, hosts...)` | Ed25519 wrapper |
| `GenerateSelfSignedCertWithAlgo(...)` | Explicit algo; uses `SafeCreate` |
| `RenewSelfSignedCert(...)` | Detect algo from existing cert, preserve |
| `RenewSelfSignedCertWithAlgo(...)` | Explicit algo; atomic `.new` + rename |
| `DetectCertAlgorithm(certFile)` | Read PEM → ed25519 / ecdsa256 / rsa2048 |
| `ParseSanHosts(hosts...)` | Split DNS vs IP SANs (supports `*.domain` wildcards) |
| `ValidateTLSCertPair(cert, key)` | Parse, match key, check validity → `*CertInfo` |
| `CertInfo` | Subject, Issuer, NotBefore/After, DaysRemaining, SANs, Status (`valid`/`expiring`) |

**CLI SANs** (`buildCertSANHosts`): domain, host, outbound IP, localhost; when WebXDC active and not path-mode → `baseDomain` + `*.baseDomain`.

Internal: `keyUsageForAlgo` (RSA → DigitalSignature\|KeyEncipherment; else DigitalSignature), `generateKey`.

### `email.go`

| Fn | Purpose |
|----|---------|
| `SendSMTP(addr, auth, from, to, msg, host, tlsSkipVerify, smtpsMode)` | SMTPS / STARTTLS / plain |
| `BuildMessage(from, fromName, to, subject, bodyHTML, bodyPlain)` | multipart/alternative MIME |

---

## Config path & secrets conventions

- Installed: `/etc/bedrud/config.yaml` (mode 0600), secrets never printed on install success
- `config show` / `settings show` redact: jwtSecret, sessionSecret, apiSecret, clientSecret, secretKey, accessKey, password (+ OAuth/LiveKit/S3 secrets in settings)
- `config validate` checks: jwtSecret (≥32 recommended), sessionSecret, database.type/path, server.port

---

## Dependency graph

```
cmd/bedrud
  └─ internal/cli
       ├─ server.Run / livekit.RunLiveKit
       ├─ install.LinuxInstall / LinuxUninstall / LinuxUpdate
       ├─ usercli  → config, database, auth, repository, services, storage, lkutil, clioutput
       ├─ roomcli  → config, database, repository, services, storage, lkutil, clioutput
       ├─ utils    (cert renew/info, OutboundIP)
       ├─ database (db migrate/status, invite, settings)
       └─ clioutput
install → utils (TLS, OutboundIP), livekit.ConfigYAML
```
