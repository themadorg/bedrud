---
name: bedrud-data
description: Data layer — GORM models, repository, database init, test utilities, admin DTOs.
license: Apache License
---

# Bedrud Data Layer

Go module `bedrud`. Root: `server/`. GORM ORM + SQLite/Postgres.
No SQL migration files — schema via `database.RunMigrations()` AutoMigrate + Postgres FKs.

---

## `internal/models/` — GORM Models

### `user.go`

```go
User {
  ID                string      // varchar(36) PK
  Email             string      // uniqueIndex:idx_email_provider with Provider
  Name              string
  Provider          string      // uniqueIndex:idx_email_provider; default "local"
  AvatarURL         string      // column:avatar_url
  Password          string      // json:"-", bcrypt hash
  RefreshToken      string      // json:"-"; stored as SHA-256 hex of raw token
  Accesses          StringArray // []string; GormDataType text[]; stored "{a,b}"
  IsActive          bool        // default true
  EmailVerifiedAt   *time.Time  // nullable, indexed
  PasswordChangedAt *time.Time  // set on password change
  CreatedAt         time.Time
  UpdatedAt         time.Time
}
```

Provider constants: `ProviderLocal`, `ProviderPasskey`, `ProviderGuest` (OAuth: `"google"`, `"github"`, `"twitter"` as strings).
`AccessLevel` enum: `superadmin`, `admin`, `moderator`, `user`, `guest`.
`StringArray`: `sql.Scanner` + `driver.Valuer` + `GormDataType() → "text[]"`.
Methods: `HasAccess(level)`, `IsAdmin()` (checks `admin` only, not superadmin).

### `room.go`

```go
Room {
  ID              string
  Name            string       // index (non-unique); active-name uniqueness via partial idx + repo
  CreatedBy       string
  IsActive        bool         // default true
  MaxParticipants int          // default 20
  AdminID         string       // room creator/admin
  IsPublic        bool         // default false
  Settings        RoomSettings // embedded, prefix settings_
  Mode            string       // default "standard", varchar(20)
  ExpiresAt       time.Time    // indexed
  LastActivityAt  *time.Time   // indexed; set on join; stale-room detection
  DeletedAt       *time.Time   // soft archive (not GORM soft-delete plugin)
  CreatedAt       time.Time
  UpdatedAt       time.Time
}

RoomSettings {
  AllowChat         bool // default true
  AllowVideo        bool // default true
  AllowAudio        bool // default true
  RequireApproval   bool // default false
  E2EE              bool // default false
  IsPersistent      bool // default false; skips idle cleanup
  RecordingsAllowed bool // default false; per-room recording gate
}

RoomParticipant {
  ID            string
  RoomID        string  // uniqueIndex:idx_room_user with UserID
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
  IsModerator   bool    // room-level moderator flag
  User          *User            // belongs-to
  Room          *Room            // belongs-to
  Permission    *RoomPermissions // gorm:"-"; not a DB column
}

RoomPermissions {
  ID              string
  RoomID          string // indexed
  UserID          string // indexed
  IsAdmin         bool
  CanKick         bool
  CanMuteAudio    bool
  CanDisableVideo bool
  CanChat         bool   // default true
  CreatedAt       time.Time
  UpdatedAt       time.Time
  // FK RoomParticipant via (RoomID,UserID) — composite, Postgres ON DELETE CASCADE
}
```

Tables: `rooms`, `room_participants`, `room_permissions`.
`ValidateRoomName(name)` — 3–63 chars, `^[a-z0-9]+(-[a-z0-9]+)*$`.
`GenerateRandomRoomName()` — crypto-random `xxx-xxxx-xxx` (letters only).
Sentinel errors: `ErrRoomNameInvalid`, `ErrRoomNameTooShort`, `ErrRoomNameTooLong`, `ErrRoomNameTaken`.

**Name uniqueness:** Only **active** rooms must have unique names (`idx_rooms_active_name` partial unique index). Idle/archived rooms allow name reuse.

### `passkey.go`

```go
Passkey {
  ID           string    // varchar(36) PK
  UserID       string    // indexed
  CredentialID []byte    // uniqueIndex
  PublicKey    []byte
  Algorithm    int       // default 0
  Counter      uint32    // default 0; replay protection
  Name         string
  CreatedAt    time.Time
}
```

