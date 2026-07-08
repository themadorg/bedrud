# CLI Overview

The `bedrud` binary exposes a Cobra CLI for running the server, installation, and day-2 operations.

**Full reference (one doc per command):** [cli/README.md](./cli/README.md)

---

## Quick index

| Command | Document |
|---------|----------|
| Global flags (`--config`, `--json`) | [cli/global-flags.md](./cli/global-flags.md) |
| `run` | [cli/run.md](./cli/run.md) |
| `livekit` | [cli/livekit.md](./cli/livekit.md) |
| `version` | [cli/version.md](./cli/version.md) |
| `install` | [cli/install.md](./cli/install.md) |
| `uninstall` | [cli/uninstall.md](./cli/uninstall.md) |
| `cert` | [cli/cert.md](./cli/cert.md) |
| `user` | [cli/user.md](./cli/user.md) |
| `room` | [cli/room.md](./cli/room.md) |
| `config` | [cli/config.md](./cli/config.md) |
| `settings` | [cli/settings.md](./cli/settings.md) |
| `invite-token` / `invite` | [cli/invite-token.md](./cli/invite-token.md) |
| `db` | [cli/db.md](./cli/db.md) |

**Public docs:** [bedrud.org CLI reference](https://bedrud.org/en/docs/getting-started/cli-reference)

**Implementation:** `server/internal/cli/`