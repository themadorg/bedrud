---
name: bedrud-auth
description: Auth service + middleware — JWT, passkeys, OAuth, session store, rate limiting.
license: Apache License
---

# Bedrud Auth Subsystem

Go module `bedrud`. `server/internal/auth/` + `server/internal/middleware/`.

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

`Claims` struct: `UserID`, `Email`, `Name`, `Provider`, `Accesses []string`, `EmailVerifiedAt *time.Time` + `jwt.RegisteredClaims`.

`IsUserBanned(userID string) bool` — in-memory set of deactivated user IDs.
`PruneRevokedTokens()` — periodic cleanup of expired entries from revocation set.

### `session_store.go`

`InitializeSessionStore(secret, secure)` — Create gorilla CookieStore for Goth. Set HttpOnly/Secure/SameSite from TLS mode.
`SetProviderToSession(c *fiber.Ctx, provider)` — Bridge Fiber → http.Request for Goth session.

### `challenge_store.go`

In-memory WebAuthn challenge store. TTL configurable via `PasskeyChallengeTTL`.

### `email.go`

Email canonicalization: lowercase, trim whitespace.

---

## `internal/middleware/auth.go` — JWT Auth Middleware

5 middleware functions:

| Fn | Behavior | Status on Fail |
|----|----------|----------------|
| `Protected()` | Extract JWT from `Authorization: Bearer` header, fallback `access_token` cookie. Check banned set. Store `*auth.Claims` in `c.Locals("user")` | 401 |
| `RequireAccess(level)` | Check `claims.Accesses` contains level (hierarchical: superadmin passes admin/user) | 403 |
| `RequireBearerForMutations()` | Reject POST/PUT/DELETE/PATCH without `Authorization` header (CSRF prevention) | 401 |
| `RejectGuest()` | Block requests from guest-identity users (profile/password/account endpoints) | 403 |
| `RequireEmailVerified(cfg, userRepo)` | Block unverified users when `RequireEmailVerification` enabled. Checks JWT `EmailVerifiedAt` first (avoids DB per request), falls back to DB for legacy tokens. Guest users exempt | 403 |

`accessLevelWeight` map enforces hierarchy: superadmin(4) > admin(3) > moderator(2) > user(1).

## `internal/middleware/ratelimit.go` — Rate Limiters

4 independent limiters:

| Fn | Default Rate | Endpoints |
|----|-------------|-----------|
| `AuthRateLimiter(cfg)` | 10 req/min per IP | register, login, refresh, OAuth, passkey login |
| `ResendRateLimiter(cfg)` | 3 req/min per IP | verification resend |
| `GuestRateLimiter(cfg)` | 5 req/min per IP | guest join |
| `APIRateLimiter(cfg)` | 30 req/min per IP | room creation, chat upload |

All configurable via `RateLimitConfig`. Setting max to 0 disables. All return 429 with JSON error body.
