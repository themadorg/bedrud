# `bedrud config`

Read and write the **YAML config file** (`config.yaml`). Does not modify DB `SystemSettings`.

**Source:** `server/internal/cli/config.go`

---

## Subcommands

### `config path`

Print resolved config file path.

```bash
bedrud config path
bedrud --json config path
```

---

### `config show`

Dump full configuration as YAML (secrets redacted: `jwtSecret`, `apiSecret`, etc.).

```bash
bedrud config show
bedrud --json config show
```

With `--json`, `data` is the masked config object (not YAML text).

---

### `config get <key>`

Read a single dotted key from YAML (Viper paths).

```bash
bedrud config get server.port
bedrud config get database.type
bedrud --json config get auth.jwtSecret
```

---

### `config set <key> <value>`

Write a value to the config file on disk. Coerces `true`/`false` to booleans.

```bash
bedrud config set server.httpPort "8080"
bedrud config set logger.level debug
bedrud --json config set server.port "8090"
```

---

### `config validate`

Parse config and check required fields (`jwtSecret`, `sessionSecret`, `database.type`, etc.).

```bash
bedrud config validate
bedrud --json config validate
```

On failure with `--json`, returns error envelope with problem list in message.

On success:

```json
{
  "ok": true,
  "message": "✓ Config OK: /etc/bedrud/config.yaml",
  "data": { "path": "...", "problems": [] }
}
```

---

## Related

- [settings.md](./settings.md) — runtime DB settings
- [../configuration.md](../configuration.md) — full reference