---
name: bedrud-data
description: Data layer — GORM models, repository, database init, test utilities, admin DTOs.
license: Apache License
---

# Bedrud Data Layer

Go module `bedrud`. Root: `server/`. GORM ORM + SQLite/Postgres.

---

## `internal/models/` — GORM Models

### `user.go`

```go
User {
  ID              string     // varchar36 PK
  Email           string     // unique
  Name            string
  Provider        string     // "google", "github", "twitter", "local", "passkey", "guest"
  AvatarURL       string
  Password        string     // json:"-", bcrypt hash
  RefreshToken    string     // json:"-"
  Accesses        StringArray // []string, PG text[] via "{val1,val2}" format
  IsActive        bool       // default true
  EmailVerifiedAt *time.Time // nullable, indexed
  CreatedAt       time.Time
  UpdatedAt       time.Time
}
```

Provider constants: `ProviderLocal`, `ProviderPasskey`, `ProviderGuest`.
`AccessLevel` enum: `superadmin`, `admin`, `moderator`, `user`, `guest`.
`StringArray` custom type: `sql.Scanner` + `driver.Valuer` + `GormDataType() string`.
Methods: `HasAccess(level)`, `IsAdmin()` (checks `admin` in Accesses).

### `room.go`

```go
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
  RoomID         string
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

### `refresh.go`

```go
BlockedRefreshToken {
  ID        string
  Token     string     // unique
  UserID    string     // indexed
  ExpiresAt time.Time  // indexed, for cleanup
  CreatedAt time.Time
}
```

### `settings.go`

```go
SystemSettings {
  ID                   uint   // auto PK, always 1 (singleton)
  RegistrationEnabled  bool   // default true
  TokenRegistrationOnly bool  // default false
  UpdatedAt            time.Time
}
```

### `invite_token.go`

```go
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

```go
UserPreferences {
  UserID         string     // PK
  PreferencesJSON string   // text, default '{}'
  UpdatedAt      time.Time
}
```

### `chat_upload.go`

```go
ChatUpload {
  ID        string     // PK
  RoomID    string     // FK → rooms.id, ON DELETE CASCADE (Postgres)
  FileHash  string     // SHA-256 hex of file content
  Extension string     // file extension with dot (e.g. ".png")
  CreatedAt time.Time
}
```

Only disk-backend uploads tracked. S3/inline skip tracking.

### `job.go` — Queue job GORM model

`Job` model: `ID`, `Type`, `Payload(JSON text)`, `Priority(int)`, `Status(string)`, `Attempts(int)`, `MaxAttempts(int)`, `RunAt(time.Time)`, `LastError(string)`, `CreatedAt`, `UpdatedAt`.

| Status | Meaning |
|--------|---------|
| `pending` | Ready to claim |
| `active` | Being processed |
| `done` | Successful |
| `failed` | Max retries exceeded |

### `verification_event.go`

```go
VerificationEvent {
  ID        uint
  UserID    string
  Email     string
  EventType string    // sent, resent, success, failed, admin_force, email_change
  IP        string
  Metadata  string    // JSON text
  CreatedAt time.Time
}
```

### `webhook.go`

```go
Webhook {
  ID        string
  URL       string
  Events    StringArray  // e.g. {"room.ended","recording.completed"}
  Secret    string
  IsActive  bool
  LastSeenAt *time.Time
  LastStatus int
  CreatedAt time.Time
  UpdatedAt time.Time
}
```

> **TODO oncoming feature:** Recording functionality is planned for a future release.

### `recording.go`

```go
Recording {
  ID            string
  RoomID        string
  RoomName      string
  EgressID      string
  Status        RecordingStatus  // pending, started, processing, completed, failed
  RecordingType string
  FileURL       string
  FileSize      int64
  DurationMs    int64
  CreatedBy     string
  CreatedAt     time.Time
  UpdatedAt     time.Time
}

RecordingStatus constants: RecordingPending, RecordingStarted, RecordingProcessing, RecordingCompleted, RecordingFailed
```

### Model Relationships

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
| `GetRecentUsers(limit)` | Most recently created users (excluding guests) |
| `CountUsers()` | Total user count |
| `CountUsersFiltered(excludeProviders)` | User count excluding certain providers |
| `CountUsersSinceFiltered(since, excludeProviders)` | New users since date, excluding providers |
| `DeleteGuestUsers(cutoff)` | Bulk delete guest users older than cutoff |
| `DeleteUnverifiedUsers(cutoff)` | Bulk delete unverified local/passkey users older than cutoff |
| `DeleteUser(userID)` | Transactional cascade: passkeys → preferences → participants → permissions → blocked tokens → user |

### `room_repository.go` — `RoomRepository{*gorm.DB}`

| Fn | Purpose |
|----|---------|
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
| `CleanupExpiredRooms()` | Bulk mark expired inactive. Excludes persistent rooms |
| `GetUserByID(userID)` | Fetch user (for participant lookups) |
| `CountActiveParticipants()` | Distinct count across all rooms |
| `CountRooms()` | Total room count |
| `CountActiveRooms()` | Active room count |
| `CountPublicRooms()` | Public room count |
| `CountPrivateRooms()` | Private room count |
| `CountPersistentRooms()` | Persistent room count |
| `CountStaleRooms(hours)` | Rooms with no activity in N hours |
| `CountRoomsSince(t)` | Rooms created since time |
| `CountRoomsByDay(days)` | Per-day room creation counts for last N days |
| `CountActiveParticipantsByDay(days)` | Per-day active participant counts |
| `CountActiveRoomsByDay(days)` | Per-day active room counts |
| `RemoveAllParticipants(roomID)` | Mark all room participants inactive |
| `DeactivateRoomParticipants(roomID)` | Set all participants to inactive |
| `GetAllActiveRoomsWithLimit(limit)` | Active rooms with cap (for idle check) |
| `IsRoomModerator(roomID, userID)` | Check if user has moderator role in room |
| `GetRecentRoomEvents(limit)` | Recent room activity events |

