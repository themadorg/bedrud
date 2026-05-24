---
name: bedrud-realtime
description: Embedded LiveKit binary lifecycle — build tag, config YAML, startup, TLS/TURN setup.
license: Apache License
---

# Bedrud Embedded LiveKit

Go module `bedrud`. `internal/livekit/`.

---

## `embed.go`

`Bin embed.FS` — contains `bin/livekit-server`. Build-tagged `!windows`.

## `config.go`

`ConfigYAML` — shared LiveKit YAML config struct. Used by installer and embedded server startup. `omitempty` on zero-value fields. Fields: `Port`, `BindAddresses`, `Keys`, `RTC` (tcp/udp ports, port range, node_ip), `TURN` (enabled, domain, udp/tls ports, cert/key), `Logging`.

## `server.go`

| Fn | Purpose |
|----|---------|
| `ExportBinary(destPath)` | Write embedded binary 0755. Remove existing first (avoid ETXTBSY) |
| `RunLiveKit(configPath)` | Run synchronously |
| `ResolveNodeIP(explicitIP, serverHost)` | Resolve LK node IP: explicit → parse server.host → detect outbound IP via UDP dial. Returns "" if all fail |
| `generateTempConfig(apiKey, apiSecret, port, nodeIP, certFile, keyFile, serverHost)` | Generate temp YAML with TURN/TLS for embedded mode. Returns temp file path |
| `StartInternalServer(ctx, apiKey, apiSecret, port, cert, key, externalConfig, nodeIP, serverHost)` | Background goroutine, 3s startup sleep. Skip if `LIVEKIT_MANAGED=true`. Generates temp LK YAML with TURN/TLS when cert/key provided |

### TLS Setup

When server TLS enabled: auto-generates temp config with TURN/TLS (port 5349) using server's certificate. TURN `domain` auto-set from `server.host`, UDP port 3478 configured, relative `certFile`/`keyFile` paths resolved to absolute. Set `livekit.nodeIP` / `LIVEKIT_NODE_IP` for explicit RTC node IP (disables STUN). For custom LiveKit YAML, set `livekit.configPath` or `LIVEKIT_CONFIG_PATH`.
