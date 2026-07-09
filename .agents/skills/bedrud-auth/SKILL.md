---
name: bedrud-auth
description: Auth service + middleware — JWT, passkeys, OAuth, session store, rate limiting, recordings gate.
license: Apache License
---

# Bedrud Auth Subsystem

Go module `bedrud`. Roots: `server/internal/auth/` + `server/internal/middleware/`.

Handlers live in `internal/handlers/` (not this skill). This skill is the service + middleware layer only.

---

## Files

| Path | Role |
|------|------|
| `auth/auth.go` | `AuthService`, password/passkey/OAuth, DTOs, errors |
| `auth/jwt.go` | Access/refresh/verify/reset JWTs, ban set, access-token revocation |
| `auth/session_store.go` | Goth gorilla CookieStore + Fiber bridge |
| `auth/challenge_store.go` | In-memory WebAuthn challenge store |
| `auth/email.go` | `CanonicalizeEmail` |
| `middleware/auth.go` | JWT + RBAC + CSRF + guest + email-verify gates |
| `middleware/ratelimit.go` | Per-IP Fiber limiters |
| `middleware/recordings_enabled.go` | Settings gate for recordings |

---

## `internal/auth/auth.go` — AuthService

```
AuthService { userRepo *repository.UserRepository, passkeyRepo *repository.PasskeyRepository }
```

### Password helpers (package-level)

| Fn | Notes |
|----|-------|
| `HashPassword(password)` | `bcrypt(sha256(password))` — bypasses bcrypt 72-byte limit |
| `VerifyPassword(password, storedHash)` | Tries sha256+bcrypt then plain bcrypt (legacy). Both paths always run (constant-ish timing) |

### Constructor + core auth

| Fn | Purpose |
|----|---------|
| `NewAuthService(userRepo, passkeyRepo)` | Constructor |
| `Register(email, password, name)` | Local user; `HashPassword`; `Provider=local`, `Accesses=["user"]`, `IsActive=true`. No tokens. |
| `Login(email, password)` | Lookup + `VerifyPassword` (dummy hash if missing user). Blocks inactive. If `RequireEmailVerification` && unverified → `*ErrEmailNotVerified`. Else token pair + store refresh. |
| `GuestLogin(name)` | Creates user `Provider=guest`, `Accesses=["guest"]`, synthetic email `guest_<uuid>@bedrud.guest`. Tokens with nil email verified. |

### Refresh / session

| Fn | Purpose |
|----|---------|
| `UpdateRefreshToken(userID, token)` | Persist refresh on user row |
| `RotateRefreshToken(userID, oldRaw, newRaw)` | Block old (if parseable) + `UpdateRefreshTokenAtomic`. Fail → `ErrRefreshTokenMismatch` |
| `ValidateRefreshToken(refreshToken)` | Not blocked → `ValidateToken` → match stored refresh for user. Mismatch → `ErrRefreshTokenMismatch` |
| `BlockRefreshToken(userID, refreshToken)` | Parse exp → insert blocked_refresh_tokens |
| `ClearRefreshToken(userID)` | Clear stored refresh (invalidate sessions) |
| `Logout(userID, refreshToken, accessToken)` | Block refresh + `RevokeAccessToken` |

### User profile / account

| Fn | Purpose |
|----|---------|
| `GetUserByID` / `GetUserByEmail` | Lookups. Email: IDNA unicode fallback for pre-Punycode rows |
| `UpdateUser(user)` | Full persist (e.g. set `EmailVerifiedAt`) |
| `UpdateProfile(userID, name)` | Display name |
| `UpdateAvatarURL(userID, avatarURL)` | Set avatar URL |
| `ClearAvatar(userID)` | Clear URL; if `/uploads/avatars/` path, delete files via `storage.DeleteUserAvatarFiles` |
| `ChangeEmail(userID, newEmail)` | Canonicalize + parse; uniqueness; clear verification; clear refresh |
| `ChangePassword(userID, current, new, accessToken)` | Local or passkey provider only. Empty password (passkey-only) skips current check. `UpdatePassword` + `RevokeAccessToken` |
| `ResetPassword(userID, newPassword, accessTokens...)` | No current password. `UpdatePassword` + revoke any passed access tokens |
| `UpdateUserAccesses(userID, accesses)` | Replace role list |

### Passkeys (WebAuthn via `go-passkeys`)

