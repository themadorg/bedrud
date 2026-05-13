---
name: bedrud-api
description: Complete Bedrud API endpoint reference. Every route, middleware, request/response shape, status code, DTO definition.
license: Apache License
---
# Skill: bedrud-api

Complete Bedrud API endpoint reference. Every route, middleware, request/response shape, status code, DTO definition.

---

## Authentication

### JWT Flow

Access + refresh token pair. HMAC-SHA256.

- Access token: configurable duration (`tokenDuration` in settings)
- Refresh token: 7-day expiry
- Tokens set as `HttpOnly` cookies (`access_token`, `refresh_token`) AND returned in JSON body
- Refresh rotation: old refresh token blocked on `POST /auth/refresh`

### Middleware

| Middleware | Behavior | Status on Fail |
|-----------|----------|----------------|
| `Protected()` | Extract JWT from `Authorization: Bearer` header, fallback `access_token` cookie. Store `*auth.Claims` in `c.Locals("user")` | 401 |
| `RequireAccess(level)` | Check `claims.Accesses` contains level. Chained after `Protected()` | 403 |
| `AuthRateLimiter()` | 10 req/min per IP | 429 |
| `GuestRateLimiter()` | 5 req/min per IP | 429 |

### Access Levels

`superadmin` > `admin` > `moderator` > `user` > `guest`

### Auth Header Format

```
Authorization: Bearer <access_token>
```

### Error Format

All errors: `{"error": "<message>"}` with appropriate HTTP status.

---

## Global Middleware (all routes)

| Order | Middleware | Purpose |
|-------|-----------|---------|
| 1 | `recover.New()` | Panic recovery |
| 2 | `helmet.New()` | XSS, Content-Type nosniff, X-Frame DENY |
| 3 | `cors.New()` | Config-driven origins/headers/methods |
| 4 | Body limit: 2MB | Custom Fiber config |

---

## Health

| Method | Path | Auth | Handler | Res |
|--------|------|------|---------|-----|
| GET | `/api/health` | none | `healthCheck` | `{"status":"healthy","time":<unix>}` |
| GET | `/api/ready` | none | `readinessCheck` | `{"status":"ready","time":<unix>}` |
| GET | `/health` | none | redirect | 307 → `/api/health` |
| GET | `/ready` | none | redirect | 307 → `/api/ready` |

---

## Auth — Local

| Method | Path | Auth | Rate Limit | Handler | Req | Res | Status |
|--------|------|------|-----------|---------|-----|-----|--------|
| POST | `/api/auth/register` | none | AuthRate | `authHandler.Register` | `{email, password, name, inviteToken}` | `LoginResponse` | 201 / 400 / 403 |
| POST | `/api/auth/login` | none | AuthRate | `authHandler.Login` | `{email, password}` | `LoginResponse` | 200 / 401 |
| POST | `/api/auth/guest-login` | none | AuthRate | `authHandler.GuestLogin` | `{name}` | `LoginResponse` | 200 / 400 |
| POST | `/api/auth/refresh` | none | AuthRate | `authHandler.RefreshToken` | `RefreshRequest` | `{accessToken, refreshToken}` | 200 / 401 |
| POST | `/api/auth/logout` | Protected | — | `authHandler.Logout` | `LogoutRequest` | `{"message":"Successfully logged out"}` | 200 |
| GET | `/api/auth/me` | Protected | — | `authHandler.GetMe` | — | `models.User` | 200 |
| PUT | `/api/auth/me` | Protected | — | `authHandler.UpdateProfile` | `{name}` | `models.User` | 200 / 400 |
| PUT | `/api/auth/password` | Protected | — | `authHandler.ChangePassword` | `{currentPassword, newPassword}` | `{"message":"Password updated successfully"}` | 200 / 400 / 401 |

### Notes

- Register: checks `registrationEnabled` + `tokenRegistrationOnly` settings. If token-only, `inviteToken` required.
- Guest login: creates transient user with `guest-` prefixed ID, `guest` access level.
- Password min length: 6 chars. Display name min: 2 chars.
- Logout: blocks refresh token + clears cookies.

---

## Auth — OAuth (Goth)

| Method | Path | Auth | Handler | Res |
|--------|------|------|---------|-----|
| GET | `/api/auth/:provider/login` | none | `BeginAuthHandler` | 307 redirect to provider |
| GET | `/api/auth/:provider/callback` | none | `CallbackHandler` | Redirect to `/auth/callback?token=...` |

