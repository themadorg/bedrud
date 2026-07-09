---
name: bedrud-api-types
description: All DTO definitions, source file index, Swagger reference.
license: Apache License
---

# Bedrud API — Type Definitions & Reference

Canonical request/response and model JSON shapes used by the Go API. Paths are relative to `server/`.

Cross-check: `docs/swagger.yaml` definitions section. Regen: `make swagger-gen`.

---

## Auth package (`internal/auth/`)

### auth.ErrorResponse
```go
type ErrorResponse struct {
    Error string `json:"error"`
}
```

### auth.RegisterRequest
```go
type RegisterRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    Name     string `json:"name"`
}
```
Handler also accepts `inviteToken` (anonymous struct in `auth_handler.go`).

### auth.LoginRequest
```go
type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}
```

### auth.GuestLoginRequest
```go
type GuestLoginRequest struct {
    Name string `json:"name"`
}
```

### auth.TokenResponse
```go
type TokenResponse struct {
    AccessToken  string `json:"accessToken"`
    RefreshToken string `json:"refreshToken"`
}
```

### auth.TokenPair
Same shape as `TokenResponse` (`accessToken`, `refreshToken`).

### auth.LoginResponse
```go
type LoginResponse struct {
    User  *models.User `json:"user"`
    Token TokenPair    `json:"tokens"`
}
```

### auth.LogoutRequest
```go
type LogoutRequest struct {
    RefreshToken string `json:"refresh_token"`
}
```

### auth.Claims (JWT payload)
```go
type Claims struct {
    UserID            string     `json:"userId"`
    Email             string     `json:"email"`
    Name              string     `json:"name"`
    Provider          string     `json:"provider"`
    Accesses          []string   `json:"accesses"`
    Purpose           string     `json:"purpose,omitempty"`
    EmailVerifiedAt   *time.Time `json:"emailVerifiedAt,omitempty"`
    PasswordChangedAt *int64     `json:"passwordChangedAt,omitempty"` // unix ts
    // + jwt.RegisteredClaims
}
```

---

## Shared handler DTOs (`internal/handlers/models.go`)

### handlers.ErrorResponse
```go
type ErrorResponse struct {
    Error string `json:"error"`
}
```

### handlers.AuthResponse
```go
type AuthResponse struct {
    User  UserResponse `json:"user"`
    Token string       `json:"token"` // single access token (OAuth path)
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

### handlers.BulkIDsRequest / BulkItemResult / BulkResult
```go
type BulkIDsRequest struct {
    IDs []string `json:"ids"`
}

type BulkItemResult struct {
    Success bool   `json:"success"`
    Name    string `json:"name,omitempty"`
    Error   string `json:"error,omitempty"`
}

