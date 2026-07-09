---
name: bedrud-api-auth
description: Auth endpoints — JWT flow, local/OAuth/passkey/verify/password-reset/avatar/preferences.
license: Apache License
---

# Bedrud API — Auth Endpoints

Source of truth: `server/internal/server/server.go` (routes), `handlers/auth_handler.go`, `handlers/auth.go`, `handlers/preferences_handler.go`, `middleware/auth.go`, `middleware/ratelimit.go`, `auth/auth.go`, `auth/jwt.go`.

---

## Authentication

### JWT Flow

Access + refresh token pair. HMAC-SHA256 (`auth.jwtSecret`).

| Token | Duration | Cookie | JSON field (login body) |
|-------|----------|--------|-------------------------|
| Access | `tokenDuration` hours (config/settings) | `access_token` HttpOnly | `tokens.accessToken` |
| Refresh | 7 days | `refresh_token` HttpOnly | `tokens.refreshToken` |

- Cookies: `Path=/`, `HttpOnly`, `Secure` when TLS or `behindProxy`, `SameSite=Lax` (or `None` when secure), optional `Domain` from config.
- Refresh rotation: `POST /auth/refresh` atomically replaces stored refresh; old token blocked. Concurrent reuse → **409**.
- Access revocation: in-memory set on logout / password change / force-logout (lost on restart until natural expiry).
- Refresh request body uses **snake_case** `refresh_token`; login response uses **camelCase** under `tokens`.

### LoginResponse shape

```json
{
  "user": { "id", "email", "name", "provider", "avatarUrl", "accesses", "isActive", "emailVerifiedAt", "passwordChangedAt", "createdAt", "updatedAt" },
  "tokens": { "accessToken": "...", "refreshToken": "..." }
}
```

(`Password` / stored `RefreshToken` never serialized.)

### Middleware

| Middleware | Behavior | Fail |
|-----------|----------|------|
| `Protected()` | JWT from `Authorization: Bearer` (or raw header), else cookie `access_token`. Banned users → 403 | 401 / 403 |
| `RequireAccess(level)` | Hierarchical weight: superadmin(4) > admin(3) > moderator(2) > user(1). After `Protected()` | 403 |
| `RequireEmailVerified(cfg, userRepo)` | When `requireEmailVerification` on: DB check `EmailVerifiedAt`. Guests exempt. After `Protected()` | 403 |
| `RequireBearerForMutations()` | POST/PUT/DELETE/PATCH need `Authorization` header (CSRF). **Defined but not mounted** on current routes | 401 |
| `RejectGuest()` | Block `provider == "guest"`. **Defined but not mounted** on current auth routes | 403 |
| `AuthRateLimiter` | Default 10 / 60s / IP (`authMaxRequests` / `authWindowSecs`; 0 = off) | 429 |
| `ResendRateLimiter` | Default 3 / 60s / IP (`authResendMaxRequests` / `authResendWindowSecs`) | 429 |
| `GuestRateLimiter` | Default 5 / 60s / IP — used on **room** guest-join, not auth guest-login | 429 |
| `APIRateLimiter` | Default 30 / 60s / IP — rooms/uploads, not auth | 429 |

### Access Levels

`superadmin` > `admin` > `moderator` > `user` > `guest`

Providers: `local`, `passkey`, `guest`, plus OAuth (`google`, `github`, `twitter` when configured).

### Error Format

`{"error":"<message>"}` (+ optional fields like `requiresVerification`, `email`, `already_verified`).

---

## Global Middleware (all routes)

| Order | Middleware | Purpose |
|-------|-----------|---------|
| 1 | `recover.New()` | Panic recovery |
| 2 | `helmet.New()` | XSS, nosniff, X-Frame DENY, referrer |
| 3 | `cors.New()` | Config origins/headers; credentials require explicit origins |
| 4 | Body limit 2MB | Fiber `BodyLimit` |

API group prefix: `/api`.

---

## Health / System (related public)

| Method | Path | Auth | Res | Status |
|--------|------|------|-----|--------|
| GET | `/api/health` | none | `{"status":"healthy","time":<unix>}` | 200 |
| GET | `/api/ready` | none | `{"status":"ready","time":...}` or DB fail | 200 / 503 |
| GET | `/health`, `/ready` | none | redirect → `/api/...` | 307 |
| GET | `/api/cert` | none | PEM cert download | 200 / 404 |
| GET | `/uploads/avatars/*` | none | avatar file | 200 / 400 |

---

## Auth — Local

