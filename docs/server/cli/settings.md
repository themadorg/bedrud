# `bedrud settings`

Manage the **`SystemSettings`** singleton in the database (registration flags, OAuth, quotas, email branding).

**Source:** `server/internal/cli/settings.go`

Distinct from [config.md](./config.md) which edits `config.yaml` on disk.

---

## Subcommands

### `settings show`

```bash
bedrud settings show
bedrud settings show --effective
bedrud --json settings show
bedrud --json settings show --effective
```

| Flag | Description |
|------|-------------|
| `--effective` | Merge DB values with `config.yaml` defaults |

**Without root `--json`:** prints raw indented JSON (secrets redacted).

**With `--json`:** standard envelope; `data.settings` + `data.effective` flag.

---

### `settings set <jsonField> <value>`

Set one field by JSON tag name from `SystemSettings` model.

```bash
bedrud settings set registrationEnabled false
bedrud settings set guestLoginEnabled true
bedrud --json settings set serverName "My Instance"
```

Reloads OAuth providers via `auth.ReloadProviders()` after save.

---

### `settings reset`

Reset settings row to zero values. **Destructive.**

```bash
bedrud settings reset --yes
bedrud --json settings reset --yes
```

| Flag | Required |
|------|----------|
| `--yes` | yes |

---

## Related

- [../settings-system.md](../settings-system.md)
- [../database-schema.md](../database-schema.md) — `system_settings` table