### `passkey_repository.go` — `PasskeyRepository{*gorm.DB}`

`CreatePasskey`, `GetPasskeyByCredentialID`, `GetPasskeysByUserID`, `UpdatePasskeyCounter`, `DeletePasskey`, `DeleteByUserID(userID)`.

### `settings_repository.go` — `SettingsRepository{*gorm.DB}`

`GetSettings()` — FirstOrCreate ID=1, default RegistrationEnabled=true.
`SaveSettings(s)` — Force ID=1, upsert.

### `invite_token_repository.go` — `InviteTokenRepository{*gorm.DB}`

`Create(t)`, `List()` (newest first), `GetByToken(token)`, `MarkUsed(tokenID, userID)`, `Delete(tokenID)`.

### `verification_event_repository.go` — `VerificationEventRepository{*gorm.DB}`

| Fn | Purpose |
|----|---------|
| `RecordEvent(userID, email, eventType, ip, metadata)` | Insert audit trail for email verification events |
| `GetRecentEvents(limit)` | Recent verification activity |
| `GetEventsByUser(userID, limit)` | All events for a specific user |

### `user_preferences_repository.go` — `UserPreferencesRepository{*gorm.DB}`

`GetByUserID(userID)` — `nil, nil` if not found.
`Upsert(userID, prefsJSON)` — `ON CONFLICT ... UPDATE ALL`.
`DeleteByUserID(userID)` — Delete preferences row.

### `webhook_repository.go` — `WebhookRepository{*gorm.DB}`

| Fn | Purpose |
|----|---------|
| `Create(webhook)` | Insert new webhook |
| `GetByID(id)` | PK lookup. Returns `ErrWebhookNotFound` |
| `List()` | All webhooks, newest first |
| `Update(webhook)` | Full save |
| `Delete(id)` | Delete by ID |
| `UpdateLastSeen(id, time.Time)` | Update last delivery timestamp + status code |
| `ListActive(event)` | Active webhooks subscribed to a specific event |
| `RotateSecret(id, secret)` | Replace HMAC secret |

> **TODO oncoming feature:** Recording functionality is planned for a future release.

### `recording_repository.go` — `RecordingRepository{*gorm.DB}`

| Fn | Purpose |
|----|---------|
| `Create(rec)` | Insert pending recording |
| `GetByID(id)` | PK lookup. Returns `ErrRecordingNotFound` |
| `GetByEgressID(egressID)` | Lookup by LiveKit egress ID. Returns `ErrRecordingNotFound` |
| `GetActiveByRoom(roomID)` | Current active recording for a room |
| `HasActiveRecording(roomID)` | Boolean check |
| `ListByRoomID(roomID, offset, limit)` | Paginated by room |
| `UpdateEgressID(id, egressID, status)` | Optimistic lock: only from pending |
| `UpdateStatus(id, fromStatus, toStatus)` | Transition status |
| `UpdateError(id, errorMsg)` | Set failed with error message |
| `UpdateCompleted(id, fileURL, durationMs)` | Set completed metadata |
| `DeleteByRoom(roomID)` | Delete all recordings for a room |
| `DeleteRecording(id)` | Delete single recording |
| `DeleteStaleRecordings(cutoff)` | Delete failed/pending recordings older than cutoff |

---

## `internal/database/` — DB Layer

### `database.go`

`Initialize(cfg)` — GORM connection. PostgreSQL (connection pooling) or SQLite.
`GetDB()` — Singleton `*gorm.DB`.
`Close()` — Close underlying `*sql.DB`.

### `migrations.go`

`RunMigrations()` — AutoMigrate: User, BlockedRefreshToken, Room, RoomParticipant, RoomPermissions, Passkey, SystemSettings, InviteToken, UserPreferences, ChatUpload, VerificationEvent, Job.
Raw SQL FK constraints: `fk_room_permissions_participant`, `fk_chat_uploads_room` (Postgres: `ON DELETE CASCADE`).

---

## `internal/testutil/db.go`

| Export | Purpose |
|--------|---------|
| `SetupTestDB()` | Create in-memory SQLite DB, run migrations, return `*gorm.DB` and cleanup fn |
| `TeardownTestDB(db)` | Close and clean up test DB |

---

## `internal/models/stats.go` — Admin Overview DTOs

`OverviewResponse{Health, KPIs, ActivityTrend, RoomComposition, NeedsAttention, RecentSignups, RecentRoomEvents, InstanceInfo}`
`OverviewHealth{Status, TLS(*TLSStatus), Realtime, AlertsCount, UptimeSeconds, DBStatus}`
`TLSStatus{Enabled, DaysRemaining, ExpiryDate, Status}`
`OverviewKPIs{TotalUsers, OnlineNow, TotalRooms, ActiveSessions, PendingActions}` (each `KpiEntry{Value, Delta, DeltaLabel, DeltaPercent, ActiveNow}`)
`RoomComposition{Live, Public, Private, Persistent, Stale}`
`DayActivity{Date, RoomsCreated, RoomsActive, Participants}`
`AttentionItem{Type, Severity, Message, DaysLeft, RoomID}`
`RoomEvent{Type, RoomID, RoomName, UserID, UserName, Timestamp}`
`RecentUser{ID, Name, Email, Provider, CreatedAt}`
`InstanceInfo{Name, Version, UptimeSeconds, StartedAt}`
`DayCount{Date, Count}`
`QueueStats` — see `bedrud-jobs` for full shape.