Table: `passkeys`.

### `refresh.go`

```go
BlockedRefreshToken {
  ID        string
  Token     string    // uniqueIndex; SHA-256 hex of raw token
  UserID    string    // indexed
  ExpiresAt time.Time // indexed; cleanup target
  CreatedAt time.Time
}
```

Table: `blocked_refresh_tokens`.

### `settings.go` — singleton runtime config (ID always 1)

```go
SystemSettings {
  ID                    uint // auto PK, always 1
  RegistrationEnabled   bool // default true
  TokenRegistrationOnly bool // default false

  // Auth
  PasskeysEnabled bool // default true
  GoogleClientID, GoogleClientSecret, GoogleRedirectURL string
  GithubClientID, GithubClientSecret, GithubRedirectURL string
  TwitterClientID, TwitterClientSecret, TwitterRedirectURL string
  JWTSecret, SessionSecret, FrontendURL string
  TokenDuration int // hours; default 24

  // Server
  ServerPort, ServerHost, ServerDomain string
  ServerEnableTLS, ServerUseACME, BehindProxy bool
  ServerCertFile, ServerKeyFile, ServerEmail string

  // Instance
  ServerName        string
  GuestLoginEnabled bool // default true

  // LiveKit
  LiveKitHost, LiveKitAPIKey, LiveKitAPISecret string
  LiveKitExternal bool

  // CORS
  CORSAllowedOrigins, CORSAllowedHeaders, CORSAllowedMethods string
  CORSAllowCredentials bool
  CORSMaxAge           int

  // Chat uploads
  ChatUploadBackend, ChatUploadDiskDir string
  ChatUploadMaxBytes, ChatUploadInlineMax int64
  ChatUploadS3Endpoint, ChatUploadS3Bucket, ChatUploadS3Region string
  ChatUploadS3AccessKey, ChatUploadS3SecretKey, ChatUploadS3PublicURL string

  // Room limits
  MaxParticipantsLimit int // default 1000
  MaxRoomsPerUser      int // default 100

  // Upload quotas
  MaxUploadBytesPerUser    int64 // default 524288000 (500MB)
  GlobalDiskThresholdBytes int64 // default 0

  // Chat retention (client-advisory)
  ChatMaxMessageCount int // default 10000
  ChatMessageTTLHours int // default 2160 (90d)

  // Recordings (shipped)
  RecordingsEnabled        bool // default false
  RecordingMaxDurationMins int  // default 60; 0 = unlimited
  RecordingMaxFileSizeMB   int  // default 2048; 0 = unlimited

  // Email branding + per-template subject/preheader overrides
  EmailInstanceName, EmailSupportEmail, EmailInstanceURL string
  EmailHeaderBg, EmailButtonBg string
  EmailSubjectVerify|Welcome|Reset|Changed|Invite string
  EmailPreheaderVerify|Welcome|Reset|Changed|Invite string

  // SMTP (empty → fall back to config.yaml)
  EmailSMTPHost, EmailUsername, EmailPassword string
  EmailFromAddress, EmailFromName string
  EmailSMTPPort int
  EmailTLSSkipVerify, EmailSMTPSMode bool

  // Logger
  LogLevel string

  UpdatedAt time.Time
}
```

Methods: `IsOAuthProviderConfigured(provider)`, `ConfiguredOAuthProviders()`.
Effective values: DB non-empty overlays `config.yaml` via `SettingsRepository.GetEffectiveSettings()`.

### `invite_token.go`

```go
InviteToken {
  ID        string // varchar(36) PK
  Token     string // unique, varchar(64)
  Email     string // optional pre-bind
  CreatedBy string
  ExpiresAt time.Time
  UsedAt    *time.Time
  UsedBy    string
  CreatedAt time.Time
}
```

### `user_preferences.go`

```go
UserPreferences {
  UserID          string // PK
  PreferencesJSON string // text, default '{}'
  UpdatedAt       time.Time
}
```

Table: `user_preferences`. Single JSON blob — no schema migs for new pref keys.

### `chat_upload.go`