| Method | Path | Auth / Limit | Req | Res | Status |
|--------|------|--------------|-----|-----|--------|
| POST | `/api/auth/register` | AuthRate | `{email, password, name, inviteToken?}` | `LoginResponse` **or** verification gate | 200 / 400 / 403 / 409 / 500 |
| POST | `/api/auth/login` | AuthRate | `{email, password}` | `LoginResponse` | 200 / 400 / 401 / 403 |
| POST | `/api/auth/guest-login` | AuthRate | `{name}` | `LoginResponse` | 200 / 400 / 403 / 500 |
| POST | `/api/auth/refresh` | AuthRate | `{refresh_token}` or cookie | `{access_token, refresh_token}` | 200 / 400 / 401 / 403 / 409 / 500 |
| POST | `/api/auth/logout` | Protected | `{refresh_token?}` or cookie | `{"message":"Successfully logged out"}` | 200 |
| GET | `/api/auth/me` | Protected + EmailVerified | — | `models.User` | 200 / 401 / 500 |
| PUT | `/api/auth/me` | Protected + EmailVerified | `{name, email?}` | profile obj (+ verification fields) | 200 / 400 |
| POST | `/api/auth/me/avatar` | Protected + EmailVerified | multipart `avatar` | profile obj | 200 / 400 / 500 |
| DELETE | `/api/auth/me/avatar` | Protected + EmailVerified | — | profile obj | 200 / 400 |
| PUT | `/api/auth/password` | Protected + EmailVerified | `{currentPassword, newPassword}` | `{"message":"Password updated successfully"}` | 200 / 400 |

### Profile response (update / avatar)

```json
{ "id", "name", "email", "provider", "accesses", "avatarUrl" }
```

Email change (local/passkey only): sets `requiresVerification`, optional `tokens: {accessToken, refreshToken}`, revokes old access JWT, enqueues verification (cooldown-gated).

### Register notes

- Email canonicalized; name 2–255 after control/HTML strip.
- Password **12–128** chars (`MinPasswordLength` / `MaxPasswordLength`).
- Settings: `registrationEnabled`, `tokenRegistrationOnly` + valid unused invite.
- Invite marked used before user create (409 if race).
- If `requireEmailVerification`: **no tokens** → `{requiresVerification, message, email}`; else cookies + `LoginResponse` + welcome email job.

### Login notes

- Unverified local user when verification required → **403** `{error, requiresVerification, email}`.
- Bad credentials always **401** `"Invalid credentials"`.

### Guest notes

- Name 2–50 after sanitize. Needs `registrationEnabled` + `guestLoginEnabled`.
- Transient user, `guest` provider/access.

### Refresh notes

- Body or `refresh_token` cookie. Re-loads user (accesses, active, verified).
- Response keys: **`access_token` / `refresh_token`** (snake_case). Sets cookies.
- Already-rotated refresh → **409**.

### Logout notes

- Best-effort block refresh + revoke access; always clears cookies.

### Password change notes

- Verifies current password; clears refresh sessions; revokes access token; enqueues `password_changed` email.

---

## Auth — Password Reset

| Method | Path | Limit | Req | Res | Status |
|--------|------|-------|-----|-----|--------|
| POST | `/api/auth/forgot-password` | AuthRate | `{email}` | `{"message":"If the account exists, a password reset email has been sent"}` | 200 / 400 |
| POST | `/api/auth/reset-password` | AuthRate | `{token, newPassword}` | success message | 200 / 400 / 500 |

### Notes

- Forgot: uniform 200 (no enumeration). Local/passkey only. Silent per-user cooldown via `CooldownCache` (default TTL 2m / `verificationEmailCooldownMins`). Email-hash key also consumed.
- Reset token JWT purpose `password_reset`; binds email + `passwordChangedAt`; frontend link `/auth/reset-password?token=`.
- New password 12–128; OAuth accounts rejected; clears sessions + enqueues notification.

---

## Auth — Email Verification

| Method | Path | Auth / Limit | Req | Res | Status |
|--------|------|--------------|-----|-----|--------|
| POST | `/api/auth/verify` | none | `{token}` | `{access_token, refresh_token, verified:true}` | 200 / 400 / 401 / 404 / 409 / 500 |
| GET | `/api/auth/verify/status` | Protected | — | `{verified, email}` | 200 / 401 |
| POST | `/api/auth/verify/resend` | ResendRate | `{email}` | `{"message":"If the account exists, a verification email has been sent"}` | 200 / 400 |

### Notes

- **POST body token**, not GET query redirect (frontend page consumes token then POSTs).
- Already verified → **409** `{error, already_verified:true}`.
- Success issues tokens (snake_case); stores refresh; does **not** call `setAuthCookies` (client uses JSON or re-login).
- Resend: uniform 200; silent cooldown; audit events via `VerificationEventRepository`.
- Frontend verify link: `{frontendURL}/auth/verify?token=...` (email only).

---

## Auth — OAuth (Goth)

| Method | Path | Limit | Handler | Res |
|--------|------|-------|---------|-----|
| GET | `/api/auth/:provider/login` | AuthRate | `BeginAuthHandler` | 307 → provider (or 400 unsupported) |
| GET | `/api/auth/:provider/callback` | AuthRate | `CallbackHandler` | redirect or JSON |