type BulkResult struct {
    Results        map[string]BulkItemResult `json:"results"`
    TotalProcessed int                       `json:"totalProcessed"`
    TotalFailed    int                       `json:"totalFailed"`
}
```

Password length constants (shared): `MinPasswordLength=12`, `MaxPasswordLength=128`.

---

## Auth handler request DTOs (`internal/handlers/auth_handler.go`)

Named types:
```go
type RefreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}
type LogoutRequest struct {
    RefreshToken string `json:"refresh_token"`
}
```

Anonymous body shapes (not exported):

| Endpoint | Body |
|----------|------|
| Register | `{email, password, name, inviteToken}` |
| Login | `{email, password}` |
| GuestLogin | `{name}` |
| UpdateProfile | `{name}` |
| ChangePassword | `{currentPassword, newPassword}` |
| ForgotPassword | `{email}` |
| ResetPassword | `{token, newPassword}` |
| VerifyEmail | `{token}` |
| ResendVerification | `{email}` |
| PasskeyRegisterFinish | `{clientDataJSON, attestationObject}` |
| PasskeyLoginFinish | `{credentialId, clientDataJSON, authenticatorData, signature}` |
| PasskeySignupBegin | `{email, name, inviteToken}` |
| PasskeySignupFinish | `{clientDataJSON, attestationObject}` |

---

## Users handler (`internal/handlers/users.go`)

### handlers.UserListResponse
```go
type UserListResponse struct {
    Users []UserDetails `json:"users"`
}
```
List endpoints also return `total`, `page`, `limit` via `fiber.Map`.

### handlers.UserDetails
```go
type UserDetails struct {
    ID              string   `json:"id"`
    Email           string   `json:"email"`
    Name            string   `json:"name"`
    Provider        string   `json:"provider"`
    IsActive        bool     `json:"isActive"`
    IsAdmin         bool     `json:"isAdmin"`
    Accesses        []string `json:"accesses"`
    EmailVerifiedAt *string  `json:"emailVerifiedAt,omitempty"`
    CreatedAt       string   `json:"createdAt"`
}
```

### handlers.UserStatusUpdateRequest / UserStatusUpdateResponse
```go
type UserStatusUpdateRequest struct {
    Active bool `json:"active"`
}
type UserStatusUpdateResponse struct {
    Message string `json:"message"`
}
```

Anonymous: UpdateUserAccesses `{accesses []string}`, SetUserPassword `{password}`.

---

## Room handler DTOs (`internal/handlers/room.go`)

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
Note: create forces `Settings.RecordingsAllowed = true`.

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

### handlers.RefreshLiveKitTokenRequest
```go
type RefreshLiveKitTokenRequest struct {
    RoomName string `json:"roomName"`
}
```
Response: `{"token": "..."}`.

### handlers.AdminUpdateRoomSettingsInput
```go
type AdminUpdateRoomSettingsInput struct {
    AllowChat       *bool `json:"allowChat"`
    AllowVideo      *bool `json:"allowVideo"`
    AllowAudio      *bool `json:"allowAudio"`
    RequireApproval *bool `json:"requireApproval"`
    E2EE            *bool `json:"e2ee"`
    IsPersistent    *bool `json:"isPersistent"` // superadmin-only merge field
}
```
Does **not** include `recordingsAllowed` (room settings model field is separate).

AdminUpdateRoom body: `{maxParticipants *int, settings *AdminUpdateRoomSettingsInput}`.

### Archived room list item (local)
```go
type ArchivedRoomDetail struct {
    ID             string    `json:"id"`
    Name           string    `json:"name"`
    CreatedAt      time.Time `json:"createdAt"`
    DeletedAt      time.Time `json:"deletedAt"`
    RecordingCount int       `json:"recordingCount"`
}
```
Wrapped as `{rooms, total, page, limit}`.

---

## Recording DTOs — SHIPPED

Types, models, handlers, services, queue payloads, and Swagger definitions exist and are tested.
Bootstrap wiring in `cmd/server/main.go` / `internal/server/server.go` may still comment route registration; treat **types as shipped**.

### handlers.RecordingDTO (`internal/handlers/recording_handler.go`)
API serialization type (not the GORM model). `DownloadStatus` is computed.
```go
type RecordingDTO struct {
    ID             string `json:"id"`
    RecordingType  string `json:"recordingType"`
    DurationMs     int64  `json:"durationMs"`
    FileSize       int64  `json:"fileSize"`
    FileURL        string `json:"fileUrl,omitempty"`
    Status         string `json:"status"`
    Error          string `json:"error,omitempty"`
    DownloadStatus string `json:"downloadStatus"` // ready | processing | failed
    RoomID         string `json:"roomId,omitempty"`
    RoomName       string `json:"roomName,omitempty"`
    CreatedBy      string `json:"createdBy,omitempty"`
    CreatedAt      string `json:"createdAt,omitempty"` // RFC3339
}
```

### Recording action responses (inline)
| Action | Status | Body |
|--------|--------|------|
| Start | 201 | `{id, status:"started", roomId}` |
| Stop | 200 | `{id, status:"processing"}` |
| List (room/admin) | 200 | `{recordings: []RecordingDTO, total, page, limit}` |
| Get | 200 | `RecordingDTO` |
| Wait (poll) | 200 | `{status, id?}` / `{status:"failed", error}` |
| Bulk delete | 202 | `BulkResult` |

Start/stop: no request body.

### models.Recording (`internal/models/recording.go`)
```go
type RecordingStatus string
// pending | started | processing | completed | failed | deleting

// Types: composite | audio | video | screen