```go
ChatUpload {
  ID             string // PK
  RoomID         string // indexed; FK → rooms.id ON DELETE CASCADE (Postgres)
  UploadedBy     string // indexed; user id
  FileHash       string // SHA-256 hex, varchar(64)
  Extension      string // e.g. ".png"
  FileSize       int64  // default 0
  StorageBackend string // default "disk"; disk|s3|inline etc.
  CreatedAt      time.Time
  Room           Room   // json:"-"; FK
}
```

Table: `chat_uploads`. No dedicated repository — written by storage layer; cascade-deleted with rooms.

### `job.go` — queue job model

```go
Job {
  ID          string    // varchar(36) PK
  Type        string    // indexed
  Payload     string    // JSON text (SQLite + PG)
  RunAt       time.Time // indexed; eligibility
  Priority    int       // indexed; lower = higher priority; default 0
  Status      JobStatus // indexed; default pending
  Attempts    int       // default 0
  MaxAttempts int       // default 3
  LastError   string
  CreatedAt, UpdatedAt time.Time
}
```

| JobStatus | Meaning |
|-----------|---------|
| `pending` | Ready to claim |
| `active`  | Being processed |
| `done`    | Successful |
| `failed`  | Max retries exceeded |

Table: `jobs`. Queue worker/API details → `bedrud-jobs`.

### `verification_event.go`

```go
VerificationEvent {
  ID        uint
  UserID    string // indexed
  Email     string
  EventType VerificationEventType // indexed
  IP        string
  Metadata  string // JSON blob
  CreatedAt time.Time
}
```

Event types: `sent`, `resent`, `success`, `failed`, `admin_force`, `email_change`.
Table: `verification_events`.

### `webhook.go` — outbound webhooks (shipped)

```go
Webhook {
  ID        string     // varchar(36) PK
  Name      string
  URL       string     // varchar(1024)
  Secret    string     // json:"-"; masked after creation via MaskedSecret()
  Events    []string   // gorm serializer:json; type text
  IsActive  bool       // default true
  LastSeen  *time.Time // indexed; last delivery time
  CreatedBy string
  CreatedAt, UpdatedAt time.Time
}
```

Event name constants: `room.created`, `room.ended`, `participant.joined`, `recording.completed`, `ping`.

### `recording.go` — LiveKit Egress recordings (**shipped**)

```go
Recording {
  ID            string
  RoomID        string          // indexed
  RoomName      string
  EgressID      string          // uniqueIndex
  FileURL       string
  FileSize      int64
  RecordingType string          // audio|video|screen|composite
  DurationMs    int64
  Status        RecordingStatus // default pending
  Error         string
  CreatedBy     string
  StartedAt     *time.Time
  CompletedAt   *time.Time
  CreatedAt, UpdatedAt time.Time
}
```

| RecordingStatus | |
|-----------------|---|
| `pending` | Created, egress not started |
| `started` | Egress running |
| `processing` | Ending / finalizing file |
| `completed` | File ready |
| `failed` | Error set |
| `deleting` | Admin/async delete in progress |

Recording types: `composite`, `audio`, `video`, `screen`.
Helpers: `ValidRecordingTypes`, `IsValidRecordingType(t)`.

### `queue_stats.go` — admin queue DTO (not a table)

```go
QueueStats {
  Pending, Active, Done24h, Failed24h, Total, MaxDepth int64
  OldestPending *time.Time
  RecentFailures []FailedJobSummary
  ProcessedPerMin, FailedPerMin, FailRate float64
  PendingEmail, FailedEmail24h int64
  LastSendError string
  LastSendErrorAt *time.Time
}
FailedJobSummary { ID, Type, Error, Attempts, UpdatedAt, Age }
```

Full queue ops → `bedrud-jobs`.

### Model relationships

```
User(1) → (N)Passkey              via UserID
User(1) → (N)BlockedRefreshToken  via UserID
User(1) → (1)UserPreferences      via UserID (PK)
User(1) → (N)RoomParticipant      via UserID
User(1) → (N)RoomPermissions      via UserID
User(1) → (N)VerificationEvent    via UserID
User(1) → (N)ChatUpload           via UploadedBy (no formal FK)
Room(1) → (N)RoomParticipant      via RoomID
Room(1) → (N)RoomPermissions      via RoomID
Room(1) → (N)ChatUpload           via RoomID (CASCADE)
Room(1) → (N)Recording            via RoomID
RoomParticipant ↔ RoomPermissions via (RoomID, UserID)
Webhook standalone; Job standalone; SystemSettings singleton
```

