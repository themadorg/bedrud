---
name: bedrud-realtime
description: Embedded LiveKit binary lifecycle — embed FS, config YAML, startup, TURN/TLS, node IP, webhook auto-config.
license: Apache License
---

# Bedrud Embedded LiveKit

Go module `bedrud`. Package `server/internal/livekit/`.

Related (not this package): `internal/lkutil/` (RoomService/Egress clients + data msgs), `handlers` LiveKit webhook receiver `POST /api/livekit/webhook`, installer writes `/etc/bedrud/livekit.yaml`.

---

## Package layout

| File | Build | Purpose |
|------|-------|---------|
| `embed.go` | `!windows` | `//go:embed bin/livekit-server` → `Bin embed.FS` |
| `embed_windows.go` | `windows` | `//go:embed bin/livekit-server.exe` → `Bin embed.FS` |
| `config.go` | all | `ConfigYAML` — LiveKit YAML shape (installer + temp config) |
| `server.go` | all | Export binary, resolve path, run/start process, node IP, temp config |
| `bin/livekit-server` | asset | Placeholder or real binary (CI may `touch` empty). Windows: `.exe` |

**Embed constants** (per platform file):

| Const | Unix | Windows |
|-------|------|---------|
| `lkBinKey` | `bin/livekit-server` | `bin/livekit-server.exe` |
| `lkExeName` | `bedrud-livekit-server` | `bedrud-livekit-server.exe` |

Build: `make init` / `livekit-download` copies release binary into `bin/`. Without it, `go:embed` still needs the path present.

---

## `config.go` — `ConfigYAML`

Shared YAML struct for LiveKit server config. Used by:

1. **Installer** (`internal/install/linux.go`) → `/etc/bedrud/livekit.yaml`
2. **Embedded startup** `generateTempConfig` → temp `bedrud-livekit-*.yaml`

`omitempty` on zero-value optional fields so LiveKit defaults apply.

| Field | YAML | Notes |
|-------|------|-------|
| `Port` | `port` | Signaling HTTP port |
| `BindAddresses` | `bind_addresses` | omitempty |
| `Keys` | `keys` | `map[apiKey]apiSecret` |
| `RTC.TCPPort` | `rtc.tcp_port` | omitempty |
| `RTC.UDPPort` | `rtc.udp_port` | omitempty |
| `RTC.PortRangeStart/End` | `rtc.port_range_*` | omitempty |
| `RTC.UseExternalIP` | `rtc.use_external_ip` | **no omitempty** — always serialized |
| `RTC.NodeIP` | `rtc.node_ip` | **no omitempty** |
| `TURN.Enabled` | `turn.enabled` | omitempty |
| `TURN.Domain` | `turn.domain` | omitempty; from `server.host` in temp config |
| `TURN.UDPPort` | `turn.udp_port` | omitempty; **3478** in temp config |
| `TURN.TLSPort` | `turn.tls_port` | omitempty; **5349** in temp config |
| `TURN.CertFile/KeyFile` | `turn.cert_file` / `key_file` | omitempty; server TLS certs |
| `Webhook.URLs` | `webhook.urls` | auto: `http://localhost:<httpPort>/api/livekit/webhook` |
| `Webhook.APIKey` | `webhook.api_key` | same as LiveKit API key |
| `Logging.JSON` | `logging.json` | omitempty |
| `Logging.Level` | `logging.level` | omitempty |

Installer path also sets RTC ports/range, bind addresses, TURN domain/ports, logging — not via `generateTempConfig`.

---

## `server.go` — API

### Binary export & path resolution

| Fn | Purpose |
|----|---------|
| `ExportBinary(destPath)` | `Bin.ReadFile(lkBinKey)` → `os.Remove(dest)` first (avoid **ETXTBSY**) → write `0755` |
| `resolveLiveKitPath()` | Try export candidates; fall back to bare `lkExeName` (PATH) |

**Export candidates** (first success wins):

1. `$TMPDIR/<lkExeName>` (`tempDirPath`)
2. `UserCacheDir()/bedrud/<lkExeName>` (`userCachePath`)
3. `dir(os.Executable())/<lkExeName>` (`exeDirPath`)
4. `cwd/<lkExeName>` (`cwdPath`)

Each: `MkdirAll` parent → `ExportBinary` → `Chmod 0755`.

### Process entrypoints

| Fn | Sync? | Purpose |
|----|-------|---------|
| `RunLiveKit(configPath)` | **sync** `cmd.Run()` | Standalone CLI: `bedrud livekit` / `--livekit`. Args: optional `--config <path>` |
| `StartInternalServer(ctx, apiKey, apiSecret, port, certFile, keyFile, externalConfigPath, nodeIP, serverHost, httpPort)` | **async** goroutine | Embedded under `server.Run`. 3s sleep then return |

**`StartInternalServer` flow:**

1. If `LIVEKIT_MANAGED=true` → log skip, return `nil` (systemd/OpenRC/SysV runs separate `livekit.service`)
2. `resolveLiveKitPath()`
3. Config selection:
   - If `externalConfigPath` set → `--config <path>` (no temp YAML, **no auto webhook/TURN**)
   - Else `generateTempConfig(...)` → temp YAML; on failure fall back to inline `--port` + `--keys "key: secret"`
4. `exec.CommandContext(ctx, lkPath, args...)` — stdout/stderr inherited
5. Goroutine: `cmd.Run()`; on exit remove temp config file if any
6. `time.Sleep(3s)` — crude ready wait; always returns `nil` after start attempt

### Node IP

```
ResolveNodeIP(explicitIP, serverHost) string
  1. explicitIP if non-empty
  2. ParseIP(serverHost) if valid, non-loopback, non-unspecified
  3. utils.OutboundIP() (UDP dial 8.8.8.8:80) if non-loopback/unspecified
  4. ""
```

