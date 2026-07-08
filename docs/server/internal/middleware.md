# Middleware (`internal/middleware`)

Custom Fiber middleware for authentication, authorization, rate limiting, and feature gating.

---

## Auth (`auth.go`)

### `Protected() fiber.Handler`

Extracts JWT from:

1. `Authorization: Bearer <token>` header (case-insensitive `bearer` prefix)
2. Fallback: `access_token` HttpOnly cookie

Validates via `auth.ValidateToken()`. Checks in-memory ban set via `auth.IsUserBanned(userID)` — returns 403 "Account is deactivated" for banned users.

Stores `*auth.Claims` in `c.Locals("user")`. Returns 401 on missing/invalid/revoked token.

### `RequireAccess(requiredAccess models.AccessLevel) fiber.Handler`

**Hierarchical** access check (not exact match):

| Level | Weight |
|-------|--------|
| `superadmin` | 4 |
| `admin` | 3 |
| `moderator` | 2 |
| `user` | 1 |

A user passes if any access in `claims.Accesses` has weight ≥ required weight. Example: `superadmin` passes `RequireAccess(admin)`.

Must be chained after `Protected()`. Returns 403 on insufficient rights.

### `RequireEmailVerified(cfg, userRepo) fiber.Handler`

When `auth.requireEmailVerification` is true:

- Blocks unverified local/passkey users with 403 `"Email not verified"`
- **Always checks DB** (`user.EmailVerifiedAt`) — does not trust JWT claim alone (prevents stale-token bypass if admin un-verifies)
- Guest users (`provider == "guest"`) are exempt
- No-op when verification is disabled in config

Used on most protected routes after `Protected()`.

### `RequireBearerForMutations() fiber.Handler`

Rejects `POST`/`PUT`/`DELETE`/`PATCH` requests that rely on cookie auth only (no `Authorization: Bearer` header). Prevents CSRF on state-changing endpoints.

### `RejectGuest() fiber.Handler`

Blocks guest users (`claims.Provider == "guest"`) with 403. Use for profile/password/account endpoints requiring persistent identity.

---

## Ban set (coordinated with `auth/jwt.go`)

On startup, `server.go` loads inactive user IDs via `userRepo.GetInactiveUserIDs()` into `auth.LoadBannedUsersFromDB()`.

Runtime updates via `auth.BanUser(userID)` / `auth.UnbanUser(userID)` when admin enables/disables users.

---

## Rate Limiting (`ratelimit.go`)

IP-based sliding window. Keys on client IP — **requires correct proxy config** when behind nginx/Cloudflare (`behindProxy: true` or `trustedProxies`).

| Middleware | Default | Config field | Env |
|-----------|---------|--------------|-----|
| `AuthRateLimiter(cfg)` | 10/min | `authMaxRequests` | `RATELIMIT_AUTH_MAX` |
| `ResendRateLimiter(cfg)` | stricter | `authResendMaxRequests` | `RATELIMIT_AUTH_RESEND_MAX` |
| `GuestRateLimiter(cfg)` | 5/min | `guestMaxRequests` | `RATELIMIT_GUEST_MAX` |
| `APIRateLimiter(cfg)` | configurable | `apiMaxRequests` | `RATELIMIT_API_MAX` |

Set limit field to `0` to disable that limiter. Returns 429 when exceeded.

`server.go` warns at startup if rate limiting is active but `behindProxy=false` and no `trustedProxies` — all clients behind a proxy would share one bucket.

---

## Recordings Gate (`recordings_enabled.go`)

### `RecordingsEnabled() fiber.Handler`

Checks `SystemSettings.RecordingsEnabled`. Returns 403 if disabled.

Prepared for recording routes (currently not registered — see [Planned Features](../planned-features.md)).

---

## Usage examples

```go
// Standard protected route with email verification
api.Get("/auth/me",
    middleware.Protected(),
    middleware.RequireEmailVerified(cfg, userRepo),
    authHandler.GetMe,
)

// Admin route
adminGroup := api.Group("/admin",
    middleware.Protected(),
    middleware.RequireEmailVerified(cfg, userRepo),
    middleware.RequireAccess(models.AccessSuperAdmin),
)

// Auth endpoint with rate limit
api.Post("/auth/login",
    middleware.AuthRateLimiter(cfg.RateLimit),
    authHandler.Login,
)
```