---

## `internal/repository/` — Data Access

Shared: `PaginationParams{Page, Limit}` in `user_repository.go`.

### `user_repository.go` — `UserRepository{*gorm.DB}`

| Fn | Purpose |
|----|---------|
| `CreateOrUpdateUser(user)` | Upsert by `(email, provider)` — FirstOrCreate + Assign |
| `GetUserByEmailAndProvider(email, provider)` | Composite lookup; `nil, nil` if missing |
| `GetUserByEmail(email)` | First match by email |
| `GetUserByID(id)` | PK; `nil, nil` if missing |
| `CreateUser(user)` | Insert |
| `CreateUserWithPasskey(user, passkey)` | TX: user + passkey |
| `UpdateUser(user)` | Full save + timestamp |
| `UpdatePassword(userID, hashed)` | Password + clear refresh + set `password_changed_at` |
| `UpdateRefreshToken(userID, raw)` | Store SHA-256 of raw token |
| `UpdateRefreshTokenAtomic(userID, oldRaw, newRaw)` | CAS rotation by hash; returns ok bool |
| `MatchRefreshToken(userID, raw)` | Compare hash |
| `ClearRefreshToken(userID)` | Invalidate sessions |
| `BlockRefreshToken(userID, raw, expiresAt)` | Insert hashed token into blocklist |
| `IsRefreshTokenBlocked(raw)` | Hash lookup; not-expired only |
| `CleanupBlockedTokens()` | Delete expired blocklist rows |
| `UpdateUserAccesses(userID, accesses)` | Replace role array |
| `UpdateAccessesAndClearToken(userID, accesses)` | Role change + force re-login |
| `UpdateUserStatusAndClearToken(userID, active)` | Ban/unban + clear token |
| `ActivateUser(userID)` | `is_active=true` without clearing token |
| `GetUsersByAccess(access)` | PG `ANY()` / SQLite LIKE on `{a,b}` |
| `GetAllUsers(p PaginationParams)` | Paginated list + total |
| `GetAllUsersFiltered(p *UserFilterParams)` | Admin filters (search, provider, role, status, verified, created, sort) |
| `GetUsersByIDs(ids)` | Batch load |
| `GetInactiveUserIDs()` | Banned/inactive ids |
| `BatchBan(ids)` / `BatchPromote(ids)` / `BatchDeleteSoft(ids)` | Admin batch; per-id error map |
| `GetRecentUsers(limit)` | Newest non-guest users |
| `GetRecentSignupsFiltered(p *RecentSignupsFilterParams)` | Admin signups feed |
| `CountUsers()` / `CountUsersFiltered(excludeProviders)` | Totals |
| `CountUsersSince(t)` / `CountUsersSinceFiltered(t, excludeProviders)` | Growth |
| `CountUsersByDay(days)` | `[]DayCount` |
| `DeleteGuestUsers(cutoff)` | Guests older than cutoff with no active participation |
| `DeleteUnverifiedUsers(cutoff)` | Unverified local/passkey older than cutoff (cascade via DeleteUser) |
| `DeleteUser(userID)` | TX cascade: passkeys → prefs → participants → permissions → blocked tokens → user |

**UserFilterParams:** `Page, Limit, Search, Provider[], Role[], Status[], Verified *bool, Created, Sort, Order`.
**RecentSignupsFilterParams:** `Page, Limit, Search, Provider[], ExcludeGuest, DateFrom, DateTo, Sort, Order`.

### `room_repository.go` — `RoomRepository{*gorm.DB}`

