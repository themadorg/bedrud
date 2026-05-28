---
name: bedrud-server
description: Full Go backend code map. Load for any server/API/auth/DB/model/handler work.
license: Apache License
---

# Bedrud Server — Full Backend Map

Go module `bedrud`. Root: `server/`. Fiber HTTP + GORM ORM + embedded LiveKit.

---

## Entrypoints

### `cmd/bedrud/main.go` — Production CLI

Dispatches by arg:

| Arg | Calls | Purpose |
|-----|-------|---------|
| `run` / `server` / `--run` | `server.Run(configPath)` | Start full app |
| `--livekit` | `livekit.RunLiveKit(configPath)` | Run LK binary standalone |
| `install` | `install.DebianInstall(...)` | Systemd install on Debian |
| `uninstall` | `install.DebianUninstall()` | Remove install |
| `user [--config <path>] promote --email <email>` | `usercli.PromoteUser()` | Add superadmin access |
| `user [--config <path>] demote --email <email>` | `usercli.DemoteUser()` | Remove superadmin access |
| `user [--config <path>] create --email <e> --password <p> --name <n>` | `usercli.CreateUser()` | Create local user |
| `user [--config <path>] delete --email <email>` | `usercli.DeleteUser()` | Delete user |

### `cmd/server/main.go` — Dev API server

Air hot-reload target. No CLI subcommands. Inits all subsystems (includes queue worker wiring, `scheduler.Initialize(db)`), registers routes, serves Swagger/Scalar docs + SPA. Swagger annotations live here.

Health: `GET /api/health`. Ready: `GET /api/ready`.

### `ui.go` — Frontend embed

`//go:embed all:frontend` → `UI embed.FS`. Populated by `make build` copying `apps/web/build/*` → `server/frontend/`.

---

## `internal/server/server.go` — Bootstrap

`Run(configPath) error` — production bootstrap. Sequence:

