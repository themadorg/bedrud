# `bedrud user`

Manage local users in the database. Requires DB access via `--config`.

**Source:** `server/internal/cli/user.go` → `internal/usercli/`

Default config: `/etc/bedrud/config.yaml`

---

## Subcommands

### `user create`

Create a local user with bcrypt password.

```bash
bedrud user create --email alice@example.com --password 'secret' --name "Alice"
bedrud user create --email admin@example.com --password 'secret' --name "Admin" --admin
bedrud --json user create --email a@ex.com --password 'secret' --name "A"
```

| Flag | Required | Description |
|------|----------|-------------|
| `--email` | yes | User email (unique per provider) |
| `--password` | yes | Plain password (hashed at rest) |
| `--name` | yes | Display name |
| `--admin` | no | Grant `superadmin` + `user` accesses |

**JSON `data`:** `created` (bool), `user` object (`id`, `email`, `name`, `provider`, `active`, `accesses`)

---

### `user delete`

Delete user and cascade: owned rooms (LiveKit + uploads), passkeys, preferences.

```bash
bedrud user delete --email alice@example.com
bedrud --json user delete --email alice@example.com
```

---

### `user promote`

Grant superadmin access (`superadmin` + `user`).

```bash
bedrud user promote --email alice@example.com
```

**JSON `data`:** `email`, `role`, `accesses`

---

### `user demote`

Remove `superadmin` access; ensures at least `user` remains.

```bash
bedrud user demote --email alice@example.com
```

---

### `user list`

Paginated user table.

```bash
bedrud user list
bedrud user list --page 2 --page-size 25
bedrud --json user list
```

| Flag | Default | Description |
|------|---------|-------------|
| `--page` | 1 | Page number (1-indexed) |
| `--page-size` | 50 | Users per page |

**JSON `data`:** `users[]`, `page`, `pageSize`, `total`

---

### `user info`

Full details for one user.

```bash
bedrud user info --email alice@example.com
bedrud --json user info --email alice@example.com
```

---

### `user password`

Set password and invalidate refresh tokens (force re-login).

```bash
bedrud user password --email alice@example.com --password 'new-secret'
```

---

### `user reset-password`

Generate random password and print it (invalidates sessions).

```bash
bedrud user reset-password --email alice@example.com
bedrud --json user reset-password --email alice@example.com
```

**JSON `data`:** includes `password` when generated.

---

### `user enable` / `user disable`

Toggle `is_active` and clear refresh token.

```bash
bedrud user disable --email alice@example.com
bedrud user enable --email alice@example.com
```

---

## Related

- [../auth-flows.md](../auth-flows.md)
- [../database-schema.md](../database-schema.md) — `users` table