| Fn | Purpose |
|----|---------|
| `CreateRoom(createdBy, name, isPublic, mode, maxParticipants, settings)` | TX: validate/gen name → create → creator as approved participant + admin perms; 24h expiry; sets LastActivityAt |
| `GetRoom(id)` / `GetRoomByName(name)` | Lookup (name case-normalized) |
| `AddParticipant(roomID, userID)` | Insert or reactivate; reject banned |
| `AddParticipantWithCapacityCheck(roomID, userID, max)` | Join with capacity guard |
| `RemoveParticipant` / `RemoveAllParticipants` | Mark inactive + left_at |
| `GetActiveParticipants` / `GetRoomParticipantsWithUsers` | Active list; latter Preload User |
| `KickParticipant` | Inactive + banned |
| `BringToStage` / `RemoveFromStage` / `IsParticipantOnStage` | Stage flags |
| `IsRoomModerator` / `SetRoomModerator` | Moderator flag |
| `IsParticipantBanned` / `IsParticipant` / `GetParticipantCount` | Checks |
| `UpdateParticipantPermissions` / `GetParticipantPermissions` | Permission row |
| `UpdateParticipantStatus(roomID, userID, updates)` | Generic map update |
| `UpdateRoomSettings(roomID, settings)` | All 7 embedded settings columns (incl. recordings_allowed) |
| `UpdateRoom(room)` | Full save |
| `DeleteRoom(roomID, userID)` | TX hard-delete if creator; cascade perms/participants/chat_uploads |
| `HardDeleteRoom(roomID)` | Same cascade, no owner check (caller authorizes) |
| `SoftDeleteRoom(roomID)` | `deleted_at` + `is_active=false`; keeps recordings |
| `GetArchivedRoomsByUserPaginated(userID, page, limit)` | Soft-deleted rooms by owner |
| `FindArchivedRoomsNoRecordings()` | Archived rooms with 0 recording rows (purge candidates) |
| `GetAllRooms` / `GetAllActiveRooms` / `GetAllActiveRoomsWithLimit` | Lists |
| `GetAllRoomsPaginated(p)` / `GetAllRoomsFiltered(p *RoomFilterParams)` | Admin list + total |
| `EnrichAdminRoomDetails(rooms)` | → `[]AdminRoomDetail` (counts, owner, last activity) |
| `GetRoomsCreatedByUser` / `GetLatestRoomsCreatedByUser` | Owner rooms |
| `GetRoomsParticipatedInByUser` | Joined rooms |
| `GetUserParticipationsPaginated(userID, p)` | Participation history + Room preload |
| `GetRoomsByIDs` / `BatchSuspendRooms` / `BatchHardDeleteRooms` | Batch admin |
| `SetRoomIdle` / `DeactivateRoomParticipants` | Idle cleanup helpers |
| `CleanupExpiredRooms()` | Bulk mark expired inactive; skips persistent |
| `GetUserByID` | User fetch for room flows |
| Counts: `CountRooms`, `CountActiveRooms`, `CountPublicRooms`, `CountPrivateRooms`, `CountPersistentRooms`, `CountEmptyRooms`, `CountStaleRooms(hours)`, `CountRoomsSince`, `CountActiveParticipants`, `CountActiveRoomsByUser`, `CountActiveRoomsWithParticipantCount`, `AvgParticipantsPerRoom` | Admin KPIs |
| Trends: `CountRoomsByDay`, `CountActiveRoomsByDay`, `CountActiveParticipantsByDay` | `[]DayCount` |
| `GetRecentRoomEvents(limit)` / `GetRoomEventsFiltered(p)` | Activity feed |

**RoomFilterParams:** search, visibility, status (`active`/`suspended`/`archived`), occupancy, capacity (legacy), owner, date ranges, sort/order.
**AdminRoomDetail:** room fields + `ParticipantsCount`, `LastActivityAt`, `OwnerName`, `OwnerEmail`, `DeletedAt`.
**RoomEventsFilterParams:** types (`room_created`, `room_joined`), date range, search, order.

### `passkey_repository.go` — `PasskeyRepository`

`CreatePasskey`, `GetPasskeyByCredentialID` (`nil,nil` if missing), `GetPasskeysByUserID`, `UpdatePasskeyCounter` (only if counter advances — clone detection), `DeletePasskey(id, userID)`, `DeleteByUserID`.

### `settings_repository.go` — `SettingsRepository{db, cfg}`

| Fn | Purpose |
|----|---------|
| `SetConfig(cfg)` | Attach config.yaml for merge |
| `GetSettings()` | FirstOrCreate ID=1; defaults Registration/Guest/Passkeys enabled, TokenDuration=24 |
| `SaveSettings(s)` | Force ID=1, upsert |
| `GetEffectiveSettings()` | DB overlaid with non-empty config.yaml values |

