---
name: bedrud-api-types
description: All DTO definitions, source file index, Swagger reference.
license: Apache License
---

# Bedrud API — Type Definitions & Reference

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
    // + jwt.RegisteredClaims
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
    IsPersistent    bool `json:"isPersistent"    default:false`
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
    RegistrationEnabled     bool   `json:"registrationEnabled"   default:true`
    TokenRegistrationOnly   bool   `json:"tokenRegistrationOnly" default:false`
    PasskeysEnabled         bool   `json:"passkeysEnabled"`
    GoogleClientID          string `json:"googleClientId"`
    GoogleClientSecret      string `json:"googleClientSecret"`   // masked
    GoogleRedirectURL       string `json:"googleRedirectUrl"`
    GithubClientID          string `json:"githubClientId"`
    GithubClientSecret      string `json:"githubClientSecret"`   // masked
    GithubRedirectURL       string `json:"githubRedirectUrl"`
    TwitterClientID         string `json:"twitterClientId"`
    TwitterClientSecret     string `json:"twitterClientSecret"`  // masked
    JWTSecret               string `json:"jwtSecret"`            // masked
    TokenDuration           int    `json:"tokenDuration"`
    SessionSecret           string `json:"sessionSecret"`        // masked
    FrontendURL             string `json:"frontendUrl"`
    ServerPort              string `json:"serverPort"`
    ServerHost              string `json:"serverHost"`
    ServerDomain            string `json:"serverDomain"`
    ServerEnableTLS         bool   `json:"serverEnableTls"`
    ServerCertFile          string `json:"serverCertFile"`
    ServerKeyFile           string `json:"serverKeyFile"`
    ServerUseACME           bool   `json:"serverUseAcme"`
    ServerEmail             string `json:"serverEmail"`
    BehindProxy             bool   `json:"behindProxy"`
    LiveKitHost             string `json:"livekitHost"`
    LiveKitAPIKey           string `json:"livekitApiKey"`
    LiveKitAPISecret        string `json:"livekitApiSecret"`     // masked
    LiveKitExternal         bool   `json:"livekitExternal"`
    CORSAllowedOrigins      string `json:"corsAllowedOrigins"`
    CORSAllowedHeaders      string `json:"corsAllowedHeaders"`
    CORSAllowedMethods      string `json:"corsAllowedMethods"`
    CORSAllowCredentials    bool   `json:"corsAllowCredentials"`
    CORSMaxAge              int    `json:"corsMaxAge"`
    ChatUploadBackend       string `json:"chatUploadBackend"`
    ChatUploadMaxBytes      int64  `json:"chatUploadMaxBytes"`
    ChatUploadInlineMax     int64  `json:"chatUploadInlineMax"`
    ChatUploadDiskDir       string `json:"chatUploadDiskDir"`
    ChatUploadS3Endpoint    string `json:"chatUploadS3Endpoint"`
    ChatUploadS3Bucket      string `json:"chatUploadS3Bucket"`
    ChatUploadS3Region      string `json:"chatUploadS3Region"`
    ChatUploadS3AccessKey   string `json:"chatUploadS3AccessKey"`
    ChatUploadS3SecretKey   string `json:"chatUploadS3SecretKey"` // masked
    ChatUploadS3PublicURL   string `json:"chatUploadS3PublicUrl"`
    LogLevel                string `json:"logLevel"`
    UpdatedAt               time.Time `json:"updatedAt"`
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

### models.QueueStats
```go
type QueueStats struct {
    Pending         int64              `json:"pending"`
    Active          int64              `json:"active"`
    Done24h         int64              `json:"done24h"`
    Failed24h       int64              `json:"failed24h"`
    Total           int64              `json:"total"`
    MaxDepth        int64              `json:"maxDepth"`
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
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Error     string    `json:"error"`
    Attempts  int       `json:"attempts"`
    UpdatedAt time.Time `json:"updatedAt"`
    Age       string    `json:"age"`
}
```

### models.AdminUpdateRoomSettingsInput
```go
type AdminUpdateRoomSettingsInput struct {
    AllowChat       *bool `json:"allowChat"`
    AllowVideo      *bool `json:"allowVideo"`
    AllowAudio      *bool `json:"allowAudio"`
    RequireApproval *bool `json:"requireApproval"`
    E2EE            *bool `json:"e2ee"`
    IsPersistent    *bool `json:"isPersistent"`   // superadmin-only
}
```

### Overview Response Types
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
    Status        string      `json:"status"`     // healthy, degraded, down
    TLS           *TLSStatus  `json:"tls"`
    Realtime      string      `json:"realtime"`   // connected, disconnected
    AlertsCount   int         `json:"alertsCount"`
    UptimeSeconds int64       `json:"uptimeSeconds"`
    DBStatus      string      `json:"dbStatus"`   // connected, error
}

type OverviewKPIs struct {
    TotalUsers     KpiEntry `json:"totalUsers"`
    OnlineNow      KpiEntry `json:"onlineNow"`
    TotalRooms     KpiEntry `json:"totalRooms"`
    ActiveSessions KpiEntry `json:"activeSessions"`
    PendingActions KpiEntry `json:"pendingActions"`
}

type RoomComposition struct {
    Live       int `json:"live"`
    Public     int `json:"public"`
    Private    int `json:"private"`
    Persistent int `json:"persistent"`
    Stale      int `json:"stale"`
}

type DayActivity struct {
    Date         string `json:"date"`
    RoomsCreated int    `json:"roomsCreated"`
    RoomsActive  int    `json:"roomsActive"`
    Participants int    `json:"participants"`
}