### Providers

`google`, `github`, `twitter` (configured via SystemSettings client ID/secret).

### Flow

1. `GET /api/auth/google/login` → redirect to Google consent
2. Google redirects to `GET /api/auth/google/callback`
3. Upsert user (email + provider) → set JWT cookies → redirect to frontend with token

---

## Auth — Passkeys (WebAuthn)

| Method | Path | Auth | Rate Limit | Handler | Req | Res |
|--------|------|------|-----------|---------|-----|-----|
| POST | `/api/auth/passkey/register/begin` | Protected | — | `authHandler.PasskeyRegisterBegin` | — | WebAuthn creation options |
| POST | `/api/auth/passkey/register/finish` | Protected | — | `authHandler.PasskeyRegisterFinish` | `{clientDataJSON, attestationObject}` | `{"message":"Passkey registered successfully"}` |
| POST | `/api/auth/passkey/login/begin` | none | AuthRate | `authHandler.PasskeyLoginBegin` | — | WebAuthn request options |
| POST | `/api/auth/passkey/login/finish` | none | AuthRate | `authHandler.PasskeyLoginFinish` | `{credentialId, clientDataJSON, authenticatorData, signature}` | `LoginResponse` |
| POST | `/api/auth/passkey/signup/begin` | none | AuthRate | `authHandler.PasskeySignupBegin` | `{email, name, inviteToken}` | WebAuthn creation options |
| POST | `/api/auth/passkey/signup/finish` | none | AuthRate | `authHandler.PasskeySignupFinish` | `{clientDataJSON, attestationObject}` | `LoginResponse` |

### Notes

- Register begin/finish: add passkey to existing logged-in user.
- Login begin/finish: authenticate existing user via passkey.
- Signup begin/finish: create new account + passkey in one flow (no password).
- RP ID derived from request origin. Relying party name: "Bedrud".

---

## Preferences

| Method | Path | Auth | Handler | Req | Res |
|--------|------|------|---------|-----|-----|
| GET | `/api/auth/preferences` | Protected | `preferencesHandler.GetPreferences` | — | `{"preferencesJson":"..."}` |
| PUT | `/api/auth/preferences` | Protected | `preferencesHandler.UpdatePreferences` | `{preferencesJson}` | `{"message":"Preferences updated"}` |

### Notes

- JSON string, max 4KB.
- Upsert: `ON CONFLICT (user_id) DO UPDATE`.

---

## Public Settings

| Method | Path | Auth | Handler | Res |
|--------|------|------|---------|-----|
| GET | `/api/auth/settings` | none | `adminHandler.GetPublicSettings` | `{registrationEnabled, tokenRegistrationOnly, passkeysEnabled, oauthProviders}` |

No secrets exposed.

---

## Rooms — CRUD + Join

| Method | Path | Auth | Handler | Req | Res | Status |
|--------|------|------|---------|-----|-----|--------|
| POST | `/api/room/create` | Protected | `roomHandler.CreateRoom` | `CreateRoomRequest` | inline room obj + livekitHost | 201 / 409 |
| POST | `/api/room/join` | Protected | `roomHandler.JoinRoom` | `JoinRoomRequest` | inline room obj + token + livekitHost | 200 / 404 |
| POST | `/api/room/guest-join` | GuestRate | `roomHandler.GuestJoinRoom` | `GuestJoinRoomRequest` | inline `{id, name, token, adminId, livekitHost}` | 200 / 404 |
| GET | `/api/room/list` | Protected | `roomHandler.ListRooms` | — | `[]models.Room` | 200 |
| DELETE | `/api/room/:roomId` | Protected | `roomHandler.DeleteRoom` | — | `{"status":"success"}` | 200 / 403 / 404 |
| PUT | `/api/room/:roomId/settings` | Protected | `roomHandler.UpdateSettings` | `{isPublic *bool, maxParticipants *int, settings *RoomSettings}` | `models.Room` | 200 / 403 / 404 |
| POST | `/api/room/:roomId/chat/upload` | Protected | `roomHandler.UploadChatImage` | multipart `{file}` | `ChatAttachment` | 200 / 400 / 413 |

### CreateRoomRequest

```go
{
    name            string       // auto-gen if empty
    maxParticipants int          // default 20
    isPublic        bool
    mode            string       // "standard" default
    settings        RoomSettings
}
```