type Recording struct {
    ID            string          `json:"id"`
    RoomID        string          `json:"roomId"`
    RoomName      string          `json:"roomName"`
    EgressID      string          `json:"egressId"`
    FileURL       string          `json:"fileUrl,omitempty"`
    FileSize      int64           `json:"fileSize"`
    RecordingType string          `json:"recordingType"`
    DurationMs    int64           `json:"durationMs,omitempty"`
    Status        RecordingStatus `json:"status"`
    Error         string          `json:"error,omitempty"`
    CreatedBy     string          `json:"createdBy"`
    StartedAt     *time.Time      `json:"startedAt,omitempty"`
    CompletedAt   *time.Time      `json:"completedAt,omitempty"`
    CreatedAt     time.Time       `json:"createdAt"`
    UpdatedAt     time.Time       `json:"updatedAt"`
}
```

### storage.RecordingAttachment
```go
type RecordingAttachment struct {
    URL  string `json:"url"`
    Size int64  `json:"size"`
}
```

---

## Webhook DTOs — SHIPPED

### models.Webhook (`internal/models/webhook.go`)
```go
// Events: room.created, room.ended, participant.joined,
//         recording.completed, ping
type Webhook struct {
    ID        string     `json:"id"`
    Name      string     `json:"name"`
    URL       string     `json:"url"`
    Secret    string     `json:"-"` // never in API except create/rotate plaintext
    Events    []string   `json:"events"`
    IsActive  bool       `json:"isActive"`
    LastSeen  *time.Time `json:"lastSeen"`
    CreatedBy string     `json:"createdBy"`
    CreatedAt time.Time  `json:"createdAt"`
    UpdatedAt time.Time  `json:"updatedAt"`
}
```

### handlers.webhookDTO / createWebhookRequest / updateWebhookRequest
```go
type webhookDTO struct {
    ID        string     `json:"id"`
    Name      string     `json:"name"`
    URL       string     `json:"url"`
    Secret    string     `json:"secret,omitempty"` // plaintext once on create/rotate; masked on list
    Events    []string   `json:"events"`
    IsActive  bool       `json:"isActive"`
    LastSeen  *time.Time `json:"lastSeen,omitempty"`
    CreatedBy string     `json:"createdBy"`
    CreatedAt time.Time  `json:"createdAt"`
    UpdatedAt time.Time  `json:"updatedAt"`
}

type createWebhookRequest struct {
    Name   string   `json:"name"`
    URL    string   `json:"url"`
    Events []string `json:"events"`
}

type updateWebhookRequest struct {
    Name     *string   `json:"name"`
    URL      *string   `json:"url"`
    Events   *[]string `json:"events"`
    IsActive *bool     `json:"isActive"`
}
```
List: `{webhooks, total, page, limit}`.

---

## Domain models

### models.User
```go
type User struct {
    ID                string      `json:"id"`
    Email             string      `json:"email"`
    Name              string      `json:"name"`
    Provider          string      `json:"provider"` // local | passkey | guest | oauth
    AvatarURL         string      `json:"avatarUrl"`
    Password          string      `json:"-"`
    RefreshToken      string      `json:"-"`
    Accesses          StringArray `json:"accesses"`
    IsActive          bool        `json:"isActive"`
    EmailVerifiedAt   *time.Time  `json:"emailVerifiedAt"`
    PasswordChangedAt *time.Time  `json:"passwordChangedAt"`
    CreatedAt         time.Time   `json:"createdAt"`
    UpdatedAt         time.Time   `json:"updatedAt"`
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
    LastActivityAt  *time.Time   `json:"lastActivityAt"`
    DeletedAt       *time.Time   `json:"deletedAt,omitempty"`
}
```

### models.RoomSettings
```go
type RoomSettings struct {
    AllowChat         bool `json:"allowChat"`         // default true
    AllowVideo        bool `json:"allowVideo"`        // default true
    AllowAudio        bool `json:"allowAudio"`        // default true
    RequireApproval   bool `json:"requireApproval"`   // default false
    E2EE              bool `json:"e2ee"`              // default false
    IsPersistent      bool `json:"isPersistent"`      // default false
    RecordingsAllowed bool `json:"recordingsAllowed"` // default false
}
```

### models.RoomParticipant / RoomPermissions
```go
type RoomParticipant struct {
    ID            string           `json:"id"`
    RoomID        string           `json:"roomId"`
    UserID        string           `json:"userId"`
    JoinedAt      time.Time        `json:"joinedAt"`
    LeftAt        *time.Time       `json:"leftAt"`
    IsActive      bool             `json:"isActive"`
    IsApproved    bool             `json:"isApproved"`
    IsMuted       bool             `json:"isMuted"`
    IsVideoOff    bool             `json:"isVideoOff"`
    IsChatBlocked bool             `json:"isChatBlocked"`
    IsBanned      bool             `json:"isBanned"`
    IsOnStage     bool             `json:"isOnStage"`
    IsModerator   bool             `json:"isModerator"`
    User          *User            `json:"user"`
    Room          *Room            `json:"room"`
    Permission    *RoomPermissions `json:"permission"`
}

