# `bedrud livekit`

Start **only** the embedded LiveKit media server subprocess (no Bedrud API, no database).

**Source:** `server/internal/cli/livekit.go` → `internal/livekit.RunLiveKit()`

---

## Usage

```bash
bedrud livekit
bedrud livekit --config /etc/bedrud/livekit.yaml
bedrud --json livekit
```

**Legacy:** `bedrud --livekit [--config path]`

Note: the `livekit` subcommand has its own `--config` flag for the **LiveKit YAML** file, separate from the root `--config` (Bedrud config).

---

## Flags

| Flag | Scope | Description |
|------|-------|-------------|
| `--config` | Subcommand | Path to LiveKit config YAML |
| `--json` | Root | Startup JSON acknowledgment |

---

## JSON output

```json
{
  "ok": true,
  "message": "starting livekit",
  "data": {
    "livekitConfigPath": ""
  }
}
```

---

## Related

- [run.md](./run.md) — full server including proxied `/livekit`
- [../internal/livekit.md](../internal/livekit.md)