type RoomEvent struct {
    Type      string    `json:"type"`
    RoomID    string    `json:"roomId"`
    RoomName  string    `json:"roomName"`
    UserID    string    `json:"userId"`
    UserName  string    `json:"userName"`
    Timestamp time.Time `json:"timestamp"`
}

type RecentUser struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Email     string `json:"email"`
    Provider  string `json:"provider"`
    CreatedAt string `json:"createdAt"`
}
```

> **TODO oncoming feature:** Recording functionality is planned for a future release.

### Recording Webhook Payloads (queue)

```go
// handler_dispatch_webhook.go
type WebhookPayload struct {
    URL     string `json:"url"`
    Event   string `json:"event"`
    Body    string `json:"body"`
    Secret  string `json:"secret"`
}

// handler_process_recording.go
type ProcessRecordingPayload struct {
    RoomID        string `json:"roomId"`
    RoomName      string `json:"roomName"`
    EgressID      string `json:"egressId"`
    FileURL       string `json:"fileUrl"`
    EgressInfoJSON string `json:"egressInfoJson"`
}
```

> **TODO oncoming feature:** Recording functionality is planned for a future release.

### Recording Model (simplified)

```go
type Recording struct {
    ID            string          `json:"id"`
    RoomID        string          `json:"roomId"`
    RoomName      string          `json:"roomName"`
    EgressID      string          `json:"egressId"`
    Status        RecordingStatus `json:"status"`   // pending, started, processing, completed, failed
    RecordingType string          `json:"recordingType"`
    FileURL       string          `json:"fileUrl,omitempty"`
    FileSize      int64           `json:"fileSize,omitempty"`
    DurationMs    int64           `json:"durationMs,omitempty"`
    CreatedBy     string          `json:"createdBy"`
    CreatedAt     time.Time       `json:"createdAt"`
    UpdatedAt     time.Time       `json:"updatedAt"`
}
```

### Webhook Model (simplified)

```go
type Webhook struct {
    ID         string    `json:"id"`
    URL        string    `json:"url"`
    Events     []string  `json:"events"`
    IsActive   bool      `json:"isActive"`
    LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
    LastStatus int       `json:"lastStatus"`
    CreatedAt  time.Time  `json:"createdAt"`
}
```

---

## Source File Index

| Concern | File |
|---------|------|
| Route registration | `cmd/server/main.go` |
| Shared handler DTOs | `internal/handlers/models.go` |
| Auth handler (local + passkey) | `internal/handlers/auth_handler.go` |
> **TODO oncoming feature:** Recording functionality is planned for a future release.
| Recording handler | `internal/handlers/recording_handler.go` |
| Recording model | `internal/models/recording.go` |
| Webhook repository | `internal/repository/webhook_repository.go` |
| Recording repository | `internal/repository/recording_repository.go` |
| Recording service | `internal/services/recording_service.go` |
| OAuth handler | `internal/handlers/auth.go` |
| Room handler | `internal/handlers/room.go` |
| Users handler | `internal/handlers/users.go` |
| Admin handler | `internal/handlers/admin_handler.go` |
| Preferences handler | `internal/handlers/preferences_handler.go` |
| Admin overview handler | `internal/handlers/admin_overview.go` |
| TLS cert handler | `internal/handlers/cert_handler.go` |
| LiveKit webhook handler | `internal/handlers/livekit_webhook.go` |
| Cooldown cache | `internal/handlers/cooldown.go` |
| Room auth helper | `internal/handlers/room_auth.go` |
| Shared error helpers | `internal/handlers/errors.go` |
| Auth middleware | `internal/middleware/auth.go` |
| Rate limit middleware | `internal/middleware/ratelimit.go` |
| Auth service | `internal/auth/auth.go` |
| Challenge store (WebAuthn) | `internal/auth/challenge_store.go` |
| Email canonicalization | `internal/auth/email.go` |
| JWT Claims + banned set | `internal/auth/jwt.go` |
| Session store | `internal/auth/session_store.go` |
| User model | `internal/models/user.go` |
| Room model + RoomSettings | `internal/models/room.go` |
| SystemSettings model | `internal/models/settings.go` |
| InviteToken model | `internal/models/invite_token.go` |
| UserPreferences model | `internal/models/user_preferences.go` |
| ChatAttachment DTO | `internal/storage/chat_upload.go` |
| ChatUpload model | `internal/models/chat_upload.go` |
| User handler | `internal/handlers/users.go` |
| Admin queue handler | `internal/handlers/admin_queue.go` |
| Job GORM model | `internal/models/job.go` |
| QueueStats model | `internal/models/queue_stats.go` |
| Queue engine | `internal/queue/queue.go` |
| Queue worker | `internal/queue/worker.go` |
| Queue job types | `internal/queue/job.go` |
| Queue handlers | `internal/queue/handler_*.go` |
| Email SMTP handler | `internal/queue/handler_email.go` |
| RoomCleanupService | `internal/services/room_cleanup.go` |
| Overview response DTOs | `internal/models/stats.go` |
| Verification event model | `internal/models/verification_event.go` |
| Verification event repo | `internal/repository/verification_event_repository.go` |
| TLS/Key/Net/SafeIO utilities | `internal/utils/tls.go`, `keys.go`, `net.go`, `safeio.go` |

---

## Swagger

- Swagger UI: `GET /api/swagger/*`
- Scalar UI: `GET /api/scalar`
- Base path: `/api`
- Security: Bearer token in Authorization header
- Regenerate: `make swagger-gen` (requires `swag` CLI)