1. Load config (`config.Load`) — includes `QueueConfig` + `EmailConfig`
2. Init LiveKit (internal or external)
3. Init session store (`auth.InitializeSessionStore`)
4. Init DB (`database.Initialize`)
5. Run migrations (`database.RunMigrations`) — includes `models.Job{}`
6. Init scheduler (`scheduler.Initialize(db)`) — DB needed for job cleanup
7. Init auth providers (`auth.Service.Init`)
8. Init all repos
9. Init cleanup service (`cleanupSvc`)
10. Init queue worker (`queue.NewWorker(db, handlers).Start(ctx)`) — needs cleanupSvc + all repos built
11. Init authService, challengeStore, authHandler
12. Init roomHandler (receives pre-existing uploadStore internally)
13. Register Fiber routes
14. Setup TLS (self-signed / manual certs / Let's Encrypt ACME)
    - When TLS enabled: also starts HTTP listener on `server.httpPort` (default `:80`). Non-root: set `httpPort: "8080"` or use `setcap 'cap_net_bind_service=+ep'`.
15. Serve embedded SPA frontend
16. Graceful shutdown on SIGINT/SIGTERM

LK reverse-proxy at `/livekit`. CORS: dynamic origin reflection. SPA fallback: `index.html` for `/`, `shell.html` for non-API routes.

---

## `internal/handlers/` — HTTP Route Handlers

### `auth.go` — OAuth flows (goth)

`responseWriter` struct: adapter bridging Fiber `Ctx` → `http.ResponseWriter` for goth.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `BeginAuthHandler` | GET | `/auth/{provider}/login` | Start OAuth → redirect to provider |
| `CallbackHandler` | GET | `/auth/{provider}/callback` | Complete OAuth → upsert user → set JWT cookie → redirect to `/auth/callback?token=...` |

### `auth_handler.go` — Local auth + passkeys

`AuthHandler` struct: holds `authService`, `config`, `settingsRepo`, `inviteTokenRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `Register` | POST | `/auth/register` | Email/pass signup. Checks reg settings + invite tokens |
| `Login` | POST | `/auth/login` | Email/pass login. Checks `IsActive` |
| `GuestLogin` | POST | `/auth/guest` | Name-only guest. `guest-` prefixed ID |
| `RefreshToken` | POST | `/auth/refresh` | Rotate token pair. Body or `refresh_token` cookie |
| `GetMe` | GET | `/auth/me` | Return current user DB record |
| `UpdateProfile` | PUT | `/auth/profile` | Update display name (min 2 chars) |
| `ChangePassword` | PUT | `/auth/password` | Validate old, set new (min 6 chars) |
| `Logout` | POST | `/auth/logout` | Block refresh token, clear cookies |
| `PasskeyRegisterBegin` | POST | `/auth/passkey/register/begin` | WebAuthn reg start |
| `PasskeyRegisterFinish` | POST | `/auth/passkey/register/finish` | WebAuthn reg complete |
| `PasskeyLoginBegin` | POST | `/auth/passkey/login/begin` | WebAuthn login start |
| `PasskeyLoginFinish` | POST | `/auth/passkey/login/finish` | WebAuthn login complete |
| `PasskeySignupBegin` | POST | `/auth/passkey/signup/begin` | Full passkey signup start (no password) |
| `PasskeySignupFinish` | POST | `/auth/passkey/signup/finish` | Full passkey signup complete |

Helpers: `setAuthCookies` (HttpOnly, secure/sameSite/domain from config), `clearAuthCookies`, `getSession` (gorilla sessions via Fiber adapter), `getRPID`/`getOrigin` (WebAuthn RP derivation).

### `room.go` — Room lifecycle + participant moderation (biggest file)

`RoomHandler` struct: `roomRepo`, `livekitHost`, `apiKey`, `apiSecret`, `livekit.RoomService` client (via `lkutil.NewClient`), `uploadTracker *storage.ChatUploadTracker`, `storage.ChatUploadStore`, `uploadMax`, `deletionInFlight sync.Map`.

Request structs: `CreateRoomRequest{name, maxParticipants, isPublic, mode, settings}`, `JoinRoomRequest{roomName}`, `GuestJoinRoomRequest{roomName, guestName}`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `CreateRoom` | POST | `/rooms` | Create in LK + DB. Auto-gen name if empty. Strips IsPersistent (superadmin-only). 409 on conflict |
| `JoinRoom` | POST | `/rooms/join` | Lookup room, add participant, gen LK token with user metadata |
| `GuestJoinRoom` | POST | `/rooms/guest-join` | Unauth guest join for public rooms. Restricted LK token |
| `ListRooms` | GET | `/rooms` | User's created rooms |
| `DeleteRoom` | DELETE | `/rooms/:roomId` | 202 Accepted. Enqueues `room_delete` job. Creator or superadmin. `deletionInFlight` prevents double-enqueue |
| `UpdateSettings` | PATCH | `/rooms/:roomId/settings` | Partial update: isPublic, maxParticipants, settings. Preserves IsPersistent (superadmin-only) |
| `PromoteParticipant` | POST | `/rooms/:roomId/participants/:identity/promote` | Add "moderator" to LK metadata |
| `DemoteParticipant` | POST | `/rooms/:roomId/participants/:identity/demote` | Remove "moderator" from metadata |
| `KickParticipant` | DELETE | `/rooms/:roomId/participants/:identity` | Remove from LK + broadcast "kick" system msg |
| `BanParticipant` | DELETE | `/rooms/:roomId/participants/:identity/ban` | Remove from LK + kick from DB + broadcast "ban" |
| `MuteParticipant` | POST | `/rooms/:roomId/participants/:identity/mute` | Mute all audio tracks via LK |
| `DisableParticipantVideo` | POST | `/rooms/:roomId/participants/:identity/disable-video` | Mute camera track |
| `StopScreenShare` | POST | `/rooms/:roomId/participants/:identity/stop-screen-share` | Mute screen-share + screen-share-audio |
| `BlockChat` | POST | `/rooms/:roomId/participants/:identity/block-chat` | Set `chatBlocked: true` in LK metadata |
| `DeafenParticipant` | POST | `/rooms/:roomId/participants/:identity/deafen` | Send targeted "deafen" data msg |
| `UndeafenParticipant` | POST | `/rooms/:roomId/participants/:identity/undeafen` | Send targeted "undeafen" data msg |
| `AskParticipantAction` | POST | `/rooms/:roomId/participants/:identity/ask/:action` | Send "ask_unmute" or "ask_camera" |
| `SpotlightParticipant` | POST | `/rooms/:roomId/participants/:identity/spotlight` | Broadcast "spotlight" to room |
| `GetParticipantInfo` | GET | `/rooms/:roomId/participants/:identity` | Identity, name, state, tracks from LK. Self/admin/mod |
| `UploadChatImage` | POST | `/rooms/:roomId/chat/upload` | Multipart upload via ChatUploadStore |
| `AdminListRooms` | GET | `/admin/rooms` | All rooms (DB) |
| `AdminCloseRoom` | DELETE | `/admin/rooms/:roomId` | 202 Accepted. Enqueues `room_delete` job. `deletionInFlight` prevents double-enqueue |
| `AdminSuspendRoom` | POST | `/admin/rooms/:roomId/suspend` | 202 Accepted. Enqueues `room_suspend` job. `deletionInFlight` prevents double-enqueue |
| `AdminUpdateRoom` | PATCH | `/admin/rooms/:roomId` | Update maxParticipants + settings merge (superadmin). IsPersistent toggle via settings |
| `AdminGetRoomParticipants` | GET | `/admin/rooms/:roomId/participants` | Live participants + track stats from LK |
| `AdminKickParticipant` | DELETE | `/admin/rooms/:roomId/participants/:identity` | Kick (no creator check) |
| `AdminMuteParticipant` | POST | `/admin/rooms/:roomId/participants/:identity/mute` | Mute audio (admin) |
| `AdminLiveKitStats` | GET | `/admin/livekit/stats` | Aggregate: total participants, publishers, per-room |
| `BulkSuspendRooms` | POST | `/admin/rooms/bulk/suspend` | Enqueue per-room `room_suspend` jobs. Returns 202 |
| `BulkCloseRooms` | POST | `/admin/rooms/bulk/close` | Enqueue per-room `room_delete` jobs. Returns 202 |
| `GetOnlineCount` | GET | `/rooms/online-count` | Active participant count across all rooms |
| `BringToStage` | POST | `/rooms/:roomId/participants/:identity/bring-to-stage` | Stub |
| `RemoveFromStage` | POST | `/rooms/:roomId/participants/:identity/remove-from-stage` | Stub |

Internal helpers: `withAuth` (inject LK token via `lkutil.AuthContext`), `resolveRoom` (load by ID + derive adminId), `sendSystemMessage` (via `lkutil.SendSystemMessage`), `sendTargetedSystemMessage`, `generateShortID`, `containsAccess`, `boolPtr`.  
Chat upload tracking: `UploadChatImage` calls `uploadTracker.Record()` only for disk-backed uploads (skips S3/inline). Room close/delete cleanup runs in queue handlers (`handler_room_delete.go` calls `cleanupSvc.CascadeDeleteRoom` which includes tracker cleanup).

### `users.go` — Admin user management

`UsersHandler` struct: `userRepo`, `roomRepo`, `passkeyRepo`, `prefsRepo`, `uploadTracker *storage.ChatUploadTracker`, `client livekit.RoomService` (via lkutil), `apiKey`, `apiSecret`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `ListUsers` | GET | `/admin/users` | All users with computed `IsAdmin`. Superadmin only |
| `UpdateUserAccesses` | PUT | `/admin/users/:id/accesses` | Replace entire Accesses slice |
| `UpdateUserStatus` | PUT | `/admin/users/:id/status` | Set `IsActive` |
| `GetUserDetail` | GET | `/admin/users/:id` | User details + their rooms |
| `DeleteUser` | DELETE | `/admin/users/:id` | 202 Accepted. Enqueues `user_delete` job. Self-deletion guard (400) |
| `BulkDeleteUsers` | POST | `/admin/users/bulk/delete` | 202 Accepted. Enqueues per-user `user_delete` jobs |

DTOs: `UserListResponse`, `UserDetails{ID, Email, Name, Provider, IsActive, IsAdmin, Accesses, CreatedAt}`, `UserStatusUpdateRequest{active}`, `UserStatusUpdateResponse{message}`.  
Constructor: `NewUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, uploadTracker, lkCfg)` — creates LiveKit client via `lkutil.NewClient`.

### `admin_handler.go` — System settings + invite tokens

`AdminHandler` struct: `settingsRepo`, `inviteTokenRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetSettings` | GET | `/admin/settings` | Full system settings |
| `GetPublicSettings` | GET | `/settings` | Unauth. `registrationEnabled`, `tokenRegistrationOnly` only |
| `UpdateSettings` | PUT | `/admin/settings` | Replace entire settings |
| `ListInviteTokens` | GET | `/admin/invite-tokens` | All tokens with computed `used` bool |
| `CreateInviteToken` | POST | `/admin/invite-tokens` | Crypto-random hex token, tied to email, configurable expiry (default 72h) |
| `DeleteInviteToken` | DELETE | `/admin/invite-tokens/:id` | Delete by ID |

### `preferences_handler.go` — Per-user JSON preferences

`PreferencesHandler` struct: `prefsRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `GetPreferences` | GET | `/api/auth/preferences` | User's `preferencesJson` blob, `"{}"` if none |
| `UpdatePreferences` | PUT | `/api/auth/preferences` | Validate JSON + ≤4KB, upsert |

