---
name: bedrud-api-auth
description: Auth endpoints — JWT flow, local/OAuth/passkey/verify/preferences.
license: Apache License
---

# Bedrud API — Auth Endpoints

---

## Authentication

### JWT Flow

Access + refresh token pair. HMAC-SHA256.
- Access token: configurable duration (`tokenDuration`)
- Refresh token: 7-day expiry
- Tokens set as `HttpOnly` cookies AND returned in JSON body
- Refresh rotation: old refresh blocked on `POST /auth/refresh`

### Middleware

| Middleware | Behavior | Status on Fail |
|-----------|----------|----------------|
| `Protected()` | Extract JWT from `Authorization: Bearer`, fallback cookie. Check banned set | 401 |
| `RequireAccess(level)` | Check `claims.Accesses` (hierarchical) | 403 |
| `RequireBearerForMutations()` | Reject POST/PUT/DELETE/PATCH without `Authorization` | 401 |
| `RejectGuest()` | Block guest-identity users | 403 |
| `RequireEmailVerified(cfg, userRepo)` | Block unverified users. Guests exempt | 403 |
| `AuthRateLimiter(cfg)` | 10 req/min per IP | 429 |
| `ResendRateLimiter(cfg)` | 3 req/min per IP for resend | 429 |
| `GuestRateLimiter(cfg)` | 5 req/min per IP | 429 |
| `APIRateLimiter(cfg)` | 30 req/min per IP for room creation/chat upload | 429 |

### Access Levels

`superadmin` > `admin` > `moderator` > `user` > `guest`

### Error Format

All errors: `{"error": "<message>"}` with appropriate HTTP status.

---

## Global Middleware (all routes)

| Order | Middleware | Purpose |
|-------|-----------|---------|
| 1 | `recover.New()` | Panic recovery |
| 2 | `helmet.New()` | XSS, Content-Type nosniff, X-Frame DENY |
| 3 | `cors.New()` | Config-driven origins/headers/methods |
| 4 | Body limit: 2MB | Custom Fiber config |

---

## Health

| Method | Path | Auth | Res |
|--------|------|------|-----|
| GET | `/api/health` | none | `{"status":"healthy","time":<unix>}` |
| GET | `/api/ready` | none | `{"status":"ready","time":<unix>}` |
| GET | `/api/cert` | none | PEM cert file (TLS only) |

---

## Auth — Local

| Method | Path | Rate Limit | Req | Res | Status |
|--------|------|-----------|-----|-----|--------|
| POST | `/api/auth/register` | AuthRate | `{email, password, name, inviteToken}` | `LoginResponse` | 201 / 400 / 403 |
| POST | `/api/auth/login` | AuthRate | `{email, password}` | `LoginResponse` | 200 / 401 |
| POST | `/api/auth/guest-login` | AuthRate | `{name}` | `LoginResponse` | 200 / 400 |
| POST | `/api/auth/refresh` | AuthRate | `RefreshRequest` | `{accessToken, refreshToken}` | 200 / 401 |
| POST | `/api/auth/logout` | Protected | `LogoutRequest` | `{"message":"Successfully logged out"}` | 200 |
| GET | `/api/auth/me` | Protected | — | `models.User` | 200 |
| PUT | `/api/auth/me` | Protected | `{name}` | `models.User` | 200 / 400 |
| PUT | `/api/auth/password` | Protected | `{currentPassword, newPassword}` | `{"message":"Password updated successfully"}` | 200 / 400 / 401 |

### Notes
- Register: checks `registrationEnabled` + `tokenRegistrationOnly`. If token-only, `inviteToken` required.
- Guest login: transient user, `guest-` prefixed ID, `guest` access.
- Password min: 6 chars. Display name min: 2 chars.

---

## Auth — Email Verification

| Method | Path | Rate Limit | Req | Res |
|--------|------|-----------|-----|-----|
| GET | `/api/auth/verify` | none | query: `token`, `status`, `reason` | Redirect or status page |
| GET | `/api/auth/verify/status` | Protected | — | `{"verified": bool}` |
| POST | `/api/auth/verify/resend` | ResendRate | `{email}` | `{"message":"If the account exists, a verification email has been sent."}` |

Cooldown: 2 min between resends (configurable). Admin can force-verify via admin panel.

---

## Auth — OAuth

| Method | Path | Handler | Res |
|--------|------|---------|-----|
| GET | `/api/auth/{provider}/login` | `BeginAuthHandler` | 307 redirect to provider |
| GET | `/api/auth/{provider}/callback` | `CallbackHandler` | Redirect to `/auth/callback?token=...` |

Providers: `google`, `github`, `twitter`. Flow: redirect → consent → callback → upsert user → JWT cookies → redirect frontend.

---

## Auth — Passkeys (WebAuthn)

| Method | Path | Rate Limit | Req | Res |
|--------|------|-----------|-----|-----|
| POST | `/api/auth/passkey/register/begin` | — | — | WebAuthn creation options |
| POST | `/api/auth/passkey/register/finish` | — | `{clientDataJSON, attestationObject}` | `{"message":"Passkey registered successfully"}` |
| POST | `/api/auth/passkey/login/begin` | AuthRate | — | WebAuthn request options |
| POST | `/api/auth/passkey/login/finish` | AuthRate | `{credentialId, clientDataJSON, authenticatorData, signature}` | `LoginResponse` |
| POST | `/api/auth/passkey/signup/begin` | AuthRate | `{email, name, inviteToken}` | WebAuthn creation options |
| POST | `/api/auth/passkey/signup/finish` | AuthRate | `{clientDataJSON, attestationObject}` | `LoginResponse` |

RP ID derived from request origin. Relying party name: "Bedrud".

---

## Preferences

| Method | Path | Auth | Req | Res |
|--------|------|------|-----|-----|
| GET | `/api/auth/preferences` | Protected | — | `{"preferencesJson":"..."}` |
| PUT | `/api/auth/preferences` | Protected | `{preferencesJson}` | `{"message":"Preferences updated"}` |

JSON string, max 4KB. Upsert.

---

## Public Settings

| Method | Path | Auth | Res |
|--------|------|------|-----|
| GET | `/api/auth/settings` | none | `{registrationEnabled, tokenRegistrationOnly, passkeysEnabled, oauthProviders}` |

No secrets exposed.