Providers: only those returned by `auth.ConfiguredProviders()` (`google`, `github`, `twitter` when secrets set).

### Flow

1. Begin → gothic session cookie + 307 to IdP.
2. Callback → CompleteUserAuth → upsert by email+provider.
3. New users blocked if registration disabled or token-only mode (existing users may still log in when registration off).
4. OAuth email treated verified if `requireEmailVerification` and not yet set.
5. Tokens + cookies; if `frontendURL` set → redirect **`{frontendURL}/auth/callback`** (token **not** in query string — cookies only). Else JSON `AuthResponse` `{user, token}` (access only in body field `token`).

Deactivated users → 403.

---

## Auth — Passkeys (WebAuthn)

| Method | Path | Auth / Limit | Req | Res | Status |
|--------|------|--------------|-----|-----|--------|
| POST | `/api/auth/passkey/register/begin` | Protected + EmailVerified | — | `{challenge, user:{id,name,displayName}, rp:{id,name}}` | 200 / 500 |
| POST | `/api/auth/passkey/register/finish` | Protected + EmailVerified | `{clientDataJSON, attestationObject}` base64url | `{"message":"Passkey registered successfully"}` | 200 / 400 |
| POST | `/api/auth/passkey/login/begin` | AuthRate | — | `{challenge, rpId}` | 200 / 500 |
| POST | `/api/auth/passkey/login/finish` | AuthRate | `{credentialId, clientDataJSON, authenticatorData, signature}` base64url | `LoginResponse` | 200 / 400 / 401 / 403 |
| POST | `/api/auth/passkey/signup/begin` | AuthRate | `{email, name, inviteToken?}` | creation options | 200 / 400 / 403 / 500 |
| POST | `/api/auth/passkey/signup/finish` | AuthRate | `{clientDataJSON, attestationObject}` | `LoginResponse` or verification gate | 200 / 400 / 500 |

### Notes

- Challenges in `ChallengeStore` + gothic session IDs for login/signup.
- RP ID: `server.domain` or request hostname. RP `name` = same as RP ID (not hard-coded "Bedrud").
- Origin: `auth.frontendURL` or request host/`X-Forwarded-Proto`.
- Signup mirrors register settings (invite, registration enabled); name 2–100.
- Signup + verification required → `{requiresVerification, message, email}` (empty tokens from service).
- Login finish: unverified → 403 + `requiresVerification` like password login.
- Successful login/signup sets auth cookies.

---

## Preferences

| Method | Path | Auth | Req | Res | Status |
|--------|------|------|-----|-----|--------|
| GET | `/api/auth/preferences` | Protected + EmailVerified | — | `{"preferencesJson":"..."}` (default `{}`) | 200 / 500 |
| PUT | `/api/auth/preferences` | Protected + EmailVerified | `{preferencesJson}` | `{"message":"Preferences updated"}` | 200 / 400 / 413 / 500 |

- Max 4 KB; must be valid JSON **object** (`{...}`). Upsert by user ID.

---

## Public Settings

| Method | Path | Auth | Handler | Res |
|--------|------|------|---------|-----|
| GET | `/api/auth/settings` | none | `adminHandler.GetPublicSettings` | public fields only |

```json
{
  "serverName": "...",
  "registrationEnabled": true,
  "tokenRegistrationOnly": false,
  "guestLoginEnabled": true,
  "passkeysEnabled": true,
  "oauthProviders": ["google", "github"],
  "requireEmailVerification": false,
  "chatMaxMessageCount": 10000,
  "chatMessageTTLHours": 2160,
  "recordingsEnabled": false
}
```

No secrets.

---

## Shared constraints

| Rule | Value |
|------|--------|
| Password length | 12–128 |
| Display name (register/profile) | 2–255 |
| Guest name | 2–50 |
| Passkey signup name | 2–100 |
| Avatar | multipart field `avatar`, max 2 MB, served at `/uploads/avatars/*` |
| Preferences blob | ≤ 4 KB JSON object |
| Email verification / forgot cooldown | default 2 minutes (`auth.verificationEmailCooldownMins`) |
| Error body | `{"error":"..."}` |

---

## Source index

| Concern | Path |
|---------|------|
| Route wiring | `server/internal/server/server.go` |
| Local / verify / passkey / avatar / reset | `server/internal/handlers/auth_handler.go` |
| OAuth begin/callback | `server/internal/handlers/auth.go` |
| Preferences | `server/internal/handlers/preferences_handler.go` |
| Public settings | `server/internal/handlers/admin_handler.go` (`GetPublicSettings`) |
| Password constants | `server/internal/handlers/models.go` |
| JWT + revocation | `server/internal/auth/jwt.go` |
| Auth service / LoginResponse | `server/internal/auth/auth.go` |
| Middleware | `server/internal/middleware/auth.go`, `ratelimit.go` |
| User model | `server/internal/models/user.go` |
| Avatar storage | `server/internal/storage/avatar.go` |