### `models.go` — Shared response DTOs

`ErrorResponse{Error string}`, `AuthResponse{User UserResponse, Token string}`, `UserResponse{ID, Email, Name, Provider, AvatarURL}`.

---

## `internal/models/` — GORM Models

### `user.go`

```
User {
  ID         string     // varchar36 PK
  Email      string     // unique
  Name       string
  Provider   string     // OAuth provider ("google", "github", "twitter", "local")
  AvatarURL  string
  Password   string     // json:"-", bcrypt hash
  RefreshToken string   // json:"-"
  Accesses   StringArray // []string, PG text[] via "{val1,val2}" format
  IsActive   bool       // default true
  CreatedAt  time.Time
  UpdatedAt  time.Time
}
```

`AccessLevel` enum: `superadmin`, `admin`, `moderator`, `user`, `guest`.
`StringArray` custom type: `sql.Scanner` + `driver.Valuer` + `GormDataTypeInterface`.
Methods: `HasAccess(level)`, `IsAdmin()` (checks `admin` in Accesses).

### `room.go`

```
Room {
  ID              string
  Name            string     // unique, URL-safe slug
  CreatedBy       string
  IsActive        bool
  MaxParticipants int        // default 20
  AdminID         string
  IsPublic        bool
  Settings        RoomSettings // embedded, prefix `settings_`
  Mode            string
  ExpiresAt       time.Time
  CreatedAt       time.Time
  UpdatedAt       time.Time
}

RoomSettings {
  AllowChat       bool   // default true
  AllowVideo      bool   // default true
  AllowAudio      bool   // default true
  RequireApproval bool   // default false
  E2EE            bool   // default false
  IsPersistent    bool   // default false, skips idle cleanup
}

RoomParticipant {
  ID          string
  RoomID      string     // composite unique with UserID: idx_room_user
  UserID      string
  JoinedAt    time.Time
  LeftAt      *time.Time
  IsActive    bool
  IsApproved  bool
  IsMuted     bool
  IsVideoOff  bool
  IsChatBlocked bool
  IsBanned    bool
  IsOnStage   bool
  // GORM belongs-to: User, Room
}

RoomPermissions {
  ID             string
  RoomID         string   // FK ref to RoomParticipant(RoomID,UserID)
  UserID         string
  IsAdmin        bool
  CanKick        bool
  CanMuteAudio   bool
  CanDisableVideo bool
  CanChat        bool
}
```