| Fn | Purpose |
|----|---------|
| `BeginRegisterPasskey(userID)` | Random 32B challenge, base64.RawURLEncoding |
| `FinishRegisterPasskey(userID, challenge, clientDataJSON, attestationObject, rpID, origin)` | Verify attestation; reject duplicate credential; store passkey |
| `FinishSignupPasskey(userID, email, name, challenge, …, rpID, origin)` | TX create user (`Provider=passkey`) + passkey. If email verification required → empty `TokenPair` (handler sends requiresVerification). Else issue tokens |
| `BeginLoginPasskey()` | Random challenge |
| `FinishLoginPasskey(challenge, credentialID, clientDataJSON, authenticatorData, signature, rpID, origin)` | Assertion + counter update; inactive / email-verify gates; tokens |

### OAuth (Goth)

| Fn | Purpose |
|----|---------|
| `Init(cfg)` | Build Google/GitHub/Twitter from config (skips empty or placeholder creds) |
| `ReloadProviders(s *models.SystemSettings)` | Rebuild from admin-saved settings |
| `ConfiguredProviders() []string` | Active provider names for public settings UI |

Placeholders skipped if id/secret lowercased starts with `your-`, `replace-`, `example-`, `xxx`, `todo`.

### Errors

| Export | Meaning |
|--------|---------|
| `ErrRefreshTokenMismatch` | Presented refresh ≠ stored (rotation / multi-device) |
| `ErrEmailNotVerified{Email}` | Login/passkey blocked until verify; message: "please verify your email before signing in" |

### DTOs

`ErrorResponse`, `RegisterRequest`, `LoginRequest`, `GuestLoginRequest`, `TokenResponse`, `TokenPair`, `LoginResponse`, `LogoutRequest`.

---

## `internal/auth/jwt.go` — Tokens + ban/revoke

### Claims

```
Claims {
  UserID, Email, Name, Provider string
  Accesses []string
  Purpose string                    // "email_verify" | "password_reset" | "" for session tokens
  EmailVerifiedAt *time.Time        // in access/refresh pairs for clients; middleware does NOT trust for gate
  PasswordChangedAt *int64          // unix; embedded in reset tokens
  jwt.RegisteredClaims              // issuer/audience "bedrud", HS256
}
```

### Access / refresh

| Fn | Purpose |
|----|---------|
| `GenerateToken(userID, email, name, provider, accesses, cfg, emailVerifiedAt)` | Access; exp = `cfg.Auth.TokenDuration` hours |
| `GenerateTokenPair(..., provider, accesses, cfg, emailVerifiedAt)` | Access + refresh (7d, with `jti` uuid) |
| `ValidateToken(tokenString, cfg)` | HMAC + iss/aud + not revoked. Fail revoked → `ErrTokenRevoked` |
| `RevokeAccessToken(tokenStr, cfg)` | In-memory SHA-256 hash → exp. Lost on restart |
| `PruneRevokedTokens()` | Drop expired revocation entries (call periodically) |

### Purpose-scoped tokens

| Fn | Purpose / TTL |
|----|----------------|
| `GenerateVerificationToken(userID, email, cfg)` | `purpose=email_verify`. Default 24h; `cfg.Auth.VerificationTokenTTLHours` |
| `ValidateVerificationToken(...)` | Returns userID, email if purpose ok |
| `GenerateResetToken(userID, email, passwordChangedAt, cfg)` | `purpose=password_reset` + optional pca. Default 1h; `cfg.Auth.ResetTokenTTLHours` |
| `ValidateResetToken(...)` | Returns userID, email, `*passwordChangedAt` |

### In-memory ban set

| Fn | Purpose |
|----|---------|
| `BanUser` / `UnbanUser` | Admin deactivate/reactivate path |
| `LoadBannedUsersFromDB(ids)` | Startup hydrate from inactive users |
| `IsUserBanned(userID)` | Used by `Protected` |

**Note:** Access revocation and ban sets are process-local (not multi-instance safe without shared store).

---

## `internal/auth/session_store.go`

| Fn | Purpose |
|----|---------|
| `InitializeSessionStore(secret, secure)` | Goth `CookieStore`: Path `/`, MaxAge 3600, HttpOnly. Secure+SameSiteNone if TLS; else Lax |
| `SetProviderToSession(c, provider)` | Fiber → synthetic `http.Request` for gothic session `provider` key |

---

## `internal/auth/challenge_store.go`

```
ChallengeStore { store map[string]*challengeEntry, ttl }
// challengeEntry: Challenge, UserID, Extra map[string]string, ExpiresAt
```

| Fn | Purpose |
|----|---------|
| `NewChallengeStore(ttlMinutes)` | Default TTL 5 min if ≤0. Wired with `cfg.Auth.PasskeyChallengeTTL` |
| `Set(key, challenge, userID, extra)` | Upsert with expiry |
| `GetAndVerify(key, expectedChallenge)` | Not found / expired / mismatch → ok=false; returns challenge + extra |
| `Delete(key)` | Remove |
| `StartCleanup(stop)` | 1-min ticker purge until `stop` closed |

