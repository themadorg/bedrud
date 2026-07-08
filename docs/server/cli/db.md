# `bedrud db`

Database utilities: migrations and connectivity check.

**Source:** `server/internal/cli/db.go`

---

## Subcommands

### `db migrate`

Run GORM auto-migrations (`internal/database/migrations.go`).

```bash
bedrud db migrate
bedrud db migrate --config /etc/bedrud/config.yaml
bedrud --json db migrate
```

**JSON `data`:** `databaseType` (e.g. `sqlite`, `postgres`)

Equivalent to startup migrations unless `BEDRUD_SKIP_MIGRATE=1` on `bedrud run`.

---

### `db status`

Ping database and print connection info.

```bash
bedrud db status
bedrud --json db status
```

**JSON `data`:**

| Field | SQLite | Postgres |
|-------|--------|----------|
| `type` | `sqlite` | `postgres` |
| `status` | `ok` | `ok` |
| `path` | db file path | — |
| `host`, `port`, `dbname` | — | connection info |

---

## Related

- [../database-schema.md](../database-schema.md)
- [../internal/database.md](../internal/database.md)
- [run.md](./run.md) — migrations on startup