### Create Response

```json
{
    "id": "uuid",
    "name": "xxx-xxxx-xxx",
    "createdBy": "uuid",
    "isActive": true,
    "isPublic": false,
    "maxParticipants": 20,
    "settings": {},
    "livekitHost": "ws://...",
    "mode": "standard"
}
```

### Join Response

```json
{
    "id": "uuid",
    "name": "room-name",
    "token": "lk-jwt-token",
    "createdBy": "uuid",
    "adminId": "uuid",
    "isActive": true,
    "isPublic": false,
    "maxParticipants": 20,
    "expiresAt": "2025-01-02T00:00:00Z",
    "settings": {},
    "livekitHost": "ws://...",
    "mode": "standard"
}
```

### Notes

- Create: auto-generates name if empty (`xxx-xxxx-xxx`). 409 on name conflict. Strips `isPersistent` from settings (superadmin-only, set via AdminUpdateRoom). Creator auto-added as approved participant + admin perms. 24h expiry.
- Join: looks up room by name. Adds participant. Generates LK token with user metadata. Rejects banned users.
- Guest join: public rooms only. Restricted LK token. `guest-` prefixed identity.
- Delete: creator or superadmin only.
- Settings update: partial — only sent fields updated (pointer fields). Preserves `isPersistent` (only changeable via AdminUpdateRoom).
- Chat upload: MIME must be png/jpeg/gif/webp. SHA256 content hash filename. Max size from `chatUploadMaxBytes` config.

---

## Rooms — Moderation

All require `Protected()`. All use path params `:roomId` + `:identity`.

| Method | Path | Handler | Action | Res |
|--------|------|---------|--------|-----|
| POST | `/api/room/:roomId/kick/:identity` | `KickParticipant` | Remove from LK + broadcast "kick" system msg | `{"status":"success"}` |
| POST | `/api/room/:roomId/ban/:identity` | `BanParticipant` | Remove from LK + mark banned in DB + broadcast "ban" | `{"status":"success"}` |
| POST | `/api/room/:roomId/mute/:identity` | `MuteParticipant` | Mute all audio tracks via LK | `{"status":"success"}` |
| POST | `/api/room/:roomId/video/:identity/off` | `DisableParticipantVideo` | Mute camera track | `{"status":"success"}` |
| POST | `/api/room/:roomId/screenshare/:identity/stop` | `StopScreenShare` | Mute screen-share + screen-share-audio tracks | `{"status":"success"}` |
| POST | `/api/room/:roomId/promote/:identity` | `PromoteParticipant` | Add "moderator" to LK metadata | `{"status":"success"}` |
| POST | `/api/room/:roomId/demote/:identity` | `DemoteParticipant` | Remove "moderator" from LK metadata | `{"status":"success"}` |
| POST | `/api/room/:roomId/chat/:identity/block` | `BlockChat` | Set `chatBlocked: true` in LK metadata | `{"status":"success"}` |
| POST | `/api/room/:roomId/deafen/:identity` | `DeafenParticipant` | Send targeted "deafen" data msg | `{"status":"success"}` |
| POST | `/api/room/:roomId/undeafen/:identity` | `UndeafenParticipant` | Send targeted "undeafen" data msg | `{"status":"success"}` |
| POST | `/api/room/:roomId/ask/:identity/:action` | `AskParticipantAction` | Send "ask_unmute" or "ask_camera". `:action` = `unmute` or `camera` | `{"status":"success"}` |
| POST | `/api/room/:roomId/spotlight/:identity` | `SpotlightParticipant` | Broadcast "spotlight" to room | `{"status":"success"}` |
| GET | `/api/room/:roomId/participant/:identity/info` | `GetParticipantInfo` | Identity, name, state, tracks from LK | inline obj |
| POST | `/api/room/:roomId/stage/:identity/bring` | `BringToStage` | Stub | 501 |
| POST | `/api/room/:roomId/stage/:identity/remove` | `RemoveFromStage` | Stub | 501 |

### Participant Info Response

```json
{
    "identity": "uuid",
    "name": "John",
    "state": "ACTIVE",
    "joinedAt": "2025-01-01T12:00:00Z",
    "tracks": [
        {"sid": "TR_xxx", "type": "AUDIO", "source": "MICROPHONE", "muted": false}
    ]
}
```

