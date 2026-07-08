# Database Models (`internal/models`)

GORM models defining database tables and domain types. Auto-migrated by `database.RunMigrations()`.

---

## User (`user.go`)

```go
User {
    ID                string      // varchar(36) PK
    Email             string      // unique with Provider (idx_email_provider)
    Name              string
    Provider          string      // local | passkey | guest | google | github | twitter
    AvatarURL         string
    Password          string      // json:"-", bcrypt hash
    RefreshToken      string      // json:"-"
    Accesses          StringArray // []string, PG text[]
    IsActive          bool        // default true; false = banned (in-memory set)
    EmailVerifiedAt   *time.Time  // nil = unverified
    PasswordChangedAt *time.Time  // invalidates old password-reset tokens
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

Provider constants: `ProviderLocal`, `ProviderPasskey`, `ProviderGuest`.

### Access levels

```go
const (
    AccessSuperadmin AccessLevel = "superadmin"
    AccessAdmin      AccessLevel = "admin"
    AccessModerator  AccessLevel = "moderator"
    AccessUser       AccessLevel = "user"
    AccessGuest      AccessLevel = "guest"
)
```

Methods: `HasAccess(level)`, `IsAdmin()` (checks `"admin"` in Accesses).

`StringArray` — custom type with `sql.Scanner`, `driver.Valuer`, `GormDataTypeInterface` for PostgreSQL `text[]`.

---

## Room (`room.go`)

```go
Room {
    ID              string
    Name            string       // unique, URL-safe slug
    CreatedBy       string
    IsActive        bool
    MaxParticipants int          // default 20
    AdminID         string
    IsPublic        bool
    Settings        RoomSettings // embedded, prefix settings_
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
```

### Room participants

```go
RoomParticipant {
    ID            string
    RoomID        string     // composite unique with UserID
    UserID        string
    JoinedAt      time.Time
    LeftAt        *time.Time
    IsActive      bool
    IsApproved    bool
    IsMuted       bool
    IsVideoOff    bool
    IsChatBlocked bool
    IsBanned      bool
    IsOnStage     bool
}

RoomPermissions {
    ID              string
    RoomID          string
    UserID          string
    IsAdmin         bool
    CanKick         bool
    CanMuteAudio    bool
    CanDisableVideo bool
    CanChat         bool
}
```

### Room name validation

`ValidateRoomName(name)` — 3–63 chars, lowercase alphanumeric + hyphens, no consecutive/leading/trailing hyphens.

`GenerateRandomRoomName()` — crypto-random `xxx-xxxx-xxx`.

Sentinel errors: `ErrRoomNameInvalid`, `ErrRoomNameTooShort`, `ErrRoomNameTooLong`, `ErrRoomNameTaken`.

---

## Passkey (`passkey.go`)

```go
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

---

## BlockedRefreshToken (`refresh.go`)

```go
BlockedRefreshToken {
    ID        string
    Token     string     // unique
    UserID    string     // indexed
    ExpiresAt time.Time  // indexed, for cleanup
    CreatedAt time.Time
}
```

---

## SystemSettings (`settings.go`)

Singleton (always ID=1). Admin panel reads/writes the full struct; secrets masked in API responses.

### Registration & instance

| Field | Default | Purpose |
|-------|---------|---------|
| `registrationEnabled` | true | Global registration on/off |
| `tokenRegistrationOnly` | false | Require invite token |
| `passkeysEnabled` | true | WebAuthn on/off |
| `guestLoginEnabled` | true | Guest login on/off |
| `serverName` | — | Display name |

### Auth (OAuth + JWT)

`googleClientId/Secret/RedirectUrl`, `github*`, `twitter*`, `jwtSecret`, `sessionSecret`, `tokenDuration`, `frontendUrl`

### Server & TLS

`serverPort`, `serverHost`, `serverDomain`, `serverEnableTls`, `serverCertFile`, `serverKeyFile`, `serverUseAcme`, `serverEmail`, `behindProxy`

### LiveKit

`livekitHost`, `livekitApiKey`, `livekitApiSecret`, `livekitExternal`

### CORS

`corsAllowedOrigins`, `corsAllowedHeaders`, `corsAllowedMethods`, `corsAllowCredentials`, `corsMaxAge`

### Chat uploads

`chatUploadBackend`, `chatUploadMaxBytes`, `chatUploadInlineMax`, `chatUploadDiskDir`, S3 fields (`chatUploadS3*`)

### Limits & quotas

`maxParticipantsLimit`, `maxRoomsPerUser`, `maxUploadBytesPerUser`, `globalDiskThresholdBytes`, `chatMaxMessageCount`, `chatMessageTTLHours`

### Recordings (planned — see planned-features.md)

`recordingsEnabled`, `recordingMaxDurationMins`, `recordingMaxFileSizeMB`

### Email branding & SMTP

Per-template subject/preheader overrides, SMTP fields (empty = fall back to `config.yaml`), `emailInstanceName`, colors.

Helper methods: `IsOAuthProviderConfigured(provider)`, `ConfiguredOAuthProviders()`.

---

## InviteToken (`invite_token.go`)

```go
InviteToken {
    ID        string
    Token     string     // unique, varchar(64)
    Email     string     // optional pre-bind
    CreatedBy string
    ExpiresAt time.Time
    UsedAt    *time.Time
    UsedBy    string
    CreatedAt time.Time
}
```

---

## UserPreferences (`user_preferences.go`)

```go
UserPreferences {
    UserID          string   // PK
    PreferencesJSON string   // text, default '{}'
    UpdatedAt       time.Time
}
```

---

## ChatUpload (`chat_upload.go`)

```go
ChatUpload {
    ID        string
    RoomID    string     // FK → rooms.id, ON DELETE CASCADE
    FileHash  string     // SHA-256 hex
    Extension string     // e.g. ".png"
    CreatedAt time.Time
}
```

Only disk-backend uploads tracked. S3 and inline uploads skipped.

---

## Job (`job.go`)

```go
Job {
    ID          string
    Type        string     // job type string
    Payload     string     // JSON
    RunAt       time.Time
    Priority    int        // lower = higher priority
    Status      JobStatus  // pending, active, done, failed
    Attempts    int
    MaxAttempts int        // default 3
    LastError   string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

---

## Recording (`recording.go`)

```go
Recording {
    ID            string
    RoomID        string
    EgressID      string
    RecordingType string   // "audio", "video", "screen", "composite"
    FilePath      string
    FileSize      int64
    DurationMs    int64
    Status        string   // "recording", "processing", "completed", "failed"
    StartedAt     time.Time
    CompletedAt   *time.Time
    CreatedAt     time.Time
}
```

---

## Webhook (`webhook.go`)

```go
Webhook {
    ID        string
    URL       string
    Secret    string
    Events    StringArray  // subscribed event types
    IsActive  bool
    CreatedAt time.Time
}

WebhookDelivery {
    ID         string
    WebhookID  string
    Event      string
    Status     string
    Response   string
    CreatedAt  time.Time
}
```

---

## VerificationEvent (`verification_event.go`)

Tracks email verification and password reset events for audit/cooldown.

---

## Stats DTOs

- `stats.go` — dashboard overview DTOs
- `queue_stats.go` — queue statistics for admin panel

---

## Relationships

```
User(1) → (N) Passkey
User(1) → (N) BlockedRefreshToken
User(1) → (1) UserPreferences
User(1) → (N) RoomParticipant
User(1) → (N) RoomPermissions
Room(1) → (N) RoomParticipant
Room(1) → (N) RoomPermissions
Room(1) → (N) ChatUpload
Room(1) → (N) Recording
RoomParticipant(1) ↔ (1) RoomPermissions  via (RoomID, UserID)
```