`ValidateRoomName(name)` — 3-63 chars, lowercase alphanumeric + hyphens, no consecutive/leading/trailing hyphens.
`GenerateRandomRoomName()` — crypto-random `xxx-xxxx-xxx`.

Sentinel errors: `ErrRoomNameInvalid`, `ErrRoomNameTooShort`, `ErrRoomNameTooLong`, `ErrRoomNameTaken`.

### `passkey.go`

```
Passkey {
  ID           string
  UserID       string     // indexed
  CredentialID []byte     // bytea
  PublicKey    []byte     // bytea
  Algorithm    int
  Counter      uint32     // replay protection
  Name         string
  CreatedAt    time.Time
}
```

### `refresh.go`

```
BlockedRefreshToken {
  ID        string
  Token     string     // unique
  UserID    string     // indexed
  ExpiresAt time.Time  // indexed, for cleanup
  CreatedAt time.Time
}
```

### `settings.go`

```
SystemSettings {
  ID                   uint   // auto PK, always 1 (singleton)
  RegistrationEnabled  bool   // default true
  TokenRegistrationOnly bool  // default false
  UpdatedAt            time.Time
}
```

### `invite_token.go`

```
InviteToken {
  ID        string
  Token     string     // unique, varchar64
  Email     string     // optional, pre-bind
  CreatedBy string
  ExpiresAt time.Time
  UsedAt    *time.Time
  UsedBy    string
  CreatedAt time.Time
}
```

### `user_preferences.go`

```
UserPreferences {
  UserID         string     // PK
  PreferencesJSON string   // text, default '{}'
  UpdatedAt      time.Time
}
```

### `chat_upload.go`

```
ChatUpload {
  ID        string     // PK
  RoomID    string     // FK → rooms.id, ON DELETE CASCADE (Postgres)
  FileHash  string     // SHA-256 hex of file content
  Extension string     // file extension with dot (e.g. ".png")
  CreatedAt time.Time
}
```

Only disk-backend uploads (`./data/uploads/chat/`) are tracked. S3 and inline uploads are skipped.

### Model relationships

```
User(1) → (N)Passkey              via UserID
User(1) → (N)BlockedRefreshToken  via UserID
User(1) → (1)UserPreferences      via UserID (PK)
User(1) → (N)RoomParticipant      via UserID (FK)
User(1) → (N)RoomPermissions      via UserID (FK)
Room(1) → (N)RoomParticipant      via RoomID (FK)
Room(1) → (N)RoomPermissions      via RoomID (FK)
Room(1) → (N)ChatUpload           via RoomID (FK)
RoomParticipant(1) ↔ (1)RoomPermissions  via (RoomID, UserID)
```

---

## `internal/repository/` — Data Access

### `user_repository.go` — `UserRepository{*gorm.DB}`

| Fn | Purpose |
|----|---------|
| `NewUserRepository(db)` | Constructor |
| `CreateOrUpdateUser(user)` | Upsert by `(email, provider)` — FirstOrCreate + Assign |
| `GetUserByEmailAndProvider(email, provider)` | Composite lookup. `nil, nil` if not found |
| `GetUserByEmail(email)` | Lookup by email |
| `GetUserByID(id)` | PK lookup |
| `CreateUser(user)` | Straight insert |
| `UpdateUser(user)` | Full save with timestamp |
| `UpdateRefreshToken(userID, token)` | Update refresh token field |
| `BlockRefreshToken(userID, token, expiresAt)` | Insert into `blocked_refresh_tokens` |
| `IsRefreshTokenBlocked(token)` | Check revocation (not expired) |
| `CleanupBlockedTokens()` | Delete expired blocked tokens |
| `UpdateUserAccesses(userID, accesses)` | Replace role array |
| `GetUsersByAccess(access)` | Find by role. PG `ANY()` for text[] |
| `GetAllUsers()` | Return all users |
| `DeleteUser(userID)` | Transactional cascade: passkeys → preferences → participants → permissions → blocked tokens → user |

### `room_repository.go` — `RoomRepository{*gorm.DB}`

