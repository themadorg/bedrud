# Authentication (`internal/auth`)

Multi-layered authentication: JWT tokens, OAuth2 (Goth), WebAuthn passkeys, and email verification.

---

## Files

| File | Purpose |
|------|---------|
| `auth.go` | `AuthService` — register, login, guest, passkey, profile, password, logout |
| `jwt.go` | Token generation/validation, revocation, ban set, purpose tokens |
| `session_store.go` | Gorilla CookieStore for Goth OAuth sessions |
| `challenge_store.go` | In-memory WebAuthn challenge TTL store |
| `email.go` | Email verification enqueue helpers |

---

## AuthService (`auth.go`)

```go
type AuthService struct {
    userRepo    *repository.UserRepository
    passkeyRepo *repository.PasskeyRepository
}
```

| Method | Purpose |
|--------|---------|
| `Register(email, password, name)` | Create local user with bcrypt hash |
| `Login(email, password)` | Validate credentials → JWT pair |
| `GuestLogin(name)` | Transient guest (`ProviderGuest`, `guest-` ID prefix) |
| `UpdateProfile(userID, name)` | Update display name |
| `ChangePassword(userID, current, new)` | Verify old → hash new |
| `Logout(userID, refreshToken)` | Block refresh token + revoke access token |
| `ValidateRefreshToken(token)` | Check blocklist → validate JWT |
| `BeginRegisterPasskey` / `FinishRegisterPasskey` | WebAuthn for logged-in user |
| `BeginLoginPasskey` / `FinishLoginPasskey` | WebAuthn authentication |
| `FinishSignupPasskey` | Passkey-only account creation |
| `Init(cfg)` | Register OAuth providers via Goth |

---

## JWT (`jwt.go`)

### Access + refresh tokens

- Signing: HMAC-SHA256, issuer `bedrud`, audience `bedrud`
- Access expiry: `auth.tokenDuration` (hours)
- Refresh expiry: 7 days, includes unique `jti` (UUID)

### Claims

```go
type Claims struct {
    UserID            string
    Email             string
    Name              string
    Provider          string
    Accesses          []string
    Purpose           string     // "email_verify" | "password_reset" | empty
    EmailVerifiedAt   *time.Time // embedded in access tokens
    PasswordChangedAt *int64     // unix, for reset token invalidation
    jwt.RegisteredClaims
}
```

### Purpose-specific tokens

| Function | Purpose claim | TTL |
|----------|---------------|-----|
| `GenerateVerificationToken` | `email_verify` | `verificationTokenTTLHours` (default 24h) |
| `GenerateResetToken` | `password_reset` | `resetTokenTTLHours` (default 1h) |

Purpose tokens cannot be used as access tokens (`ValidateToken` checks purpose indirectly via separate validators).

### Access token revocation

In-memory revocation set keyed by SHA-256 hash of token:

- `RevokeAccessToken(token, cfg)` — on logout, password change, force-logout
- `ValidateToken` checks `isRevoked()` before accepting
- `PruneRevokedTokens()` — hourly scheduler cleanup of expired entries

**Trade-off:** Revocation list is in-memory; lost on restart. Revoked tokens valid until natural expiry (up to `tokenDuration`). Refresh tokens use DB blocklist (`BlockedRefreshToken`) which persists.

### Deactivated user ban set

Separate in-memory set for fast middleware checks:

- `LoadBannedUsersFromDB([]userID)` — startup from `GetInactiveUserIDs()`
- `BanUser(userID)` / `UnbanUser(userID)` — runtime admin actions
- `IsUserBanned(userID)` — checked in `middleware.Protected()`

---

## OAuth (Goth)

Configured in `AuthService.Init()` from `config.yaml` and/or `SystemSettings` DB overrides.

| Provider | Config |
|----------|--------|
| Google | `auth.google.*` |
| GitHub | `auth.github.*` |
| Twitter | `auth.twitter.*` |

Flow handled in `handlers/auth.go` + `handlers/auth_handler.go` (`CallbackHandler`).

---

## Passkeys (WebAuthn)

- Library: `github.com/go-passkeys/go-passkeys`
- Challenge store: `auth.NewChallengeStore(passkeyChallengeTTL)` — in-memory, minutes TTL
- RP ID derived from request `Origin` header
- Credentials in `Passkey` model (credential ID, public key, counter)

---

## Email verification

When `requireEmailVerification: true`:

1. Register creates user with `EmailVerifiedAt = nil`
2. Verification email enqueued (`send_email` job) or logged without SMTP
3. `POST /api/auth/verify` validates `email_verify` purpose JWT
4. Resend gated by `ResendRateLimiter` + `CooldownCache` (per-email, default 2 min)
5. Unverified accounts deleted by scheduler (default 48h TTL)

Admin can force-verify via `POST /api/admin/users/:id/verify`.

---

## User providers (`models/user.go`)

| Constant | Value |
|----------|-------|
| `ProviderLocal` | `local` |
| `ProviderPasskey` | `passkey` |
| `ProviderGuest` | `guest` |
| OAuth | `google`, `github`, `twitter` |

---

## Cookie handling (`handlers/auth_handler.go`)

- `setAuthCookies` — HttpOnly `access_token` + `refresh_token`
- `clearAuthCookies` — on logout
- Secure/SameSite/domain from TLS and config