### Authorization

- Room actions: creator, room admin, or user with `superadmin`/`admin` access.
- Self-info: participant can view own info. Admin/mod can view any.

---

## Online Count

| Method | Path | Auth | Handler | Res |
|--------|------|------|---------|-----|
| GET | `/api/room/online-count` | Protected | `roomHandler.GetOnlineCount` | `{"count": <int>}` |

---

## Admin — Users

All routes: `Protected()` + `RequireAccess(superadmin)`. Prefix: `/api/admin`.

| Method | Path | Handler | Req | Res | Status |
|--------|------|---------|-----|-----|--------|
| GET | `/api/admin/users` | `usersHandler.ListUsers` | query: `page`, `limit` | `{"users":[UserDetails],"total":int,"page":int,"limit":int}` | 200 |
| GET | `/api/admin/users/:id` | `usersHandler.GetUserDetail` | — | `{"user":UserDetails,"rooms":[Room]}` | 200 |
| PUT | `/api/admin/users/:id/status` | `usersHandler.UpdateUserStatus` | `{active: bool}` | `{"message":"User status updated successfully"}` | 200 |
| PUT | `/api/admin/users/:id/accesses` | `usersHandler.UpdateUserAccesses` | `{accesses: []string}` | `{"message":"User accesses updated"}` | 200 |
| DELETE | `/api/admin/users/:id` | `usersHandler.DeleteUser` | — | `202 {"message":"User deletion started","rooms":N}` (async 3-phase, 202 Accepted) | 202 / 400 / 403 / 404 / 500 |

### DeleteUser Notes
- **Self-deletion guard**: 400 if the requesting superadmin targets their own ID.
- **3-phase async goroutine**: Phase 1 (notify + stop LiveKit rooms, best-effort) → Phase 2 (hard-delete rooms from DB + chat upload cleanup, critical — abort on failure) → Phase 3 (delete passkeys, preferences, user record in transaction).
- **202 Accepted**: Returns immediately. Rooms stopped and participants see "This room has been deleted by an administrator".
- **Partial failure**: LiveKit failures are non-fatal (rooms auto-expire). DB room deletion failures abort the entire deletion (user stays intact).

### UserDetails DTO

```go
{
    id        string
    email     string
    name      string
    provider  string    // "local" | "google" | "github" | "twitter"
    isActive  bool
    isAdmin   bool      // computed: "admin" in accesses
    accesses  []string  // ["user","admin","superadmin",...]
    createdAt string
}
```

---

## Admin — Rooms

All routes: `Protected()` + `RequireAccess(superadmin)`. Prefix: `/api/admin`.

| Method | Path | Handler | Req | Res |
|--------|------|---------|-----|-----|
| GET | `/api/admin/rooms` | `roomHandler.AdminListRooms` | query: `page`, `limit` | `{"rooms":[],"total":int,"page":int,"limit":int}` |
| POST | `/api/admin/rooms/:roomId/close` | `roomHandler.AdminCloseRoom` | — | `{"status":"success"}` |
| PUT | `/api/admin/rooms/:roomId` | `roomHandler.AdminUpdateRoom` | `{maxParticipants *int, settings *AdminUpdateRoomSettingsInput}` | `models.Room` |
| POST | `/api/admin/rooms/:roomId/token` | `roomHandler.AdminGenerateToken` | — | 501 `{"error":"not yet implemented"}` |
| GET | `/api/admin/rooms/:roomId/participants` | `roomHandler.AdminGetRoomParticipants` | — | `{"participants":[...],"room":Room}` |
| POST | `/api/admin/rooms/:roomId/participants/:identity/kick` | `roomHandler.AdminKickParticipant` | — | `{"status":"success"}` |
| POST | `/api/admin/rooms/:roomId/participants/:identity/mute` | `roomHandler.AdminMuteParticipant` | — | `{"status":"success"}` |
| GET | `/api/admin/online-count` | `roomHandler.GetOnlineCount` | — | `{"count":int}` |
| GET | `/api/admin/livekit/stats` | `roomHandler.AdminLiveKitStats` | — | Stats obj |

### LiveKit Stats Response

```json
{
    "totalParticipants": 42,
    "totalPublishers": 10,
    "activeRooms": 5,
    "rooms": [
        {"room": "xxx-xxxx-xxx", "participants": 8, "publishers": 3}
    ]
}
```