| Fn | Purpose |
|----|---------|
| `NewRoomRepository(db)` | Constructor |
| `CreateRoom(createdBy, name, isPublic, mode, settings)` | TX: validate/gen name → create room → add creator as approved participant + admin perms. 24h expiry |
| `GetRoom(id)` / `GetRoomByName(name)` | Lookup by ID or name (case-insensitive) |
| `AddParticipant(roomID, userID)` | Insert or reactivate. Reject banned |
| `RemoveParticipant(roomID, userID)` | Mark inactive, set left_at |
| `GetActiveParticipants(roomID)` | Currently active participants |
| `GetRoomParticipantsWithUsers(roomID)` | Same + Preload("User") |
| `KickParticipant(roomID, userID)` | Mark inactive + banned |
| `BringToStage(roomID, userID)` / `RemoveFromStage(roomID, userID)` | Toggle is_on_stage |
| `IsParticipantOnStage(roomID, userID)` | Boolean check |
| `UpdateParticipantPermissions(roomID, userID, perms)` | Write permission row |
| `GetParticipantPermissions(roomID, userID)` | Read permission row |
| `UpdateParticipantStatus(roomID, userID, updates)` | Generic map-based update |
| `UpdateRoomSettings(roomID, settings)` | Atomic map-based update of embedded settings (all 6 fields). Merge-safe — only sent columns updated |
| `UpdateRoom(room)` | Full save |
| `DeleteRoom(roomID, userID)` | TX cascade. Checks created_by |
| `AdminDeleteRoom(roomID)` | Same, no owner check. Also deletes `chat_uploads` rows inside the transaction |
| `GetAllRooms()` / `GetAllActiveRooms()` | List rooms |
| `GetRoomsCreatedByUser(userID)` | User's created rooms |
| `GetRoomsParticipatedInByUser(userID)` | Rooms user joined |
| `SetRoomIdle(roomID)` | Mark inactive |
| `CleanupExpiredRooms()` | Bulk mark expired inactive. Excludes persistent rooms (`settings_is_persistent = false`) |
| `GetUserByID(userID)` | Fetch user (for participant lookups) |
| `CountActiveParticipants()` | Distinct count across all rooms |

### `passkey_repository.go` — `PasskeyRepository{*gorm.DB}`

`CreatePasskey`, `GetPasskeyByCredentialID`, `GetPasskeysByUserID`, `UpdatePasskeyCounter`, `DeletePasskey`, `DeleteByUserID(userID)`.

### `settings_repository.go` — `SettingsRepository{*gorm.DB}`

`GetSettings()` — FirstOrCreate ID=1, default RegistrationEnabled=true.
`SaveSettings(s)` — Force ID=1, upsert.

### `invite_token_repository.go` — `InviteTokenRepository{*gorm.DB}`

`Create(t)`, `List()` (newest first), `GetByToken(token)`, `MarkUsed(tokenID, userID)`, `Delete(tokenID)`.

### `user_preferences_repository.go` — `UserPreferencesRepository{*gorm.DB}`

`GetByUserID(userID)` — `nil, nil` if not found.
`Upsert(userID, prefsJSON)` — `ON CONFLICT ... UPDATE ALL`.
`DeleteByUserID(userID)` — Delete preferences row.

---

## `internal/auth/` — Auth Service

### `auth.go` — `AuthService{userRepo, passkeyRepo}`

| Fn | Purpose |
|----|---------|
| `NewAuthService(userRepo, passkeyRepo)` | Constructor |
| `Register(email, password, name)` | Create local user, bcrypt hash |
| `Login(email, password)` | Validate credentials → JWT pair + user |
| `GuestLogin(name)` | Transient guest user + tokens |
| `UpdateRefreshToken(userID, token)` | Store new refresh |
| `GetUserByID(userID)` / `GetUserByEmail(email)` | User lookups |
| `UpdateProfile(userID, name)` | Display name |
| `ChangePassword(userID, current, new)` | Verify old → hash new |
| `Logout(userID, refreshToken)` | Block refresh token |
| `ValidateRefreshToken(refreshToken)` | Check blocklist → validate JWT |
| `UpdateUserAccesses(userID, accesses)` | Modify roles |
| `BeginRegisterPasskey(userID)` | WebAuthn reg start |
| `FinishRegisterPasskey(...)` | WebAuthn reg complete |
| `FinishSignupPasskey(...)` | Full passkey signup: create user + passkey + tokens |
| `BeginLoginPasskey()` | WebAuthn login start |
| `FinishLoginPasskey(...)` | WebAuthn login/assertion |
| `Init(cfg)` | Register OAuth providers (Google/GitHub/Twitter) via Goth |

DTOs: `ErrorResponse`, `RegisterRequest`, `LoginRequest`, `GuestLoginRequest`, `TokenResponse`, `TokenPair`, `LoginResponse`, `LogoutRequest`.

### `jwt.go` — Token generation + validation

`GenerateToken(userID, email, name, provider, accesses, cfg)` — Access token, expiry from `cfg.Auth.TokenDuration`.
`ValidateToken(tokenString, cfg)` — Parse HMAC-SHA256 JWT → `*Claims`.
`GenerateTokenPair(userID, email, name, accesses, cfg)` — Access + 7-day refresh.

`Claims` struct: `UserID`, `Email`, `Name`, `Provider`, `Accesses []string` + `jwt.RegisteredClaims`.

### `session_store.go`

