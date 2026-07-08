# CLI Helpers (`usercli` + `roomcli`)

Low-level CLI implementation packages called by `internal/cli/` Cobra commands. Both load config, init DB + migrations, and perform operations directly (not via HTTP).

---

## `internal/usercli/`

### `withUser(configPath, email, fn)`

Shared bootstrap: load config → init DB → migrations → lookup user by email → call `fn`.

### Functions

| Function | Description |
|----------|-------------|
| `PromoteUser(configPath, email, role)` | Set role accesses (default role: `superadmin`) |
| `DemoteUser(configPath, email, role)` | Remove role from accesses; ensures `user` remains |
| `CreateUser(configPath, email, password, name)` | bcrypt hash + insert local user |
| `DeleteUser(configPath, email)` | Full cascade: LK rooms → DB → passkeys → prefs → user |
| `ListUsers(configPath)` | Tabular user list |
| `GetUserInfo(configPath, email)` | User details |
| `SetPassword(configPath, email, password)` | Direct password set |
| `ResetPassword(configPath, email)` | Generate reset token + enqueue email |
| `EnableUser` / `DisableUser` | Toggle `IsActive`; updates in-memory ban set |

### Delete user flow

1. Load user's created rooms
2. For each room: send "room deleted" system message via `lkutil`
3. Close/delete from LiveKit
4. `AdminDeleteRoom` in DB + `ChatUploadTracker.DeleteByRoom`
5. Delete passkeys, preferences, user record
6. Aborts if any DB room deletion fails

---

## `internal/roomcli/`

### `withRepo(configPath, fn)`

Shared bootstrap: load config → init DB → migrations → call `fn` with repos + LK client.

### Functions

| Function | Description |
|----------|-------------|
| `ListRooms(configPath, page, pageSize, activeOnly)` | Paginated or active-only room table |
| `GetRoomInfo(configPath, roomID)` | Room details + participant count from LK |
| `CloseRoom(configPath, roomID)` | `RoomCleanupService.CascadeDeleteRoom` |
| `SuspendRoom(configPath, roomID)` | `RoomCleanupService.SuspendRoom` |
| `ReactivateRoom(configPath, roomID)` | Mark room active in DB |
| `KickParticipant(configPath, roomID, identity)` | Remove participant from LiveKit |

### Output format

CLI commands print human-readable tables to stdout with `✓` / error messages to stderr.