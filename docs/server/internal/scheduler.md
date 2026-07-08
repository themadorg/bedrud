# Background Scheduler (`internal/scheduler`)

Uses [gocron](https://github.com/go-co-op/gocron) for periodic background tasks.

---

## Scheduled Tasks

| Task | Schedule | Function |
|------|----------|----------|
| Expired room cleanup | Every 1 min | `roomRepo.CleanupExpiredRooms()` |
| Idle room detection | Every 1 min | `checkIdleRooms` — 0 LK participants + >5 min old |
| Stale guest user delete | Weekly Mon 03:00 | Guests older than 7 days, no active rooms |
| Unverified account delete | Daily 03:30 | `DeleteUnverifiedUsers` (TTL from config, default 48h) |
| Blocked refresh token cleanup | Every 1 hour | `userRepo.CleanupBlockedTokens()` |
| Revoked access token prune | Every 1 hour | `auth.PruneRevokedTokens()` |
| Queue job cleanup | Daily 03:00 | Done jobs >7d, failed jobs >30d |
| Stale recording cleanup | Daily 03:00 | Failed/pending recordings >7d |
| TLS cert expiry check | Daily 09:00 | Auto-renew if ≤30 days remaining |
| Recording retention | Every N hours | Only when `recStore` wired (planned) |

---

## Idle Room Detection

`checkIdleRooms(roomRepo, cfg, client)`:

1. List active DB rooms
2. Query LiveKit participant counts per room
3. Mark idle if 0 participants and room older than 5 minutes
4. Skips rooms with `IsPersistent = true`
5. Logs error on failure, info on success

---

## Job Cleanup

Daily at 03:00:

| Function | Retention |
|----------|-----------|
| `CleanupJobs(db, 7d)` | Delete `done` jobs older than 7 days |
| `CleanupFailedJobs(db, 30d)` | Delete `failed` jobs older than 30 days |

---

## Initialize signature

```go
func Initialize(
    db *gorm.DB,
    roomRepo *repository.RoomRepository,
    userRepo *repository.UserRepository,
    recordingRepo *repository.RecordingRepository,
    lkCfg *config.LiveKitConfig,
    serverCfg *config.ServerConfig,
    recStore storage.RecordingStore,  // nil in current bootstrap
    recCfg *config.RecordingConfig,   // nil in current bootstrap
)
```

| Function | Purpose |
|----------|---------|
| `Initialize(...)` | Register all cron jobs + `scheduler.StartAsync()` |
| `Stop()` | Graceful scheduler shutdown |

### TLS auto-renewal

When manual TLS (non-ACME), scheduler checks cert expiry daily at 09:00. Renews via `utils.RenewSelfSignedCert` when ≤ `CertWarnDays` (30) days remain. SANs built from domain, server host IP, outbound IP, localhost.