`InitializeSessionStore(secret, secure)` — Create gorilla CookieStore for Goth. Set HttpOnly/Secure/SameSite from TLS mode.
`SetProviderToSession(c *fiber.Ctx, provider)` — Bridge Fiber → http.Request for Goth session.

---

## `internal/middleware/auth.go`

`Protected() fiber.Handler` — Extract JWT from `Authorization: Bearer` (fallback: `access_token` cookie). Validate. Store `*auth.Claims` in `c.Locals("user")`.

`RequireAccess(requiredAccess models.AccessLevel) fiber.Handler` — Check user Accesses contains role. 403 if not.

---

## `internal/database/` — DB Layer

### `database.go`

`Initialize(cfg)` — GORM connection. PostgreSQL (connection pooling) or SQLite.
`GetDB()` — Singleton `*gorm.DB`.
`Close()` — Close underlying `*sql.DB`.

### `migrations.go`

`RunMigrations()` — AutoMigrate: User, BlockedRefreshToken, Room, RoomParticipant, RoomPermissions, Passkey, SystemSettings, InviteToken, UserPreferences, ChatUpload.  
Raw SQL FK constraints: `fk_room_permissions_participant`, `fk_chat_uploads_room` (Postgres: `ON DELETE CASCADE`).

---

## `internal/livekit/` — Embedded Media Server

### `embed.go`

`Bin embed.FS` — contains `bin/livekit-server`. Build-tagged `!windows`.

### `config.go`

`ConfigYAML` — shared LiveKit YAML config struct. Used by both the installer (`internal/install/linux.go`) and the embedded server startup. Uses `omitempty` on zero-value fields (`bind_addresses`, `tcp_port`, `udp_port`, `port_range_start/end`, `turn.enabled/domain/udp_port/tls_port/cert_file/key_file`, `logging.json/level`) so they are omitted from marshaled YAML and LiveKit uses its defaults. Fields: `Port`, `BindAddresses`, `Keys`, `RTC` (tcp/udp ports, port range, node_ip), `TURN` (enabled, domain, udp/tls ports, cert/key), `Logging`.

### `server.go`

`ExportBinary(destPath)` — Write embedded binary with 0755. Remove existing first (avoid ETXTBSY).
`RunLiveKit(configPath)` — Run synchronously.
`ResolveNodeIP(explicitIP, serverHost)` — Resolve LiveKit node IP: use explicit `nodeIP` if set, else parse `server.host` if valid non-loopback IP, else detect outbound IP via UDP dial. Returns "" if all fail.
`generateTempConfig(apiKey, apiSecret, port, nodeIP, certFile, keyFile, serverHost)` — Generate temp YAML with TURN/TLS for embedded mode. When TLS enabled: sets `TURN.Domain = serverHost`, `TURN.UDPPort = 3478`, `TURN.TLSPort = 5349`. When `nodeIP` set: `UseExternalIP = false`, `NodeIP = nodeIP`. Returns temp file path.
`StartInternalServer(ctx, apiKey, apiSecret, port, cert, key, externalConfig, nodeIP, serverHost)` — Background goroutine, 3s startup sleep. Skip if `LIVEKIT_MANAGED=true`. When cert/key provided and no external config, generates temp LiveKit YAML with TURN/TLS (port 5349) using server's certificate. Falls back to inline `--port`/`--keys` args if no TLS. Resolves relative cert/key paths at the caller level (server.go).

---

## `internal/queue/` — Job Queue

`server/internal/queue/` — Internal job queue for async task processing. Worker polls `jobs` table, dispatches to registered handlers. Two DB backends with different claim algorithms (PostgreSQL: `SKIP LOCKED`; SQLite: two-step with serialized writes via `SetMaxOpenConns(1)`).

### Files

| File | Purpose |
|------|---------|
| `job.go` | 7 payload structs: `UserDeletePayload`, `RoomDeletePayload`, `RoomSuspendPayload`, `ChatUploadS3Payload`, `SendEmailPayload`, `WebhookPayload`, `ProcessRecordingPayload` |
| `queue.go` | `Enqueue(ctx, db, jobType, payload, opts...)` — inserts job row with priority/retry opts. `Handler` type: `func(ctx, db, job) error`. `Worker` struct with `Start(ctx)`/`Stop()` |
| `worker.go` | Claim loop: poll every 500ms, claim pending jobs via DB-specific algorithm, dispatch to handler, retry with exponential backoff (`2^attempts * 5s`, capped 1h), mark done/failed |
| `handler_user_delete.go` | Fetch user's rooms → `cleanupSvc.DeleteUserRooms` → delete passkeys → delete prefs → delete user |
| `handler_room_delete.go` | Fetch room → `cleanupSvc.CascadeDeleteRoom` |
| `handler_room_suspend.go` | Fetch room → `cleanupSvc.SuspendRoom` |
| `handler_chat_upload.go` | Decode base64 payload → `uploadStore.Store` → `uploadTracker.Record` |
| `handler_stubs.go` | 3 stubs (email, webhook, recording) — log "not implemented — stub". Ready for future implementation |

### Worker Options

`WorkerOptions{Interval: 500ms, Concurrency: 1}` (configurable via `QueueConfig`). Worker drains all available jobs per tick (inner loop until nil), not one-per-tick. `Start(ctx)` runs in background goroutine, `Stop()` signals via channel.

