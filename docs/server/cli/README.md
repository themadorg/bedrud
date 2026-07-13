# Bedrud CLI Reference

The `bedrud` binary is the production entrypoint (`server/cmd/bedrud/main.go`). Commands are implemented with [Cobra](https://github.com/spf13/cobra) in `server/internal/cli/`.

**Implementation:** `server/internal/cli/`  
**User/room helpers:** `internal/usercli/`, `internal/roomcli/`

---

## Global flags

| Flag | Env | Description |
|------|-----|-------------|
| `--config <path>` | `BEDRUD_CONFIG`, `CONFIG_PATH` | Config file path |
| `--json` | — | Machine-readable JSON output on stdout; errors on stderr |

See [global-flags.md](./global-flags.md) for JSON envelope format and config resolution.

**Default config paths:**

| Context | Default |
|---------|---------|
| `bedrud run` | `config.yaml` (cwd) |
| Management commands (`user`, `room`, `db`, …) | `/etc/bedrud/config.yaml` |

---

## Complete command list

### Server runtime

| Command | Doc |
|---------|-----|
| `bedrud run` (alias `server`) | [run.md](./run.md) |
| `bedrud livekit` | [livekit.md](./livekit.md) |
| `bedrud version` | [version.md](./version.md) |

### Installation

| Command | Doc |
|---------|-----|
| `bedrud install` | [install.md](./install.md) |
| `bedrud uninstall` | [uninstall.md](./uninstall.md) |

### TLS

| Command | Doc |
|---------|-----|
| `bedrud certificate regenerate` | [cert.md](./cert.md) |
| `bedrud cert renew` | [cert.md](./cert.md) (alias of certificate) |
| `bedrud cert info` | [cert.md](./cert.md) |

### Users (`bedrud user …`)

| Subcommand | Doc |
|------------|-----|
| `create` | [user.md](./user.md) |
| `delete` | [user.md](./user.md) |
| `promote` | [user.md](./user.md) |
| `demote` | [user.md](./user.md) |
| `list` | [user.md](./user.md) |
| `info` | [user.md](./user.md) |
| `password` | [user.md](./user.md) |
| `reset-password` | [user.md](./user.md) |
| `enable` | [user.md](./user.md) |
| `disable` | [user.md](./user.md) |

### Rooms (`bedrud room …`)

| Subcommand | Doc |
|------------|-----|
| `list` | [room.md](./room.md) |
| `info <id-or-name>` | [room.md](./room.md) |
| `close <id-or-name> --yes` | [room.md](./room.md) |
| `suspend <id-or-name>` | [room.md](./room.md) |
| `reactivate <id-or-name>` | [room.md](./room.md) |
| `kick <id-or-name> --identity <id>` | [room.md](./room.md) |

### Config file (`bedrud config …`)

| Subcommand | Doc |
|------------|-----|
| `path` | [config.md](./config.md) |
| `show` | [config.md](./config.md) |
| `get <key>` | [config.md](./config.md) |
| `set <key> <value>` | [config.md](./config.md) |
| `validate` | [config.md](./config.md) |

### Runtime DB settings (`bedrud settings …`)

| Subcommand | Doc |
|------------|-----|
| `show` | [settings.md](./settings.md) |
| `set <field> <value>` | [settings.md](./settings.md) |
| `reset --yes` | [settings.md](./settings.md) |

### Invite tokens (`bedrud invite-token …` / `bedrud invite …`)

| Subcommand | Doc |
|------------|-----|
| `list` | [invite-token.md](./invite-token.md) |
| `create` | [invite-token.md](./invite-token.md) |
| `delete <id>` | [invite-token.md](./invite-token.md) |

### Database (`bedrud db …`)

| Subcommand | Doc |
|------------|-----|
| `migrate` | [db.md](./db.md) |
| `status` | [db.md](./db.md) |

---

## Legacy flags (backward compatible)

Pre-Cobra invocations still work (systemd units, older docs):

| Legacy | Equivalent |
|--------|------------|
| `bedrud --run [--config path] [--skip-migrate]` | `bedrud run` |
| `bedrud --livekit [--config path]` | `bedrud livekit` |
| `bedrud --version` / `-v` | `bedrud version` |

Add `--json` anywhere in the argument list for legacy forms too.

---

## Architecture

```
cmd/bedrud/main.go
    └── cli.Execute(version)
            ├── dispatchLegacy()     # --run, --livekit, --version
            └── cobra root
                    ├── --config, --json (persistent)
                    ├── run            → server.Run()
                    ├── livekit        → livekit.RunLiveKit()
                    ├── install        → install.LinuxInstall()
                    ├── uninstall      → install.LinuxUninstall()
                    ├── version
                    ├── cert *         → utils TLS helpers
                    ├── user *         → usercli.*
                    ├── room *         → roomcli.*
                    ├── config *       → config file I/O
                    ├── settings *     → settings repo
                    ├── invite-token * → invite token repo
                    └── db *           → database migrations
```

---

## Related docs

- [../configuration.md](../configuration.md) — `config.yaml` reference
- [../cli.md](../cli.md) — short overview (links here)
- [Public CLI docs](https://bedrud.org/en/docs/getting-started/cli-reference)