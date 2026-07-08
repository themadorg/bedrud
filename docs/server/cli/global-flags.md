# CLI Global Flags

Available on every `bedrud` subcommand via the root command.

---

## `--config <path>`

Path to the Bedrud `config.yaml` file.

| Env var | Notes |
|---------|-------|
| `BEDRUD_CONFIG` | Bound via Viper |
| `CONFIG_PATH` | Fallback if flag unset |

**Resolution order** (`resolveConfigPath`):

1. `--config` flag
2. `BEDRUD_CONFIG` / Viper `config`
3. `CONFIG_PATH` environment variable
4. Command-specific fallback (`config.yaml` for `run`, `/etc/bedrud/config.yaml` for management commands)
5. `config.yaml` in current directory

```bash
bedrud --config /etc/bedrud/config.yaml user list
CONFIG_PATH=./config.yaml bedrud db status
```

---

## `--json`

Emit structured JSON instead of human-readable text.

### Success (stdout)

```json
{
  "ok": true,
  "message": "optional human message",
  "data": { }
}
```

### Error (stderr)

```json
{
  "ok": false,
  "message": "error description"
}
```

### Examples

```bash
bedrud --json version
bedrud --json user list --config config.yaml
bedrud --json db status
bedrud --json config validate
```

### Long-running commands

`run` and `livekit` print a **startup acknowledgment** JSON line, then block until the process exits. They do not stream logs as JSON.

```bash
bedrud --json run --config config.yaml
# {"ok":true,"message":"starting server","data":{"configPath":"config.yaml","version":"...","skipMigrate":false}}
# … server logs follow in normal text …
```

### Commands with special JSON behavior

| Command | `--json` behavior |
|---------|-------------------|
| `settings show` (no `--json`) | Raw indented JSON (legacy) |
| `settings show --json` | Standard `{ok, data}` envelope |
| `config show` | Masked config as `data` object |
| `user reset-password` | Includes generated `password` in `data` when applicable |

**Implementation:** `server/internal/clioutput/`