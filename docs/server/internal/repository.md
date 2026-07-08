# Repositories (`internal/repository`)

Data access layer. Handlers call repositories instead of writing GORM queries directly.

---

## UserRepository

```go
type UserRepository struct { *gorm.DB }
```

| Method | Purpose |
|--------|---------|
| `CreateOrUpdateUser(user)` | Upsert by `(email, provider)` |
| `GetUserByEmailAndProvider(email, provider)` | Composite lookup. `nil, nil` if not found |
| `GetUserByEmail(email)` | Lookup by email |
| `GetUserByID(id)` | PK lookup |
| `CreateUser(user)` | Straight insert |
| `UpdateUser(user)` | Full save |
| `UpdateRefreshToken(userID, token)` | Update refresh token field |
| `BlockRefreshToken(userID, token, expiresAt)` | Insert blocked token |
| `IsRefreshTokenBlocked(token)` | Check revocation |
| `CleanupBlockedTokens()` | Delete expired blocked tokens |
| `UpdateUserAccesses(userID, accesses)` | Replace role array |
| `GetUsersByAccess(access)` | Find by role (PG `ANY()` for text[]) |
| `GetAllUsers()` | Return all users |
| `DeleteUser(userID)` | Transactional cascade: passkeys → prefs → participants → permissions → blocked tokens → user |
| `GetInactiveUserIDs()` | IDs of `IsActive=false` users (ban set preload) |
| `DeleteGuestUsers(cutoff)` | Delete stale guest accounts older than cutoff |
| `DeleteUnverifiedUsers(cutoff)` | Delete local/passkey users with `EmailVerifiedAt=nil` older than cutoff |

---

## RoomRepository

```go
type RoomRepository struct { *gorm.DB }
```

| Method | Purpose |
|--------|---------|
| `CreateRoom(createdBy, name, isPublic, mode, settings)` | TX: validate/gen name → create room → add creator as participant + admin perms. 24h expiry |
| `GetRoom(id)` / `GetRoomByName(name)` | Lookup by ID or name (case-insensitive) |
| `AddParticipant(roomID, userID)` | Insert or reactivate. Reject banned |
| `RemoveParticipant(roomID, userID)` | Mark inactive, set `left_at` |
| `GetActiveParticipants(roomID)` | Currently active participants |
| `GetRoomParticipantsWithUsers(roomID)` | Same + `Preload("User")` |
| `KickParticipant(roomID, userID)` | Mark inactive + banned |
| `BringToStage` / `RemoveFromStage` | Toggle `is_on_stage` |
| `UpdateParticipantPermissions(roomID, userID, perms)` | Write permission row |
| `GetParticipantPermissions(roomID, userID)` | Read permission row |
| `UpdateParticipantStatus(roomID, userID, updates)` | Generic map-based update |
| `UpdateRoomSettings(roomID, settings)` | Atomic map-based update (merge-safe) |
| `UpdateRoom(room)` | Full save |
| `DeleteRoom(roomID, userID)` | TX cascade. Checks `created_by` |
| `AdminDeleteRoom(roomID)` | Same, no owner check. Deletes `chat_uploads` in TX |
| `GetAllRooms()` / `GetAllActiveRooms()` | List rooms |
| `GetAllRoomsPaginated(params)` | Paginated room list (CLI) |
| `GetRoomsCreatedByUser(userID)` | User's created rooms |
| `GetRoomsParticipatedInByUser(userID)` | Rooms user joined |
| `SetRoomIdle(roomID)` | Mark inactive |
| `CleanupExpiredRooms()` | Bulk mark expired inactive. Excludes persistent rooms |
| `CountActiveParticipants()` | Distinct count across all rooms |

---

## PasskeyRepository

| Method | Purpose |
|--------|---------|
| `CreatePasskey` | Insert passkey |
| `GetPasskeyByCredentialID` | Lookup by credential |
| `GetPasskeysByUserID` | List user's passkeys |
| `UpdatePasskeyCounter` | Replay protection counter |
| `DeletePasskey` | Delete single passkey |
| `DeleteByUserID(userID)` | Delete all user passkeys |

---

## SettingsRepository

| Method | Purpose |
|--------|---------|
| `GetSettings()` | `FirstOrCreate` ID=1, default `RegistrationEnabled=true` |
| `SaveSettings(s)` | Force ID=1, upsert |

---

## InviteTokenRepository

| Method | Purpose |
|--------|---------|
| `Create(t)` | Insert token |
| `List()` | Newest first |
| `GetByToken(token)` | Lookup by token string |
| `MarkUsed(tokenID, userID)` | Set `usedAt` + `usedBy` |
| `Delete(tokenID)` | Delete by ID |

---

## UserPreferencesRepository

| Method | Purpose |
|--------|---------|
| `GetByUserID(userID)` | `nil, nil` if not found |
| `Upsert(userID, prefsJSON)` | `ON CONFLICT ... UPDATE ALL` |
| `DeleteByUserID(userID)` | Delete preferences row |

---

## RecordingRepository

| Method | Purpose |
|--------|---------|
| `Create(recording)` | Insert recording |
| `GetByID(id)` | PK lookup |
| `ListByRoomID(roomID)` | Room's recordings |
| `UpdateStatus(id, status)` | Update recording status |
| `Delete(id)` | Delete recording row |
| `ListExpired(before)` | For retention cleanup |

---

## WebhookRepository

| Method | Purpose |
|--------|---------|
| `Create(webhook)` | Insert webhook endpoint |
| `List()` | All webhooks |
| `GetByID(id)` | PK lookup |
| `Update(webhook)` | Full save |
| `Delete(id)` | Delete webhook |
| `CreateDelivery(delivery)` | Log delivery attempt |

---

## VerificationEventRepository

Tracks verification and password reset events for cooldown enforcement and audit.