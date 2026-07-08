# `bedrud invite-token`

Manage registration invite tokens (gated signup).

**Aliases:** `invite`  
**Source:** `server/internal/cli/invite.go`

---

## Subcommands

### `invite-token list`

```bash
bedrud invite-token list
bedrud invite list --page 1 --page-size 20
bedrud --json invite-token list
```

| Flag | Default | Description |
|------|---------|-------------|
| `--page` | 1 | Page number |
| `--page-size` | 50 | Tokens per page |

**JSON `data`:** `tokens[]`, `page`, `pageSize`, `total`

Each token: `id`, `token`, `email`, `createdBy`, `expiresAt`, `usedAt`

---

### `invite-token create`

```bash
bedrud invite-token create
bedrud invite create --email guest@example.com --ttl-hours 48
bedrud --json invite create --email a@ex.com
```

| Flag | Default | Description |
|------|---------|-------------|
| `--email` | "" | Bind token to email (optional) |
| `--created-by` | `cli` | Creator attribution |
| `--ttl-hours` | 168 | Lifetime (7 days) |

---

### `invite-token delete <id>`

```bash
bedrud invite-token delete <uuid>
bedrud --json invite delete <uuid>
```

---

## Related

- [../auth-flows.md](../auth-flows.md) — `tokenRegistrationOnly`
- [../database-schema.md](../database-schema.md) — `invite_tokens` table