### Temp config — `generateTempConfig`

```
generateTempConfig(apiKey, apiSecret, port, nodeIP, certFile, keyFile, serverHost, httpPort) (path, error)
```

| Setting | Behavior |
|---------|----------|
| `Port` / `Keys` | Always set |
| `nodeIP` set | Validate IP; `RTC.UseExternalIP=false`, `RTC.NodeIP=nodeIP` (STUN off). Invalid IP → error |
| `nodeIP` empty | `RTC.UseExternalIP=true` + warn (STUN; may fail air-gapped/firewalled) |
| `certFile` **and** `keyFile` non-empty | TURN on: TLS **5349**, UDP **3478**, cert/key paths; `TURN.Domain=serverHost` if host set |
| `httpPort` non-empty | Webhook: `urls: [http://localhost:<httpPort>/api/livekit/webhook]`, `api_key: apiKey` |
| File | `os.CreateTemp("", "bedrud-livekit-*.yaml")` |

---

## Bootstrap caller — `internal/server/server.go`

Embedded LK starts only when:

```go
useInternalLK := !cfg.LiveKit.External &&
  (strings.Contains(internalHost, "localhost") || strings.Contains(internalHost, "127.0.0.1"))
```

(`internalHost` = lowercased `cfg.LiveKit.InternalHost`.)

| Step | Detail |
|------|--------|
| Secret length | `APISecret` must be ≥ 32 chars or `Run` errors with openssl hint |
| Certs for TURN | Only if `EnableTLS && !DisableTLS`: resolve `CertFile`/`KeyFile` (default `/etc/bedrud/cert.pem` + `key.pem`); non-default paths → `filepath.Abs` |
| Keypair gen | If `APIKey == ""`: `utils.GenerateLiveKitKeypair()` into cfg (in-memory only) |
| Node IP | `ResolveNodeIP(cfg.LiveKit.NodeIP, cfg.Server.Host)` |
| Start | `StartInternalServer(ctx, apiKey, apiSecret, **7880**, cert, key, cfg.LiveKit.ConfigPath, nodeIP, cfg.Server.Host, cfg.Server.HTTPPort)` |

**Hardcoded embedded RTC/signaling port: `7880`.** Dev stack often uses separate `livekit-server --config server/livekit.yaml` (e.g. 7072) via `make dev-livekit` / devcli — not this package.

If `cfg.LiveKit.External`: log external host; do not start subprocess.

---

## App config — `config.LiveKitConfig`

| YAML | Env | Role |
|------|-----|------|
| `host` | `LIVEKIT_HOST` | Browser signaling URL |
| `hostLocal` | `LIVEKIT_HOST_LOCAL` | Localhost browser override |
| `internalHost` | `LIVEKIT_INTERNAL_HOST` | API client base URL; gates embedded start |
| `apiKey` | `LIVEKIT_API_KEY` | Keys + webhook JWT |
| `apiSecret` | `LIVEKIT_API_SECRET` | ≥32 for embedded |
| `configPath` | `LIVEKIT_CONFIG_PATH` | External LK YAML → skip temp config |
| `skipTLSVerify` | `LIVEKIT_SKIP_TLS_VERIFY` | Twirp client TLS skip (`lkutil`) |
| `external` | `LIVEKIT_EXTERNAL` | Skip embed + `/livekit` proxy intent |
| `nodeIP` | `LIVEKIT_NODE_IP` | Explicit RTC node IP |

Also: `LIVEKIT_MANAGED=true` (process env, not yaml) — set by installer on main bedrud service when separate `livekit.service` owns the binary.

---

## CLI entrypoints

| Form | Implementation |
|------|----------------|
| `bedrud livekit --config <path>` | `cli.newLiveKitCmd` → `RunLiveKit` |
| `bedrud --livekit --config <path>` | `cli.dispatchLegacy` → `RunLiveKit` (systemd unit form) |
| Installer unit | `ExecStart=... bedrud --livekit --config /etc/bedrud/livekit.yaml` + main service `Environment=LIVEKIT_MANAGED=true` |

---

## TURN / TLS summary

| Mode | TURN |
|------|------|
| Embedded + server TLS on + no `configPath` | Auto: TURN TLS 5349 + UDP 3478, certs from server, domain = `server.host` |
| Embedded + no TLS | No TURN in temp config |
| `configPath` / external YAML | Caller-owned (installer always enables TURN; TLS ports only if install TLS) |
| External LiveKit | Operator configures TURN + webhook in LK dashboard |

Webhook for **external** LK: manually set URL to `https://<domain>/api/livekit/webhook` (same apiKey/secret for JWT). Embedded auto-wires only when temp config is generated (`httpPort` from `server.httpPort`).

---

## Gotchas

- **Placeholder binary:** empty `bin/livekit-server` builds but process fails at run — run `make init` / `livekit-download`.
- **ETXTBSY:** always unlink before overwrite when re-exporting running binary.
- **External config skips auto webhook:** if `livekit.configPath` set, no `generateTempConfig` → no auto webhook/TURN from bedrud.
- **Webhook URL is HTTP localhost:** uses `http://localhost:<httpPort>/api/livekit/webhook` even when main app is HTTPS.
- **3s sleep ≠ health check:** `StartInternalServer` does not probe readiness.
- **Managed install dual process:** `LIVEKIT_MANAGED` prevents double-start; LK runs as sibling service via `RunLiveKit`.
- **STUN vs node IP:** empty `ResolveNodeIP` → `use_external_ip: true`; set `livekit.nodeIP` / `LIVEKIT_NODE_IP` for Docker/air-gap.
- **Clients:** API uses `lkutil.NewClient` → `InternalHost` then `Host`; not this package.