### AdminUpdateRoom Request Body

```go
{
    maxParticipants *int                          // optional
    settings        *AdminUpdateRoomSettingsInput // optional, merge-patched
}

// AdminUpdateRoomSettingsInput — all fields are *bool for merge semantics
AdminUpdateRoomSettingsInput {
    allowChat       *bool
    allowVideo      *bool
    allowAudio      *bool
    requireApproval *bool
    e2ee            *bool
    isPersistent    *bool   // superadmin toggle for persistent room
}
```

### AdminUpdateRoom Notes

- Partial merge: only sent fields override existing room settings. Unsent fields unchanged.
- `isPersistent` can only be set via this endpoint (superadmin-only).
- After settings + maxParticipants update, the room is re-fetched from DB and returned.

### Admin Close vs Delete

- `close`: removes from LK, sets `IsActive = false` in DB. Room record preserved.
- `DELETE /admin/rooms/:roomId` (normal delete): creator or superadmin. Removes from LK + DB.
- `DELETE /api/admin/users/:id` (hard delete user): superadmin-only. 3-phase async: closes all user's rooms (LK + DB + chat upload files), then deletes user + passkeys + preferences.

---

## Admin — Settings

All routes: `Protected()` + `RequireAccess(superadmin)`. Prefix: `/api/admin`.

| Method | Path | Handler | Req | Res |
|--------|------|---------|-----|-----|
| GET | `/api/admin/settings` | `adminHandler.GetSettings` | — | `SystemSettings` (secrets masked) |
| PUT | `/api/admin/settings` | `adminHandler.UpdateSettings` | `SystemSettings` (full body) | `SystemSettings` (secrets masked) |

### Masked Fields in Response

These fields return `"******"` instead of actual values:
- `googleClientSecret`
- `githubClientSecret`
- `twitterClientSecret`
- `jwtSecret`
- `sessionSecret`
- `livekitApiSecret`
- `chatUploadS3SecretKey`

### Notes

- PUT replaces entire settings object. Send all fields.
- Singleton: always ID=1.

---

## Admin — Invite Tokens

All routes: `Protected()` + `RequireAccess(superadmin)`. Prefix: `/api/admin`.

| Method | Path | Handler | Req | Res | Status |
|--------|------|---------|-----|-----|--------|
| GET | `/api/admin/invite-tokens` | `adminHandler.ListInviteTokens` | — | `{"tokens":[{InviteToken + used bool}]}` | 200 |
| POST | `/api/admin/invite-tokens` | `adminHandler.CreateInviteToken` | `{email string, expiresIn int}` | `InviteToken` | 201 |
| DELETE | `/api/admin/invite-tokens/:id` | `adminHandler.DeleteInviteToken` | — | `{"status":"success"}` | 200 |

### Notes

- Token: crypto-random hex, varchar(64).
- Email: optional pre-bind. If set, only that email can use the token.
- `expiresIn`: hours. Default 72.
- List response adds computed `used` bool (true if `usedAt` not nil).

---

## All DTO Definitions

### auth.LoginResponse

```go
type LoginResponse struct {
    User  *models.User `json:"user"`
    Token TokenPair    `json:"tokens"`
}
```

### auth.TokenPair

```go
type TokenPair struct {
    AccessToken  string `json:"accessToken"`
    RefreshToken string `json:"refreshToken"`
}
```

### auth.Claims (JWT payload)

```go
type Claims struct {
    UserID   string   `json:"userId"`
    Email    string   `json:"email"`
    Name     string   `json:"name"`
    Provider string   `json:"provider"`
    Accesses []string `json:"accesses"`
    // + jwt.RegisteredClaims (sub, exp, iat, etc.)
}
```

### handlers.ErrorResponse

```go
type ErrorResponse struct {
    Error string `json:"error"`
}
```

### handlers.UserResponse

```go
type UserResponse struct {
    ID        string `json:"id"`
    Email     string `json:"email"`
    Name      string `json:"name"`
    Provider  string `json:"provider"`
    AvatarURL string `json:"avatarUrl"`
}
```

### handlers.UserDetails

```go
type UserDetails struct {
    ID        string   `json:"id"`
    Email     string   `json:"email"`
    Name      string   `json:"name"`
    Provider  string   `json:"provider"`
    IsActive  bool     `json:"isActive"`
    IsAdmin   bool     `json:"isAdmin"`
    Accesses  []string `json:"accesses"`
    CreatedAt string   `json:"createdAt"`
}
```

