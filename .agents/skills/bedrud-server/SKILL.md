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

Air hot-reload target. No CLI subcommands. Inits all subsystems, registers routes, serves Swagger/Scalar docs + SPA. Swagger annotations live here.

Health: `GET /api/health`. Ready: `GET /api/ready`.

### `ui.go` — Frontend embed

`//go:embed all:frontend` → `UI embed.FS`. Populated by `make build` copying `apps/web/build/*` → `server/frontend/`.

---

## `internal/server/server.go` — Bootstrap

`Run(configPath) error` — production bootstrap. Sequence:

1. Load config (`config.Load`)
2. Init LiveKit (internal or external)
3. Init session store (`auth.InitializeSessionStore`)
4. Init DB (`database.Initialize`)
5. Run migrations (`database.RunMigrations`)
6. Init scheduler (`scheduler.Initialize`)
7. Init auth providers (`auth.Service.Init`)
8. Init all repos + handlers
9. Register Fiber routes
10. Setup TLS (self-signed / manual certs / Let's Encrypt ACME)
    - When TLS enabled: also starts HTTP listener on `server.httpPort` (default `:80`). Non-root: set `httpPort: "8080"` or use `setcap 'cap_net_bind_service=+ep'`.
11. Serve embedded SPA frontend
12. Graceful shutdown on SIGINT/SIGTERM

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

`RoomHandler` struct: `roomRepo`, `livekitHost`, `apiKey`, `apiSecret`, `livekit.RoomService` client, `storage.ChatUploadStore`, `uploadMax`.

Request structs: `CreateRoomRequest{name, maxParticipants, isPublic, mode, settings}`, `JoinRoomRequest{roomName}`, `GuestJoinRoomRequest{roomName, guestName}`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `CreateRoom` | POST | `/rooms` | Create in LK + DB. Auto-gen name if empty. Strips IsPersistent (superadmin-only). 409 on conflict |
| `JoinRoom` | POST | `/rooms/join` | Lookup room, add participant, gen LK token with user metadata |
| `GuestJoinRoom` | POST | `/rooms/guest-join` | Unauth guest join for public rooms. Restricted LK token |
| `ListRooms` | GET | `/rooms` | User's created rooms |
| `DeleteRoom` | DELETE | `/rooms/:roomId` | Delete from LK + DB. Creator or superadmin |
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
| `AdminCloseRoom` | POST | `/admin/rooms/:roomId/close` | Delete from LK, set `IsActive = false` |
| `AdminUpdateRoom` | PATCH | `/admin/rooms/:roomId` | Update maxParticipants + settings merge (superadmin). IsPersistent toggle via settings |
| `AdminGetRoomParticipants` | GET | `/admin/rooms/:roomId/participants` | Live participants + track stats from LK |
| `AdminKickParticipant` | DELETE | `/admin/rooms/:roomId/participants/:identity` | Kick (no creator check) |
| `AdminMuteParticipant` | POST | `/admin/rooms/:roomId/participants/:identity/mute` | Mute audio (admin) |
| `AdminLiveKitStats` | GET | `/admin/livekit/stats` | Aggregate: total participants, publishers, per-room |
| `GetOnlineCount` | GET | `/rooms/online-count` | Active participant count across all rooms |
| `BringToStage` | POST | `/rooms/:roomId/participants/:identity/bring-to-stage` | Stub |
| `RemoveFromStage` | POST | `/rooms/:roomId/participants/:identity/remove-from-stage` | Stub |

Internal helpers: `withAuth` (inject LK token), `resolveRoom` (load by ID + derive adminId), `sendSystemMessage`, `sendTargetedSystemMessage`, `generateShortID`, `containsAccess`, `boolPtr`.

### `users.go` — Admin user management

`UsersHandler` struct: `userRepo`, `roomRepo`.

| Fn | Method | Route | Purpose |
|----|--------|-------|---------|
| `ListUsers` | GET | `/admin/users` | All users with computed `IsAdmin`. Superadmin only |
| `UpdateUserAccesses` | PUT | `/admin/users/:id/accesses` | Replace entire Accesses slice |
| `UpdateUserStatus` | PUT | `/admin/users/:id/status` | Set `IsActive` |
| `GetUserDetail` | GET | `/admin/users/:id` | User details + their rooms |

DTOs: `UserListResponse`, `UserDetails{ID, Email, Name, Provider, IsActive, IsAdmin, Accesses, CreatedAt}`, `UserStatusUpdateRequest{active}`, `UserStatusUpdateResponse{message}`.

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

### Model relationships

```
User(1) → (N)Passkey              via UserID
User(1) → (N)BlockedRefreshToken  via UserID
User(1) → (1)UserPreferences      via UserID (PK)
User(1) → (N)RoomParticipant      via UserID (FK)
User(1) → (N)RoomPermissions      via UserID (FK)
Room(1) → (N)RoomParticipant      via RoomID (FK)
Room(1) → (N)RoomPermissions      via RoomID (FK)
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
| `DeleteUser(userID)` | Cascade: participants → permissions → blocked tokens → user |

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
| `AdminDeleteRoom(roomID)` | Same, no owner check |
| `GetAllRooms()` / `GetAllActiveRooms()` | List rooms |
| `GetRoomsCreatedByUser(userID)` | User's created rooms |
| `GetRoomsParticipatedInByUser(userID)` | Rooms user joined |
| `SetRoomIdle(roomID)` | Mark inactive |
| `CleanupExpiredRooms()` | Bulk mark expired inactive. Excludes persistent rooms (`settings_is_persistent = false`) |
| `GetUserByID(userID)` | Fetch user (for participant lookups) |
| `CountActiveParticipants()` | Distinct count across all rooms |

### `passkey_repository.go` — `PasskeyRepository{*gorm.DB}`

`CreatePasskey`, `GetPasskeyByCredentialID`, `GetPasskeysByUserID`, `UpdatePasskeyCounter`, `DeletePasskey`.

### `settings_repository.go` — `SettingsRepository{*gorm.DB}`

`GetSettings()` — FirstOrCreate ID=1, default RegistrationEnabled=true.
`SaveSettings(s)` — Force ID=1, upsert.

### `invite_token_repository.go` — `InviteTokenRepository{*gorm.DB}`

`Create(t)`, `List()` (newest first), `GetByToken(token)`, `MarkUsed(tokenID, userID)`, `Delete(tokenID)`.

### `user_preferences_repository.go` — `UserPreferencesRepository{*gorm.DB}`

`GetByUserID(userID)` — `nil, nil` if not found.
`Upsert(userID, prefsJSON)` — `ON CONFLICT ... UPDATE ALL`.

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

`RunMigrations()` — AutoMigrate: User, BlockedRefreshToken, Room, RoomParticipant, RoomPermissions, Passkey, SystemSettings, InviteToken, UserPreferences. Raw SQL FK: `fk_room_permissions_participant`.

---

## `internal/livekit/` — Embedded Media Server

### `embed.go`

`Bin embed.FS` — contains `bin/livekit-server`. Build-tagged `!windows`.

### `config.go`

`ConfigYAML` — shared LiveKit YAML config struct. Used by both the installer (`internal/install/linux.go`) and the embedded server startup. Fields: `Port`, `BindAddresses`, `Keys`, `RTC` (tcp/udp ports, port range, node_ip), `TURN` (enabled, domain, udp/tls ports, cert/key), `Logging`.

### `server.go`

`ExportBinary(destPath)` — Write embedded binary with 0755. Remove existing first (avoid ETXTBSY).
`RunLiveKit(configPath)` — Run synchronously.
`generateTempConfig(apiKey, apiSecret, port, certFile, keyFile)` — Generate temp YAML with TURN/TLS for embedded mode. Returns temp file path.
`StartInternalServer(ctx, apiKey, apiSecret, port, cert, key, externalConfig)` — Background goroutine, 3s startup sleep. Skip if `LIVEKIT_MANAGED=true`. When cert/key provided and no external config, generates temp LiveKit YAML with TURN/TLS (port 5349) using server's certificate. Falls back to inline `--port`/`--keys` args if no TLS.

---

## `internal/scheduler/scheduler.go`

gocron scheduler. Every 1 min: `checkIdleRooms`.
`checkIdleRooms(roomRepo, cfg, client)` — List active DB rooms → query LK participant counts → mark idle if 0 participants + >5min old. Skips rooms with `IsPersistent = true`. Logs error on failure, info on success.
`Initialize(roomRepo, lkCfg)` — Setup + start async.
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

## `internal/usercli/usercli.go`

`PromoteUser(configPath, email)` — Add `superadmin` to Accesses.
`DemoteUser(configPath, email)` — Remove `superadmin`.
`CreateUser(configPath, email, password, name)` — bcrypt hash, insert.
`DeleteUser(configPath, email)` — Remove by email.
`withUser(configPath, email, fn)` — Load config + init DB + lookup user → call fn.

---

## Dependency graph

```
cmd/bedrud → server → handlers → repository → models
                   ↘            → auth → repository
                   ↘ middleware → auth
                   ↘ scheduler → repository + livekit
                   ↘ storage (standalone)
                   ↘ utils (standalone)
                   ↘ install → utils
                   ↘ usercli → repository
                   ↘ database → models (via migrations)
cmd/server → (same wiring, direct in main.go)
```
