# LiveKit Integration (`internal/livekit`)

Manages the embedded LiveKit media server binary and configuration.

**Public docs:** [LiveKit Integration](https://bedrud.org/en/docs/backend/livekit) · [WebRTC Connectivity](https://bedrud.org/en/docs/architecture/webrtc-connectivity) · [TURN Server](https://bedrud.org/en/docs/architecture/turn-server) — [`backend/livekit.mdx`](../../../apps/site/src/content/docs/en/backend/livekit.mdx). Full map: [../public-docs.md](../public-docs.md).

---

## Files

| File | Purpose |
|------|---------|
| `embed.go` | `//go:embed bin/livekit-server` (build tag `!windows`) |
| `embed_windows.go` | Windows stub (no embedded binary) |
| `config.go` | `ConfigYAML` struct for LiveKit YAML generation |
| `server.go` | Binary export, startup, TURN/TLS config |

---

## Binary embedding (`embed.go`)

The pre-compiled `livekit-server` executable is embedded at build time:

```
internal/livekit/bin/livekit-server  →  embed.FS  →  extracted at runtime
```

CI placeholder: `touch internal/livekit/bin/livekit-server` (empty file satisfies build).

### `ExportBinary(destPath)`

Writes embedded binary with `0755` permissions. Removes existing file first (avoids `ETXTBSY`).

---

## ConfigYAML (`config.go`)

Shared LiveKit YAML config struct. Used by installer and embedded server startup. Fields use `omitempty` so zero values are omitted and LiveKit uses defaults.

```go
type ConfigYAML struct {
    Port           int
    BindAddresses  []string
    Keys           map[string]string
    RTC            RTCConfig    // tcp/udp ports, port range, node_ip
    TURN           TURNConfig   // enabled, domain, udp/tls ports, cert/key
    Logging        LoggingConfig
}
```

---

## Server lifecycle (`server.go`)

### `StartInternalServer(ctx, apiKey, apiSecret, port, cert, key, externalConfig, nodeIP, serverHost)`

Background goroutine that:

1. Skips if `LIVEKIT_MANAGED=true` (external management)
2. Exports binary to temp path
3. Generates temp YAML with TURN/TLS when server TLS enabled
4. Launches subprocess with 3-second startup sleep
5. Falls back to inline `--port`/`--keys` args if no TLS

### `RunLiveKit(configPath)`

Run synchronously (for `bedrud livekit` CLI command).

### `ResolveNodeIP(explicitIP, serverHost)`

Resolution order:

1. Explicit `nodeIP` from config
2. Parse `server.host` if valid non-loopback IP
3. Detect outbound IP via UDP dial
4. Return `""` if all fail (LiveKit uses STUN)

### `generateTempConfig(...)`

When TLS enabled on the main server:

- `TURN.Domain` = `serverHost`
- `TURN.UDPPort` = 3478
- `TURN.TLSPort` = 5349
- Uses server's certificate files
- When `nodeIP` set: `UseExternalIP = false`, `NodeIP = nodeIP`

---

## Reverse proxy

In `internal/server/server.go`, requests to `/livekit/*` are reverse-proxied to `livekit.internalHost` (default `http://127.0.0.1:7880`). The `/livekit` prefix is stripped.

### External mode

When `livekit.external: true`:

- Embedded server not started
- `/livekit` proxy disabled
- Handlers connect directly to `livekit.internalHost`

---

## Webhook

Embedded LiveKit auto-configures webhook URL:

```
http://localhost:<httpPort>/api/livekit/webhook
```

For external LiveKit (Cloud or self-hosted), configure manually in the LiveKit dashboard using the same API key/secret.

---

## Environment variables

| Variable | Purpose |
|----------|---------|
| `LIVEKIT_HOST` | Public LiveKit URL |
| `LIVEKIT_INTERNAL_HOST` | Internal URL for proxy/client |
| `LIVEKIT_API_KEY` | API key |
| `LIVEKIT_API_SECRET` | API secret |
| `LIVEKIT_CONFIG_PATH` | External YAML config |
| `LIVEKIT_NODE_IP` | Explicit RTC node IP |
| `LIVEKIT_MANAGED` | Skip embedded server startup |