### `invite_token_repository.go` — `InviteTokenRepository`

`Create(t)`, `List(p PaginationParams)` → tokens + total (newest first), `GetByToken` (`nil,nil` if missing), `MarkUsed(tokenID, userID)` (fails if used/expired), `Delete(tokenID)`.

### `verification_event_repository.go` — `VerificationEventRepository`

`RecordEvent(userID, email, eventType, ip, metadata)`, `GetRecentEvents(limit)`, `GetEventsByUser(userID, limit)`.

### `user_preferences_repository.go` — `UserPreferencesRepository`

`GetByUserID` (`nil,nil` if missing), `Upsert(userID, prefsJSON)` (`ON CONFLICT UPDATE ALL`), `DeleteByUserID`.

### `webhook_repository.go` — `WebhookRepository`

| Fn | Purpose |
|----|---------|
| `Create(w)` | Insert |
| `GetByID(id)` | PK; `ErrWebhookNotFound` |
| `List()` | All, newest first |
| `ListPaginated(p)` | Page/limit + total |
| `Update(w)` | Full save |
| `Delete(id)` | `ErrWebhookNotFound` if none |
| `UpdateLastSeen(id, t)` | Delivery timestamp |
| `ListActive(event)` | Active + subscribed (filter in Go) |
| `UpdateSecret(id, secret)` | Rotate HMAC secret |

### `recording_repository.go` — `RecordingRepository` (**shipped**)

Errors: `ErrRecordingNotFound`, `ErrOptimisticLock`.

| Fn | Purpose |
|----|---------|
| `Create(rec)` | Validate room/creator/type; insert |
| `GetByID` / `GetByEgressID` | Lookup; not found → `ErrRecordingNotFound` |
| `GetActiveByRoom(roomID)` | pending\|started\|processing; `nil,nil` if none |
| `HasActiveRecording(roomID)` | Bool |
| `CountByRoom` / `CountByRoomAndCreator` | Counts |
| `ListByRoomID(roomID, offset, limit)` | Paginated + total |
| `ListByRoomAndCreator(...)` | Filtered list + total |
| `ListAdmin(offset, limit, roomID, status, after, before)` | Admin filters |
| `GetRecordingsByIDs(ids)` | Batch |
| `UpdateEgressID(id, egressID, status)` | Only from `pending`; sets started_at |
| `UpdateStatus(id, from, to)` | Optimistic status transition |
| `UpdateStartedAt(id, t)` | Timestamp |
| `UpdateError(id, errMsg)` | → failed if pending/started/processing |
| `UpdateCompleted(id, fileURL, fileSize, durationMs, completedAt)` | From `processing` only |
| `MarkDeleting(id)` | Status → deleting (no optimistic lock) |
| `DeleteRecording(id)` | Hard delete row |
| `DeleteByRoom(roomID)` | All rows for room |
| `DeleteStaleRecordings(cutoff)` | failed\|pending\|started older than cutoff |
| `FindExpiredOnArchivedRooms(cutoff)` | Completed/failed on archived rooms past retention |

---

## `internal/database/` — DB Layer

### `database.go`

| Export | Purpose |
|--------|---------|
| `Initialize(cfg *config.DatabaseConfig)` | Postgres or SQLite connection |
| `GetDB()` | Global `*gorm.DB` |
| `Close()` | Close underlying `*sql.DB` |
| `SetForTest(db)` / `ResetForTest()` | Test-only global inject |
| `DBTypePostgres` / `DBTypeSQLite` | Dialector name constants |

Behavior:
- GORM log level mapped from zerolog.
- `DisableForeignKeyConstraintWhenMigrating: true` (FKs added manually on Postgres).
- SQLite: `PRAGMA foreign_keys=ON`, WAL, `MaxOpenConns(1)`.
- Postgres: pool from config (`MaxIdleConns`, `MaxOpenConns`, `MaxLifetime`).

### `migrations.go` — `RunMigrations()`

Skip with `BEDRUD_SKIP_MIGRATE=1`.

**AutoMigrate order:** User → BlockedRefreshToken → Room → (name index fix + partial unique `idx_rooms_active_name`) → RoomParticipant → RoomPermissions → Passkey → SystemSettings → InviteToken → UserPreferences → ChatUpload → Job → VerificationEvent → Webhook → Recording.

