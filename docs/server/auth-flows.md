# Authentication Flows

How Bedrud handles login, tokens, OAuth, passkeys, email verification, and authorization.

**Packages:** `internal/auth/`, `internal/handlers/auth_handler.go`, `internal/handlers/auth.go`, `internal/middleware/auth.go`

**Public docs:** [Authentication](https://bedrud.org/en/docs/backend/authentication) · [User Roles](https://bedrud.org/en/docs/guides/roles) — [`backend/authentication.mdx`](../../apps/site/src/content/docs/en/backend/authentication.mdx). Full map: [public-docs.md](./public-docs.md).

---

## Token model

| Token | Lifetime | Storage | Purpose |
|-------|----------|---------|---------|
| **Access JWT** | Configurable (`auth.tokenDuration`, default 24h) | `Authorization: Bearer` header or `access_token` cookie | API auth |
| **Refresh token** | Longer; stored hashed in DB | Request body on `/auth/refresh` | Mint new access token |
| **LiveKit token** | Short-lived | Returned on join/refresh | WebRTC room access |

JWT claims (`auth.Claims`):

- `userID`, `email`, `name`, `provider`
- `accesses` — array of access levels
- Standard `exp`, `iat`, `jti`

Secrets:

- `auth.jwtSecret` — **required** at startup (env: `AUTH_JWT_SECRET`)
- `auth.sessionSecret` — OAuth/passkey session cookies (env: `AUTH_SESSION_SECRET`)

---

## Local registration and login

### Register (`POST /api/auth/register`)

```
Client → Register handler
  ├─ Check settings: registrationEnabled, tokenRegistrationOnly
  ├─ Validate invite token if required
  ├─ Hash password (bcrypt)
  ├─ Create user (provider=local)
  ├─ If requireEmailVerification: set email_verified_at=NULL, enqueue verify email
  └─ Return access + refresh tokens (or require verification first)
```

### Login (`POST /api/auth/login`)

```
Client → Login handler
  ├─ Find user by email+provider=local
  ├─ Verify bcrypt password
  ├─ Check is_active (ban)
  ├─ Check email verification if required
  ├─ Issue JWT + refresh token
  └─ Store refresh token hash in users.refresh_token
```

### Guest login (`POST /api/auth/guest-login`)

Creates ephemeral guest user (`provider=guest`, `accesses=[guest]`). Gated by `guestLoginEnabled` in settings.

---

## Token refresh and logout

### Refresh (`POST /api/auth/refresh`)

1. Validate refresh token against DB hash
2. Check `blocked_refresh_tokens` table
3. Check `password_changed_at` — reject tokens issued before password change
4. Issue new access + refresh pair (rotation)

### Logout (`POST /api/auth/logout`)

Requires `Protected()` middleware. Adds current refresh token to blocklist and clears cookie.

### Force logout (admin)

`POST /api/admin/users/:id/force-logout` — invalidates all sessions for user via refresh token blocklist + ban cache refresh.

---

## OAuth (Goth)

Providers: Google, GitHub, Twitter (configured via settings merge or `config.yaml`).

```
GET /api/auth/:provider/login     → redirect to provider
GET /api/auth/:provider/callback  → CallbackHandler
  ├─ Goth session exchange
  ├─ Find or create user (provider = google/github/twitter)
  ├─ Copy avatar, name from profile
  └─ Issue JWT tokens, redirect to frontend
```

Session store: `auth.InitializeSessionStore(sessionSecret, tlsEnabled)` — secure cookies when TLS on.

OAuth client IDs/secrets live in `SystemSettings` (admin UI) with `config.yaml` fallback. See [Settings System](./settings-system.md).

---

## WebAuthn passkeys

Uses `github.com/go-webauthn/webauthn` with in-memory `ChallengeStore` (TTL from `auth.passkeyChallengeTTL`).

| Endpoint | Auth required | Flow |
|----------|---------------|------|
| `POST /auth/passkey/register/begin` | Yes + verified email | Begin credential creation for logged-in user |
| `POST /auth/passkey/register/finish` | Yes | Store passkey in DB |
| `POST /auth/passkey/login/begin` | No | Discoverable or email-based challenge |
| `POST /auth/passkey/login/finish` | No | Verify assertion → JWT |
| `POST /auth/passkey/signup/begin` | No | Create new user + passkey in one flow |
| `POST /auth/passkey/signup/finish` | No | Complete signup |

`getRPID()` / `getOrigin()` derive from request Host header and TLS — important for production domain setup.

Gated by `passkeysEnabled` in settings.

---

## Email verification

Enabled when `auth.requireEmailVerification=true` (config) **and** SMTP configured (or emails log-only).

| Endpoint | Purpose |
|----------|---------|
| `POST /auth/verify` | Click link token → set `email_verified_at` |
| `GET /auth/verify/status` | Check current user verification state |
| `POST /auth/verify/resend` | Resend email (rate limited + cooldown cache) |

**Middleware:** `RequireEmailVerified()` blocks most protected routes for unverified users. Exceptions: verify endpoints themselves.

**Audit:** `verification_events` table records sends, verifies, admin overrides.

**Admin override:**

- `PUT /api/admin/users/:id/verify` — manually verify
- `POST /api/admin/users/:id/resend-verification` — admin-triggered resend

Cooldown: `auth.verificationEmailCooldownMins` (default 2 min) via `handlers.CooldownCache`.

---

## Password reset

```
POST /auth/forgot-password  → enqueue send_email (password_reset template)
POST /auth/reset-password   → validate token, set new password, invalidate old refresh tokens
```

Rate limited via `AuthRateLimiter`. Tokens are time-limited single-use strings (see handler implementation).

`POST /auth/password` (authenticated) — change password with current password verification.

---

## Authorization layers

### 1. JWT validation (`Protected`)

Extracts token from header or cookie → `auth.ValidateToken()` → sets `c.Locals("user", claims)`.

### 2. Ban check (in-memory)

`auth.IsUserBanned(userID)` — O(1) lookup. Populated at startup from inactive users; updated on ban/unban.

### 3. Email verification (`RequireEmailVerified`)

Queries user repo if claims lack verification timestamp.

### 4. Global RBAC (`RequireAccess`)

Hierarchical weights:

| Level | Weight |
|-------|--------|
| superadmin | 4 |
| admin | 3 |
| moderator | 2 |
| user | 1 |

Superadmin passes admin checks. Admin group requires `superadmin` for `/api/admin/*`.

### 5. Room-scoped moderation

`isRoomModerator()` — separate from global RBAC. See [Room Lifecycle](./room-lifecycle.md).

---

## Rate limiting

Configured in `config.yaml` `rateLimit` section. Applied per route group:

| Limiter | Routes |
|---------|--------|
| `AuthRateLimiter` | register, login, refresh, forgot-password, passkey |
| `GuestRateLimiter` | guest-join |
| `APIRateLimiter` | room create, refresh-token, chat upload |
| `ResendRateLimiter` | verification resend |

**Proxy warning:** If behind nginx/Cloudflare without `behindProxy: true`, all clients share one IP bucket. See [architecture.md](./architecture.md).

---

## Security checklist for operators

1. Set `jwtSecret` and `sessionSecret` ≥ 32 random bytes
2. Enable TLS in production
3. Set `behindProxy: true` when behind reverse proxy
4. Configure SMTP before enabling email verification
5. Set explicit CORS origins when `allowCredentials: true`
6. Configure LiveKit webhook URL for external deployments

See [security.md](./security.md).

---

## Related docs

- [internal/auth.md](./internal/auth.md) — AuthService, JWT helpers, ban cache
- [internal/middleware.md](./internal/middleware.md) — full middleware reference
- [email-webhooks.md](./email-webhooks.md) — transactional email templates
- [routes.md](./routes.md) — all auth route paths