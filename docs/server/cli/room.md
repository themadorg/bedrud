# `bedrud room`

Manage meeting rooms from the CLI. Requires DB + LiveKit API access.

**Source:** `server/internal/cli/room.go` → `internal/roomcli/`

---

## Subcommands

### `room list`

```bash
bedrud room list
bedrud room list --page 1 --page-size 20
bedrud room list --active
bedrud --json room list
```

| Flag | Default | Description |
|------|---------|-------------|
| `--page` | 1 | Page number |
| `--page-size` | 50 | Rooms per page |
| `--active` | false | Active rooms only (ignores pagination) |

**JSON `data`:** `rooms[]`, `total`, `activeOnly`, optional `page`/`pageSize`

---

### `room info <room-id-or-name>`

Show room metadata, settings, and active participant count.

```bash
bedrud room info abc-room-name
bedrud room info <uuid>
bedrud --json room info my-room
```

---

### `room close <room-id-or-name>`

Cascade delete: LiveKit room, chat uploads, DB rows. **Destructive.**

```bash
bedrud room close my-room --yes
bedrud --json room close my-room --yes
```

| Flag | Required | Description |
|------|----------|-------------|
| `--yes` | yes | Confirm destructive operation |

---

### `room suspend <room-id-or-name>`

Disconnect participants, delete LiveKit room, set `is_active=false`. DB record kept.

```bash
bedrud room suspend my-room
```

---

### `room reactivate <room-id-or-name>`

Re-enable suspended room; extends `expires_at` by 24h.

```bash
bedrud room reactivate my-room
```

---

### `room kick <room-id-or-name>`

Kick participant from LiveKit and update DB.

```bash
bedrud room kick my-room --identity <user-id-or-name>
bedrud --json room kick my-room --identity user-uuid
```

| Flag | Required | Description |
|------|----------|-------------|
| `--identity` | yes | LiveKit participant identity |

---

## Related

- [../room-lifecycle.md](../room-lifecycle.md)
- [../queue-deep-dive.md](../queue-deep-dive.md) — async admin deletes