**Room name index migration:** Drops legacy unique `idx_rooms_name` if unique; recreates non-unique. Active uniqueness via:

```sql
CREATE UNIQUE INDEX idx_rooms_active_name ON rooms(name) WHERE is_active = true  -- PG
-- SQLite: WHERE is_active = 1
```

**Postgres FKs (idempotent):**

| Constraint | |
|------------|---|
| `fk_room_permissions_participant` | (room_id,user_id) → room_participants CASCADE |
| `fk_chat_uploads_room` | room_id → rooms CASCADE |
| `fk_room_participants_room` | room_id → rooms CASCADE |
| `fk_room_participants_user` | user_id → users CASCADE |
| `fk_rooms_created_by` / `fk_rooms_admin_id` | → users SET NULL |
| `fk_passkeys_user` | → users CASCADE |
| `fk_blocked_tokens_user` | → users CASCADE |
| `fk_invite_tokens_created_by` | → users SET NULL |

No `server/migrations/` SQL directory.

---

## `internal/testutil/`

### `db.go`

| Export | Purpose |
|--------|---------|
| `SetupTestDB(t *testing.T) *gorm.DB` | In-memory SQLite, AutoMigrate, MaxOpenConns(1), `database.SetForTest` |

Migrates: User, BlockedRefreshToken, Room, RoomParticipant, RoomPermissions, Passkey, SystemSettings, InviteToken, UserPreferences, ChatUpload, Job, Webhook, Recording.
**Note:** Does **not** AutoMigrate `VerificationEvent` (add if tests need it).

No `TeardownTestDB` — use `t.Cleanup` + `database.ResetForTest()` if needed.

### `livekit_mock.go`

| Type | Purpose |
|------|---------|
| `MockRoomService` | `livekit.RoomService` mock; hooks + `atomic.Int64` call counts |
| `MockEgress` | `livekit.Egress` mock for recording tests; default active egress id |
| `NewMockRoomService()` / `NewMockEgress()` | Constructors |

---

## `internal/models/stats.go` — Admin overview DTOs

| Type | Fields (summary) |
|------|------------------|
| `OverviewResponse` | Health, KPIs, ActivityTrend, RoomComposition, NeedsAttention, RecentSignups, RecentEvents (`json:"recentRoomEvents"`), InstanceInfo |
| `OverviewHealth` | Status, TLS, Realtime, AlertsCount, UptimeSeconds, DBStatus |
| `TLSStatus` | Enabled, DaysRemaining, ExpiryDate, Status |
| `KpiEntry` | Value, Delta, DeltaLabel, DeltaPercent, PeakToday, ActiveNow |
| `OverviewKPIs` | TotalUsers, OnlineNow, TotalRooms, ActiveSessions, PendingActions |
| `RoomComposition` | Live, Public, Private, Persistent, Stale |
| `DayActivity` | Date, RoomsCreated, RoomsActive, Participants |
| `AttentionItem` | Type, Severity, Message, DaysLeft, RoomID |
| `RoomEvent` | Type (`room_created`/`room_joined`), RoomID, RoomName, UserID, UserName, Timestamp |
| `RecentUser` | ID, Name, Email, Provider, CreatedAt (string) |
| `InstanceInfo` | Name, Version, UptimeSeconds, StartedAt |
| `DayCount` | Date, Count |

---

## Gotchas for agents

1. **Refresh tokens are hashed** (SHA-256 hex) in DB and blocklist — never store raw tokens.
2. **Room soft-delete:** `SoftDeleteRoom` sets `DeletedAt`; hard delete via `HardDeleteRoom` / `DeleteRoom`. There is no `AdminDeleteRoom`.
3. **Room name not globally unique** — only among `is_active=true` rooms.
4. **Recording is shipped** — model, repo, settings flags, webhook event, MockEgress all exist.
5. **SystemSettings is the runtime config store** — far more than registration flags; use `GetEffectiveSettings` for runtime values.
6. **No chat_upload repository** — model only; storage + room cascade.
7. **Webhook.Events is `[]string` + JSON serializer**, not `StringArray`.
8. **Test DB skips VerificationEvent** migration unless tests add it.