type RoomPermissions struct {
    ID              string    `json:"id"`
    RoomID          string    `json:"roomId"`
    UserID          string    `json:"userId"`
    IsAdmin         bool      `json:"isAdmin"`
    CanKick         bool      `json:"canKick"`
    CanMuteAudio    bool      `json:"canMuteAudio"`
    CanDisableVideo bool      `json:"canDisableVideo"`
    CanChat         bool      `json:"canChat"`
    CreatedAt       time.Time `json:"createdAt"`
    UpdatedAt       time.Time `json:"updatedAt"`
}
```

### models.SystemSettings (key groups)
```go
type SystemSettings struct {
    ID                      uint   `json:"id"`
    RegistrationEnabled     bool   `json:"registrationEnabled"`
    TokenRegistrationOnly   bool   `json:"tokenRegistrationOnly"`
    PasskeysEnabled         bool   `json:"passkeysEnabled"`
    // OAuth: google*, github*, twitter* ClientID/Secret/RedirectURL
    JWTSecret               string `json:"jwtSecret"`            // masked
    TokenDuration           int    `json:"tokenDuration"`
    SessionSecret           string `json:"sessionSecret"`        // masked
    FrontendURL             string `json:"frontendUrl"`
    // Server: serverPort/Host/Domain/EnableTls/CertFile/KeyFile/UseAcme/Email, behindProxy
    ServerName              string `json:"serverName"`
    GuestLoginEnabled       bool   `json:"guestLoginEnabled"`
    // LiveKit: livekitHost/ApiKey/ApiSecret/External
    // CORS: corsAllowedOrigins/Headers/Methods, corsAllowCredentials, corsMaxAge
    // Chat uploads: chatUploadBackend/MaxBytes/InlineMax/DiskDir/S3*
    MaxParticipantsLimit    int    `json:"maxParticipantsLimit"`
    MaxRoomsPerUser         int    `json:"maxRoomsPerUser"`
    MaxUploadBytesPerUser   int64  `json:"maxUploadBytesPerUser"`
    GlobalDiskThresholdBytes int64 `json:"globalDiskThresholdBytes"`
    ChatMaxMessageCount     int    `json:"chatMaxMessageCount"`
    ChatMessageTTLHours     int    `json:"chatMessageTTLHours"`
    // Recordings (shipped fields)
    RecordingsEnabled        bool `json:"recordingsEnabled"`
    RecordingMaxDurationMins int  `json:"recordingMaxDurationMins"` // 0=unlimited
    RecordingMaxFileSizeMB   int  `json:"recordingMaxFileSizeMB"`   // 0=unlimited
    // Email branding + subject/preheader overrides + SMTP fields (email*)
    LogLevel  string    `json:"logLevel"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```
Masked secrets in admin GET: OAuth secrets, `jwtSecret`, `sessionSecret`, `livekitApiSecret`, `chatUploadS3SecretKey`, `emailPassword`.

### Public settings response (inline `GetPublicSettings`)
```json
{
  "serverName": "",
  "registrationEnabled": true,
  "tokenRegistrationOnly": false,
  "guestLoginEnabled": true,
  "passkeysEnabled": true,
  "oauthProviders": ["google"],
  "requireEmailVerification": false,
  "chatMaxMessageCount": 10000,
  "chatMessageTTLHours": 2160,
  "recordingsEnabled": false
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
List adds computed `used bool`. Create body: `{email?, expiresInHours?}` (default 72, max 720).

### models.UserPreferences
```go
type UserPreferences struct {
    UserID          string    `json:"userId"`
    PreferencesJSON string    `json:"preferencesJson"`
    UpdatedAt       time.Time `json:"updatedAt"`
}
```
Handler GET/PUT use `{preferencesJson}` only (max 4KB, must be JSON object).

### models.Passkey
```go
type Passkey struct {
    ID           string    `json:"id"`
    UserID       string    `json:"userId"`
    CredentialID []byte    `json:"credentialId"`
    PublicKey    []byte    `json:"publicKey"`
    Algorithm    int       `json:"algorithm"`
    Counter      uint32    `json:"counter"`
    Name         string    `json:"name"`
    CreatedAt    time.Time `json:"createdAt"`
}
```

### models.ChatUpload
```go
type ChatUpload struct {
    ID             string    `json:"id"`
    RoomID         string    `json:"roomId"`
    UploadedBy     string    `json:"uploadedBy"`
    FileHash       string    `json:"fileHash"`
    Extension      string    `json:"extension"`
    FileSize       int64     `json:"fileSize"`
    StorageBackend string    `json:"storageBackend"`
    CreatedAt      time.Time `json:"createdAt"`
}
```

### storage.ChatAttachment
```go
type ChatAttachment struct {
    Kind   string `json:"kind"`
    URL    string `json:"url"`
    Mime   string `json:"mime"`
    Size   int64  `json:"size"`
    Width  int    `json:"w"`
    Height int    `json:"h"`
}
```

### models.Job
```go
type JobStatus string // pending | active | done | failed

type Job struct {
    ID, Type, Payload string
    RunAt             time.Time
    Priority          int
    Status            JobStatus
    Attempts, MaxAttempts int
    LastError         string
    CreatedAt, UpdatedAt time.Time
}
// No json tags — internal queue table, not HTTP DTO
```

### models.VerificationEvent
```go
// eventType: sent | resent | success | failed | admin_force | email_change
type VerificationEvent struct {
    ID        uint      `json:"id"`
    UserID    string    `json:"userId"`
    Email     string    `json:"email"`
    EventType string    `json:"eventType"`
    IP        string    `json:"ip,omitempty"`
    Metadata  string    `json:"metadata,omitempty"`
    CreatedAt time.Time `json:"createdAt"`
}
```

### models.BlockedRefreshToken
```go
type BlockedRefreshToken struct {
    ID, Token, UserID string
    ExpiresAt, CreatedAt time.Time
}
```

---

## Overview / stats (`internal/models/stats.go`)

```go
type OverviewResponse struct {
    Health          OverviewHealth  `json:"health"`
    KPIs            OverviewKPIs    `json:"kpis"`
    ActivityTrend   []DayActivity   `json:"activityTrend"`
    RoomComposition RoomComposition `json:"roomComposition"`
    NeedsAttention  []AttentionItem `json:"needsAttention"`
    RecentSignups   []RecentUser    `json:"recentSignups"`
    RecentEvents    []RoomEvent     `json:"recentRoomEvents"`
    InstanceInfo    InstanceInfo    `json:"instanceInfo"`
}

type OverviewHealth struct {
    Status        string     `json:"status"`     // healthy | degraded | down
    TLS           *TLSStatus `json:"tls"`
    Realtime      string     `json:"realtime"`   // connected | disconnected
    AlertsCount   int        `json:"alertsCount"`
    UptimeSeconds int64      `json:"uptimeSeconds"`
    DBStatus      string     `json:"dbStatus"`   // connected | error
}

type TLSStatus struct {
    Enabled       bool   `json:"enabled"`
    DaysRemaining int    `json:"daysRemaining"`
    ExpiryDate    string `json:"expiryDate,omitempty"`
    Status        string `json:"status"` // valid | expiring | expired | unknown
}

type KpiEntry struct {
    Value        int    `json:"value"`
    Delta        int    `json:"delta,omitempty"`
    DeltaLabel   string `json:"deltaLabel,omitempty"`
    DeltaPercent int    `json:"deltaPercent,omitempty"`
    PeakToday    int    `json:"peakToday,omitempty"`
    ActiveNow    int    `json:"activeNow,omitempty"`
}

type OverviewKPIs struct {
    TotalUsers, OnlineNow, TotalRooms, ActiveSessions, PendingActions KpiEntry
}

type RoomComposition struct {
    Live, Public, Private, Persistent, Stale int
}

type AttentionItem struct {
    Type     string `json:"type"`     // tls_expiry, stale_room, empty_room, auth_spike
    Severity string `json:"severity"` // error | warning | info
    Message  string `json:"message"`
    DaysLeft int    `json:"daysLeft,omitempty"`
    RoomID   string `json:"roomId,omitempty"`
}

type InstanceInfo struct {
    Name, Version, StartedAt string
    UptimeSeconds            int64
}

type DayActivity struct {
    Date string `json:"date"`
    RoomsCreated, RoomsActive, Participants int
}

type RecentUser struct {
    ID, Name, Email, Provider, CreatedAt string
}

type RoomEvent struct {
    Type      string    `json:"type"` // room_created, room_joined
    RoomID    string    `json:"roomId,omitempty"`
    RoomName  string    `json:"roomName,omitempty"`
    UserID    string    `json:"userId,omitempty"`
    UserName  string    `json:"userName,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

type DayCount struct {
    Date  time.Time `json:"date"`
    Count int       `json:"count"`
}
```

---

## Queue stats (`internal/models/queue_stats.go`)

```go
type QueueStats struct {
    Pending, Active, Done24h, Failed24h, Total, MaxDepth int64
    OldestPending   *time.Time         `json:"oldestPending,omitempty"`
    RecentFailures  []FailedJobSummary `json:"recentFailures,omitempty"`
    ProcessedPerMin float64            `json:"processedPerMin"`
    FailedPerMin    float64            `json:"failedPerMin"`
    FailRate        float64            `json:"failRate"`
    PendingEmail    int64              `json:"pendingEmail"`
    FailedEmail24h  int64              `json:"failedEmail24h"`
    LastSendError   string             `json:"lastSendError,omitempty"`
    LastSendErrorAt *time.Time         `json:"lastSendErrorAt,omitempty"`
}

type FailedJobSummary struct {
    ID, Type, Error, Age string
    Attempts             int
    UpdatedAt            time.Time
}
```

---

## Queue job payloads (`internal/queue/job.go`) — SHIPPED

JSON tags are **snake_case** (internal job payload, not public REST).

```go
type UserDeletePayload struct {
    UserID  string   `json:"user_id"`
    Email   string   `json:"email"`
    RoomIDs []string `json:"room_ids,omitempty"`
}

type RoomDeletePayload struct {
    RoomID          string `json:"room_id"`
    SystemEvent     string `json:"system_event"`
    SystemMessage   string `json:"system_message"`
    DeletedIdentity string `json:"deleted_identity,omitempty"`
    Purge           bool   `json:"purge"` // true=hard delete+files; false=archive
}

type RoomSuspendPayload struct {
    RoomID string `json:"room_id"`
}

type ChatUploadS3Payload struct {
    Data, RoomID, MimeType, UserID string // data=base64
}

type SendEmailPayload struct {
    To, Subject, TemplateName string
    TemplateData              map[string]any `json:"template_data,omitempty"`
}

type WebhookPayload struct {
    URL    string         `json:"url"`
    Event  string         `json:"event"`
    Body   map[string]any `json:"body"`
    Secret string         `json:"secret,omitempty"`
}

type RecordingDeletePayload struct {
    RecordingID string `json:"recording_id"`
    RoomID      string `json:"room_id"`
    RoomName    string `json:"room_name"`
}

type ProcessRecordingPayload struct {
    RoomID, RoomName, EgressID, FileURL string
    FileSize                            int64  `json:"file_size"`
    RecordingType                       string `json:"recording_type"`
    DurationMs                          int64  `json:"duration_ms"`
    StartedAt                           string `json:"started_at,omitempty"` // RFC3339
}
```

---

## Cert (`internal/utils/tls.go`)

```go
type CertInfo struct {
    Subject       string    `json:"subject"`
    Issuer        string    `json:"issuer"`
    NotBefore     time.Time `json:"notBefore"`
    NotAfter      time.Time `json:"notAfter"`
    DaysRemaining int       `json:"daysRemaining"`
    SANs          []string  `json:"sans"`
    Status        string    `json:"status"`
}
```

---

## Source File Index

| Concern | File |
|---------|------|
| Route registration (dev Air) | `cmd/server/main.go` |
| Route registration (prod CLI) | `internal/server/server.go` |
| Shared handler DTOs | `internal/handlers/models.go` |
| Auth handler (local + passkey + verify/reset) | `internal/handlers/auth_handler.go` |
| OAuth handler | `internal/handlers/auth.go` |
| Room handler + room DTOs | `internal/handlers/room.go` |
| Room auth helper | `internal/handlers/room_auth.go` |
| Recording handler + RecordingDTO | `internal/handlers/recording_handler.go` |
| Users handler + UserDetails | `internal/handlers/users.go` |
| Admin settings / invites / webhooks | `internal/handlers/admin_handler.go` |
| Admin overview | `internal/handlers/admin_overview.go` |
| Admin queue stats | `internal/handlers/admin_queue.go` |
| Preferences handler | `internal/handlers/preferences_handler.go` |
| TLS cert handler | `internal/handlers/cert_handler.go` |
| LiveKit webhook handler | `internal/handlers/livekit_webhook.go` |
| Cooldown cache | `internal/handlers/cooldown.go` |
| Shared error helpers | `internal/handlers/errors.go` |
| Auth middleware | `internal/middleware/auth.go` |
| Rate limit middleware | `internal/middleware/ratelimit.go` |
| Recordings enabled middleware | `internal/middleware/recordings_enabled.go` |
| Auth service DTOs | `internal/auth/auth.go` |
| JWT Claims | `internal/auth/jwt.go` |
| Challenge store (WebAuthn) | `internal/auth/challenge_store.go` |
| Email canonicalization | `internal/auth/email.go` |
| Session store | `internal/auth/session_store.go` |
| User model | `internal/models/user.go` |
| Room + RoomSettings + participant | `internal/models/room.go` |
| Recording model | `internal/models/recording.go` |
| Webhook model | `internal/models/webhook.go` |
| SystemSettings | `internal/models/settings.go` |
| InviteToken | `internal/models/invite_token.go` |
| UserPreferences | `internal/models/user_preferences.go` |
| Passkey | `internal/models/passkey.go` |
| ChatUpload | `internal/models/chat_upload.go` |
| Job model | `internal/models/job.go` |
| QueueStats | `internal/models/queue_stats.go` |
| Overview/stats DTOs | `internal/models/stats.go` |
| VerificationEvent | `internal/models/verification_event.go` |
| BlockedRefreshToken | `internal/models/refresh.go` |
| ChatAttachment DTO | `internal/storage/chat_upload.go` |
| RecordingStore + RecordingAttachment | `internal/storage/recording_store.go` |
| Queue job payloads | `internal/queue/job.go` |
| Queue engine / worker | `internal/queue/queue.go`, `worker.go` |
| Queue handlers | `internal/queue/handler_*.go` |
| Recording service | `internal/services/recording_service.go` |
| RoomCleanupService | `internal/services/room_cleanup.go` |
| Recording repository | `internal/repository/recording_repository.go` |
| Webhook repository | `internal/repository/webhook_repository.go` |
| Verification event repo | `internal/repository/verification_event_repository.go` |
| CertInfo | `internal/utils/tls.go` |
| Swagger definitions | `docs/swagger.yaml` |

---

## Swagger

- Swagger UI: `GET /api/swagger/*`
- Scalar UI: `GET /api/scalar`
- Base path: `/api`
- Host (dev): `localhost:7071` (Vite proxy / make dev API port; prod uses server `httpPort`)
- Security: Bearer token in `Authorization` header
- Regenerate: `make swagger-gen` (requires `swag` CLI)

### Swagger-named definitions (current)

`auth.*`: ErrorResponse, LoginRequest, LoginResponse, RegisterRequest, TokenPair, TokenResponse  

`handlers.*`: AuthResponse, BulkIDsRequest, BulkItemResult, BulkResult, CreateRoomRequest, ErrorResponse, GuestJoinRoomRequest, JoinRoomRequest, LogoutRequest, RecordingDTO, RefreshLiveKitTokenRequest, RefreshRequest, UserDetails, UserListResponse, UserResponse, createWebhookRequest, updateWebhookRequest, webhookDTO  

`models.*`: AttentionItem, DayActivity, FailedJobSummary, InstanceInfo, InviteToken, KpiEntry, OverviewHealth, OverviewKPIs, OverviewResponse, QueueStats, RecentUser, Room, RoomComposition, RoomEvent, RoomSettings, SystemSettings, TLSStatus, User  

`utils.CertInfo`