### Retry & Backoff

On failure: if `attempts >= maxAttempts` → status = `failed`. Else → `run_at = now + (2^attempts * 5s)`, status stays `pending`. Default `MaxAttempts=3`: 3 total attempts (1 original + 2 retries). Backoff sequence: 10s, 20s, 40s.

### Cleanup

`scheduler.Initialize` sets up daily 03:00 cleanup: `CleanupJobs(db, 7d)` for done jobs, `CleanupFailedJobs(db, 30d)` for failed jobs.

### Payloads

| Type | Struct | Priority |
|------|--------|----------|
| `user_delete` | `UserDeletePayload{UserID, Email, RoomIDs}` | 1 (HIGH) |
| `room_delete` | `RoomDeletePayload{RoomID, SystemEvent, SystemMessage, DeletedIdentity}` | 1 (HIGH) |
| `room_suspend` | `RoomSuspendPayload{RoomID}` | 2 (MEDIUM) |
| `chat_upload_s3` | `ChatUploadS3Payload{Data(base64), RoomID, MimeType, UserID}` | 0 (DEFAULT) |
| `send_email` | `SendEmailPayload{To, Subject, TemplateName, TemplateData}` | stub |
| `dispatch_webhook` | `WebhookPayload{URL, Event, Body, Secret, MaxRetries}` | stub |
| `process_recording` | `ProcessRecordingPayload{RoomID, RoomName, EgressID, FileURL, ...}` | stub |

### Config Additions

Added to `config/config.go`:

```go
type QueueConfig struct {
    PollInterval ConfigInt `yaml:"pollInterval"` // ms, default 500. Env: QUEUE_POLL_INTERVAL
    MaxAttempts  int       `yaml:"maxAttempts"`  // default 3. Env: QUEUE_MAX_ATTEMPTS
    Concurrency  int       `yaml:"concurrency"`  // default 1. Env: QUEUE_CONCURRENCY
}

type EmailConfig struct {
    SMTPHost    string `yaml:"smtpHost"`    // Env: EMAIL_SMTP_HOST
    SMTPPort    int    `yaml:"smtpPort"`    // Env: EMAIL_SMTP_PORT
    Username    string `yaml:"username"`    // Env: EMAIL_USERNAME
    Password    string `yaml:"password"`    // Env: EMAIL_PASSWORD
    FromAddress string `yaml:"fromAddress"` // Env: EMAIL_FROM_ADDRESS
    FromName    string `yaml:"fromName"`    // Env: EMAIL_FROM_NAME
}
```

---

## `internal/services/room_cleanup.go` — RoomCleanupService

Cross-cutting service for cascading room/user cleanup. Used by queue handlers and user CLI.

`RoomCleanupService` struct: `roomRepo`, `livekit.RoomService` client, `apiKey`, `apiSecret`, `uploadTracker`.

| Fn | Purpose |
|----|---------|
| `NewRoomCleanupService(roomRepo, lkClient, apiKey, apiSecret, uploadTracker)` | Constructor |
| `CascadeDeleteRoom(ctx, room, reason, deletedIdentity)` | Close LK room → broadcast end msg → AdminDeleteRoom → chat upload tracker cleanup |
| `SuspendRoom(ctx, room)` | Close LK room → mark room inactive |
| `DeleteUserRooms(ctx, user, rooms)` | Iterate rooms, CascadeDeleteRoom each |

---

## `internal/testutil/db.go`

Test utilities for database-dependent unit tests.

| Export | Purpose |
|--------|---------|
| `SetupTestDB()` | Create in-memory SQLite DB, run migrations, return `*gorm.DB` and cleanup fn |
| `TeardownTestDB(db)` | Close and clean up test DB |

---

## `internal/scheduler/scheduler.go`

gocron scheduler. Every 1 min: `checkIdleRooms`.
`checkIdleRooms(roomRepo, cfg, client)` — List active DB rooms → query LK participant counts → mark idle if 0 participants + >5min old. Skips rooms with `IsPersistent = true`. Logs error on failure, info on success.
`Initialize(roomRepo, lkCfg, db *gorm.DB)` — Setup + start async. DB param for daily 03:00 job cleanup (deletes done jobs >7d, failed jobs >30d).
`Stop()` — Graceful stop.

---

## `internal/storage/chat_upload.go`

`ChatUploadStore` interface: `Store(data []byte) (*ChatAttachment, error)`.

Backends: `disk`, `inline` (base64), `hybrid`, `s3` (raw AWS SigV4, no SDK).

`ChatAttachment` struct: `URL`, `MIME`, `Size`, `Width`, `Height`.
`NewChatUploadStore(cfg)` — Factory by `cfg.Backend`.

Validation: MIME must be png/jpeg/gif/webp. SHA256 content hash filename.

---

## `internal/utils/tls.go`