### handlers.UserStatusUpdateRequest

```go
type UserStatusUpdateRequest struct {
    Active bool `json:"active"`
}
```

### handlers.RefreshRequest

```go
type RefreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}
```

### handlers.LogoutRequest

```go
type LogoutRequest struct {
    RefreshToken string `json:"refresh_token"`
}
```

### handlers.CreateRoomRequest

```go
type CreateRoomRequest struct {
    Name            string              `json:"name"`
    MaxParticipants int                 `json:"maxParticipants"`
    IsPublic        bool                `json:"isPublic"`
    Mode            string              `json:"mode"`
    Settings        models.RoomSettings `json:"settings"`
}
```

### handlers.JoinRoomRequest

```go
type JoinRoomRequest struct {
    RoomName string `json:"roomName"`
}
```

### handlers.GuestJoinRoomRequest

```go
type GuestJoinRoomRequest struct {
    RoomName  string `json:"roomName"`
    GuestName string `json:"guestName"`
}
```

### models.RoomSettings

```go
type RoomSettings struct {
    AllowChat       bool `json:"allowChat"       default:true`
    AllowVideo      bool `json:"allowVideo"      default:true`
    AllowAudio      bool `json:"allowAudio"      default:true`
    RequireApproval bool `json:"requireApproval" default:false`
    E2EE            bool `json:"e2ee"            default:false`
    IsPersistent    bool `json:"isPersistent"    default:false` // superadmin-only, skips idle cleanup
}
```

### models.Room

```go
type Room struct {
    ID              string       `json:"id"`
    Name            string       `json:"name"`
    CreatedBy       string       `json:"createdBy"`
    IsActive        bool         `json:"isActive"`
    MaxParticipants int          `json:"maxParticipants"`
    CreatedAt       time.Time    `json:"createdAt"`
    UpdatedAt       time.Time    `json:"updatedAt"`
    ExpiresAt       time.Time    `json:"expiresAt"`
    AdminID         string       `json:"adminId"`
    IsPublic        bool         `json:"isPublic"`
    Settings        RoomSettings `json:"settings"`
    Mode            string       `json:"mode"`
}
```

### models.User

```go
type User struct {
    ID        string      `json:"id"`
    Email     string      `json:"email"`
    Name      string      `json:"name"`
    Provider  string      `json:"provider"`
    AvatarURL string      `json:"avatarUrl"`
    Password  string      `json:"-"`           // never serialized
    Accesses  StringArray `json:"accesses"`
    IsActive  bool        `json:"isActive"`
    CreatedAt time.Time   `json:"createdAt"`
    UpdatedAt time.Time   `json:"updatedAt"`
}
```

### models.SystemSettings