---

## `internal/auth/email.go`

`CanonicalizeEmail(email)`:

1. Trim space  
2. NFKC normalize  
3. Strip BOM  
4. Domain → IDNA ASCII (Punycode)  
5. Lowercase local + domain  

Used by change-email and registration paths (handlers may also canonicalize on input).

---

## `internal/middleware/auth.go`

Must run `Protected` before claims-based middleware. Locals key: `"user"` → `*auth.Claims`.

| Fn | Behavior | Fail |
|----|----------|------|
| `Protected()` | Bearer from `Authorization` (prefix-optional), else cookie `access_token`. `ValidateToken`. Ban → deactivated. Sets locals | 401 missing/invalid; **403** banned |
| `RequireAccess(level models.AccessLevel)` | Hierarchical weight on `claims.Accesses`. Unknown required level → fail closed (weight 9999) | 401 no claims; 403 insufficient |
| `RequireBearerForMutations()` | POST/PUT/DELETE/PATCH must have `Authorization: Bearer …` (cookie-only mutations blocked — CSRF) | 401 |
| `RejectGuest()` | `claims.Provider == "guest"` blocked | 401 no claims; 403 guest |
| `RequireEmailVerified(cfg, userRepo)` | No-op if `!cfg.Auth.RequireEmailVerification`. Guests exempt. **Always DB** (`user.EmailVerifiedAt`); never trusts JWT claim (stale-token / admin un-verify safe) | 401 missing/not found; 403 unverified |

### Access hierarchy (`accessLevelWeight`)

`superadmin(4) > admin(3) > moderator(2) > user(1)`. Higher weight satisfies lower required level.

---

## `internal/middleware/ratelimit.go`

Fiber `limiter`, key = client IP. `max == 0` → no-op passthrough.

| Fn | Defaults | Config fields |
|----|----------|---------------|
| `AuthRateLimiter(cfg)` | 10 / 60s | `AuthMaxRequests`, `AuthWindowSecs` |
| `ResendRateLimiter(cfg)` | 3 / 60s | `AuthResendMaxRequests`, `AuthResendWindowSecs` |
| `GuestRateLimiter(cfg)` | 5 / 60s | `GuestMaxRequests`, `GuestWindowSecs` |
| `APIRateLimiter(cfg)` | 30 / 60s | `APIMaxRequests`, `APIWindowSecs` |

429 bodies: auth/resend `"too many requests, please try again later"`; guest `"too many guest join attempts"`; API `"too many requests"`.

Typical mounts (handlers/server): auth routes → Auth; verification resend → Resend; guest → Guest; room create / chat upload → API.

---

## `internal/middleware/recordings_enabled.go`

| Fn | Behavior |
|----|----------|
| `RecordingsEnabled(settingsRepo)` | `GetSettings()`; if `!RecordingsEnabled` → 403 `"Recordings are disabled on this server"`. Settings load error → 500. First gate; service layer re-checks. File marked TODO oncoming feature. |

---

## Config touchpoints (auth-related)

| Field | Use |
|-------|-----|
| `Auth.JWTSecret` | HS256 signing |
| `Auth.TokenDuration` | Access token hours |
| `Auth.RequireEmailVerification` | Login/passkey gates + middleware |
| `Auth.VerificationTokenTTLHours` | Email verify JWT |
| `Auth.ResetTokenTTLHours` | Password reset JWT |
| `Auth.PasskeyChallengeTTL` | Challenge store minutes (default 5) |
| `Auth.Google/Github/Twitter` | OAuth client id/secret/redirect |
| `RateLimit.*` | Limiter max/window pointers (`nil` → code defaults) |

---

## Gotchas

- **Password hashing:** always `HashPassword` / `VerifyPassword`; do not call bcrypt directly.
- **Logout / password change:** revoke access token in-memory + block/clear refresh in DB.
- **Refresh rotation:** use `RotateRefreshToken`, not blind `UpdateRefreshToken`, for concurrent-safe refresh.
- **Email verify middleware:** DB always; JWT `EmailVerifiedAt` is for clients / optional optimization elsewhere only.
- **Purpose tokens:** never use verify/reset JWTs as access tokens (`Purpose` must match).
- **Multi-instance:** ban set + access revocation are per-process; refresh blocklist is DB-backed.
- **OAuth reload:** after admin settings save, call `ReloadProviders` so Goth matches DB settings.
- **Guests:** exempt from email verification; blocked by `RejectGuest` on account endpoints.
