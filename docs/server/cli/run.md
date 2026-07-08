# `bedrud run`

Start the full Bedrud meeting server: HTTP API, embedded LiveKit (unless external), queue worker, scheduler, and embedded React SPA.

**Alias:** `server`  
**Source:** `server/internal/cli/run.go` → `internal/server/server.go` `Run()`

---

## Usage

```bash
bedrud run
bedrud run --config /etc/bedrud/config.yaml
bedrud --json run --config config.yaml
CONFIG_PATH=/path/config.yaml bedrud run
```

**Legacy:** `bedrud --run [--config path] [--skip-migrate]`

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `config.yaml` | Bedrud config file (root persistent flag) |
| `--json` | false | Startup acknowledgment as JSON |
| `--skip-migrate` | false | Set `BEDRUD_SKIP_MIGRATE=1` before boot |

---

## What starts

1. Load config, validate secrets
2. Embedded LiveKit (if not external)
3. Database + migrations (unless skipped)
4. Queue worker, scheduler, auth
5. Fiber HTTP listener + TLS/ACME
6. Embedded frontend from `ui.go`

See [../bootstrap.md](../bootstrap.md).

---

## JSON output

With `--json`, prints once before blocking:

```json
{
  "ok": true,
  "message": "starting server",
  "data": {
    "configPath": "config.yaml",
    "version": "1.0.0",
    "skipMigrate": false
  }
}
```

Subsequent server logs remain plain text on stdout/stderr.

---

## Related

- [livekit.md](./livekit.md) — LiveKit-only mode
- [db.md](./db.md) — run migrations separately
- [../configuration.md](../configuration.md)