```go
type SystemSettings struct {
    RegistrationEnabled   bool   `json:"registrationEnabled"   default:true`
    TokenRegistrationOnly bool   `json:"tokenRegistrationOnly" default:false`
    PasskeysEnabled       bool   `json:"passkeysEnabled"`
    GoogleClientID        string `json:"googleClientId"`
    GoogleClientSecret    string `json:"googleClientSecret"`   // masked
    GoogleRedirectURL     string `json:"googleRedirectUrl"`
    GithubClientID        string `json:"githubClientId"`
    GithubClientSecret    string `json:"githubClientSecret"`   // masked
    GithubRedirectURL     string `json:"githubRedirectUrl"`
    TwitterClientID       string `json:"twitterClientId"`
    TwitterClientSecret   string `json:"twitterClientSecret"`  // masked
    JWTSecret             string `json:"jwtSecret"`            // masked
    TokenDuration         int    `json:"tokenDuration"`
    SessionSecret         string `json:"sessionSecret"`        // masked
    FrontendURL           string `json:"frontendUrl"`
    ServerPort            string `json:"serverPort"`
    ServerHost            string `json:"serverHost"`
    ServerDomain          string `json:"serverDomain"`
    ServerEnableTLS       bool   `json:"serverEnableTls"`
    ServerCertFile        string `json:"serverCertFile"`
    ServerKeyFile         string `json:"serverKeyFile"`
    ServerUseACME         bool   `json:"serverUseAcme"`
    ServerEmail           string `json:"serverEmail"`
    BehindProxy           bool   `json:"behindProxy"`
    LiveKitHost           string `json:"livekitHost"`
    LiveKitAPIKey         string `json:"livekitApiKey"`
    LiveKitAPISecret      string `json:"livekitApiSecret"`     // masked
    LiveKitExternal       bool   `json:"livekitExternal"`
    CORSAllowedOrigins    string `json:"corsAllowedOrigins"`
    CORSAllowedHeaders    string `json:"corsAllowedHeaders"`
    CORSAllowedMethods    string `json:"corsAllowedMethods"`
    CORSAllowCredentials  bool   `json:"corsAllowCredentials"`
    CORSMaxAge            int    `json:"corsMaxAge"`
    ChatUploadBackend     string `json:"chatUploadBackend"`
    ChatUploadMaxBytes    int64  `json:"chatUploadMaxBytes"`
    ChatUploadInlineMax   int64  `json:"chatUploadInlineMax"`
    ChatUploadDiskDir     string `json:"chatUploadDiskDir"`
    ChatUploadS3Endpoint  string `json:"chatUploadS3Endpoint"`
    ChatUploadS3Bucket    string `json:"chatUploadS3Bucket"`
    ChatUploadS3Region    string `json:"chatUploadS3Region"`
    ChatUploadS3AccessKey string `json:"chatUploadS3AccessKey"`
    ChatUploadS3SecretKey string `json:"chatUploadS3SecretKey"` // masked
    ChatUploadS3PublicURL string `json:"chatUploadS3PublicUrl"`
    LogLevel              string `json:"logLevel"`
    UpdatedAt             time.Time `json:"updatedAt"`
}
```

### models.InviteToken

```go
type InviteToken struct {
    ID        string     `json:"id"`
    Token     string     `json:"token"`
    Email     string     `json:"email"`
    CreatedBy string     `json:"createdBy"`
    ExpiresAt time.Time  `json:"expiresAt"`
    UsedAt    *time.Time `json:"usedAt"`
    UsedBy    string     `json:"usedBy"`
    CreatedAt time.Time  `json:"createdAt"`
}
```

### models.UserPreferences

```go
type UserPreferences struct {
    UserID          string    `json:"userId"`
    PreferencesJSON string    `json:"preferencesJson"`
    UpdatedAt       time.Time `json:"updatedAt"`
}
```

### storage.ChatAttachment

```go
type ChatAttachment struct {
    URL    string `json:"url"`
    Mime   string `json:"mime"`
    Size   int64  `json:"size"`
    Width  int    `json:"w"`
    Height int    `json:"h"`
}
```

---

## Source File Index

| Concern | File |
|---------|------|
| Route registration | `cmd/server/main.go` |
| Shared handler DTOs | `internal/handlers/models.go` |
| Auth handler (local + passkey) | `internal/handlers/auth_handler.go` |
| OAuth handler | `internal/handlers/auth.go` |
| Room handler | `internal/handlers/room.go` |
| Users handler | `internal/handlers/users.go` |
| Admin handler | `internal/handlers/admin_handler.go` |
| Preferences handler | `internal/handlers/preferences_handler.go` |
| Auth middleware | `internal/middleware/auth.go` |
| Rate limit middleware | `internal/middleware/ratelimit.go` |
| Auth service DTOs | `internal/auth/auth.go` |
| JWT Claims | `internal/auth/jwt.go` |
| User model | `internal/models/user.go` |
| Room model + RoomSettings | `internal/models/room.go` |
| SystemSettings model | `internal/models/settings.go` |
| InviteToken model | `internal/models/invite_token.go` |
| UserPreferences model | `internal/models/user_preferences.go` |
| ChatAttachment DTO | `internal/storage/chat_upload.go` |
| ChatUploadTracker (Record + DeleteByRoom) | `internal/storage/chat_upload.go` |
| ChatUpload model | `internal/models/chat_upload.go` |
| Shared LiveKit helpers (NewClient, AuthContext, SendSystemMessage) | `internal/lkutil/lkutil.go` |
| User handler (DeleteUser, Shutdown) | `internal/handlers/users.go` |

---

## Swagger

- Swagger UI: `GET /api/swagger/*`
- Scalar UI: `GET /api/scalar`
- Base path: `/api`
- Security: Bearer token in Authorization header
- Regenerate: `make swagger-gen` (requires `swag` CLI)