- `const CertWarnDays = 30` — days before expiry to warn and auto-renew.
- `const SelfSignedCertDays = 1825` — self-signed cert validity (~5 years).
- `KeyAlgorithm` — string enum: `KeyEd25519` (default), `KeyECDSA256`, `KeyRSA2048`, `KeyRSA4096`.
- `GenerateSelfSignedCert(certFile, keyFile, hosts...)` — wrapper, generates **Ed25519** (~128-bit security, deterministic, 32B pub key), PKCS8-encoded (RFC5958, generic for any algo). KeyUsage: DigitalSignature only. Default SANs: localhost + 127.0.0.1 + ::1, CN=localhost, Org="Bedrud Open Source". Errors clean up partial files.
- `GenerateSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...)` — explicit algo variant.
- `RenewSelfSignedCert(certFile, keyFile, hosts...)` — calls `detectCertAlgorithm()` to read existing cert's public key type, preserves it. Overwrites atomically via `.new` temp files + `os.Rename`. SANs from `hosts` parameter (not old cert).
- `RenewSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...)` — explicit algo override.
- `detectCertAlgorithm(certFile)` — PEM-decode + parse x509 → maps `PublicKeyAlgorithm` (Ed25519/ECDSA/RSA) to `KeyAlgorithm`. ECDSA → P256, RSA → 2048 (safest subset of supported types).
- `keyUsageForAlgo(algo)` — RSA → `DigitalSignature | KeyEncipherment` (RFC 3279). Ed25519/ECDSA → `DigitalSignature` only.
- `generateKey(algo)` — dispatch: `ed25519.GenerateKey`, `ecdsa.GenerateKey(elliptic.P256)`, `rsa.GenerateKey(2048/4096)`. All return `crypto.Signer`.
- `ValidateTLSCertPair(certFile, keyFile)` — reads, decodes PEM, parses x509, checks expiry/validity range, verifies key match. Returns `(*CertInfo, error)`.
- `CertInfo` — struct: Subject, Issuer, NotBefore, NotAfter, DaysRemaining, SANs, Status.

---

## `internal/install/debian.go`

`DebianInstall(...)` — Interactive Debian installer:
- Copy binary → `/usr/local/bin/bedrud`
- Write `/etc/bedrud/config.yaml` + `/etc/bedrud/livekit.yaml`
- Create `bedrud.service` + `livekit.service` systemd units
- Support: external LK, separate LK domain, reverse-proxy mode, ACME, self-signed certs

`DebianUninstall()` — Stop services, remove units, binaries, configs, data dirs.

---

## `internal/lkutil/lkutil.go` — Shared LiveKit Helpers

Cross-cutting package used by `handlers/users.go`, `handlers/room.go`, and `usercli/usercli.go`.

| Export | Signature | Purpose |
|--------|-----------|---------|
| `NewClient(lkCfg)` | `func(*config.LiveKitConfig) livekit.RoomService` | Create LiveKit RoomService protobuf client. Respects `InternalHost` / `Host`, handles `SkipTLSVerify` |
| `AuthContext(ctx, apiKey, apiSecret, grants...)` | `func(context.Context, string, string, ...*lkauth.VideoGrant) context.Context` | Inject Bearer token into twirp context |
| `SendSystemMessage(ctx, client, roomName, event, message)` | `func(context.Context, livekit.RoomService, string, string, string)` | Send typed system data message over LiveKit data channel (topic `"system"`, kind `RELIABLE`) |

---

## `internal/usercli/usercli.go`

`PromoteUser(configPath, email)` — Add `superadmin` to Accesses.
`DemoteUser(configPath, email)` — Remove `superadmin`.
`CreateUser(configPath, email, password, name)` — bcrypt hash, insert.
`DeleteUser(configPath, email)` — Full cleanup: loads config, inits DB + migrations, fetches user + their created rooms. For each room: sends "room deleted" system message via `lkutil`, deletes from LiveKit, hard-deletes from DB via `AdminDeleteRoom`, cleans up chat upload files via `ChatUploadTracker.DeleteByRoom`. Then deletes passkeys (`PasskeyRepository.DeleteByUserID`), preferences (`UserPreferencesRepository.DeleteByUserID`), and user (`UserRepository.DeleteUser`). Aborts if any DB room deletion fails.
`withUser(configPath, email, fn)` — Load config + init DB + lookup user → call fn.

---

## Dependency graph

```
cmd/bedrud → server → handlers → repository → models
                   ↘            → auth → repository
                   ↘            → lkutil (shared LiveKit client + auth + system messages)
                   ↘ middleware → auth
                   ↘ scheduler → repository + livekit + database (job cleanup)
                   ↘ services → repository + lkutil
                   ↘ queue → database + models + services (via handler deps)
                   ↘ storage (standalone; ChatUploadTracker depends on db + models)
                   ↘ utils (standalone)
                   ↘ testutil (standalone; depends on database)
                   ↘ install → utils
                   ↘ usercli → repository + lkutil + services
                   ↘ database → models (via migrations)
cmd/server → (same wiring, direct in main.go)
                   ↘ handlers/users → lkutil + storage (ChatUploadTracker) + queue (Enqueue)
                   ↘ handlers/room → lkutil + storage (ChatUploadTracker) + queue (Enqueue)
                   ↘ handlers/admin_queue → database + models + queue
```
