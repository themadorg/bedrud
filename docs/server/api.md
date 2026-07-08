# API Reference

Bedrud exposes a REST API under `/api/`. All endpoints return JSON unless noted. Errors: `{"error": "<message>"}`.

**Authoritative route list:** [routes.md](./routes.md)  
**Interactive docs:** `/api/swagger`, `/api/scalar`  
**OpenAPI spec:** `server/docs/swagger.yaml`

**Public docs:** [API Reference](https://bedrud.org/en/docs/api/api-reference) — interactive Swagger on the docs site — [`api/api-reference.mdx`](../../apps/site/src/content/docs/en/api/api-reference.mdx). Code-audited routes: [routes.md](./routes.md). Full map: [public-docs.md](./public-docs.md).

---

## Authentication

### JWT flow

- Access + refresh token pair, HMAC-SHA256, issuer/audience `bedrud`
- Access duration: `auth.tokenDuration` (hours)
- Refresh: 7-day expiry, rotatable via blocklist
- Cookies: HttpOnly `access_token`, `refresh_token` + JSON body
- Access revocation: in-memory on logout (see [auth.md](./internal/auth.md))
- Refresh revocation: `blocked_refresh_tokens` table (persistent)

### Header

```
Authorization: Bearer <access_token>
```

### Access levels (hierarchical)

`superadmin` ≥ `admin` ≥ `moderator` ≥ `user` > `guest`

`RequireAccess(superadmin)` also allows users with higher weight.

### Middleware summary

| Middleware | Fail | Notes |
|-----------|------|-------|
| `Protected()` | 401 | JWT + ban check |
| `RequireEmailVerified()` | 403 | DB check when verification enabled |
| `RequireAccess(level)` | 403 | Hierarchical |
| `AuthRateLimiter()` | 429 | Login, register, refresh, passkey |
| `ResendRateLimiter()` | 429 | Verification resend only |
| `GuestRateLimiter()` | 429 | Guest join |
| `APIRateLimiter()` | 429 | Room create, upload, refresh-token |

---

## Health

| Method | Path | Response |
|--------|------|----------|
| GET | `/api/health` | `{"status":"healthy","time":<unix>}` |
| GET | `/api/ready` | 200 ready / 503 if DB unavailable |

---

## Auth endpoints (summary)

| Group | Key endpoints |
|-------|---------------|
| Local | `register`, `login`, `guest-login`, `refresh`, `logout`, `me`, `password` |
| Verify | `verify`, `verify/status`, `verify/resend` |
| Reset | `forgot-password`, `reset-password` |
| OAuth | `/:provider/login`, `/:provider/callback` |
| Passkeys | `passkey/register/*`, `passkey/login/*`, `passkey/signup/*` |
| Prefs | `preferences` GET/PUT |
| Public | `settings` (registration flags, OAuth list) |

---

## Room endpoints (summary)

| Group | Key endpoints |
|-------|---------------|
| Lifecycle | `create`, `join`, `guest-join`, `refresh-token`, `list`, `archived`, `settings`, `delete` |
| Moderation | kick, ban, mute, video off, screenshare stop, promote, demote, chat block, deafen, spotlight, stage |
| Upload | `chat/upload` (multipart), static `GET /uploads/chat/*` |
| Stats | `online-count` (admin prefix) |

Most room routes require `Protected` + `EmailVerified`.

---

## Admin endpoints (summary)

All require `superadmin`. Prefix `/api/admin`.

| Group | Endpoints |
|-------|-----------|
| Users | list, recent, detail, status, accesses, force-logout, verify, sessions, delete |
| Rooms | list, events, close, suspend, reactivate, update, token, participants, kick, mute, bulk ops |
| System | overview, queue stats, settings, test-email, validate connectivity |
| Tokens | invite-tokens CRUD |
| Webhooks | webhooks CRUD, rotate-secret, test |
| LiveKit | stats, online-count |
| TLS | cert-info |

Public: `GET /api/cert` (PEM).

---

## LiveKit webhook

`POST /api/livekit/webhook` — LiveKit JWT auth (not app middleware). Handles participant disconnect and egress events.

---

## Async operations (202 Accepted)

| Action | Job type |
|--------|----------|
| User delete | `user_delete` |
| Room delete / admin close | `room_delete` |
| Room suspend | `room_suspend` |
| Bulk room ops | per-room jobs |
| S3 chat upload (large) | `chat_upload_s3` |
| Transactional email | `send_email` |
| Outbound webhook | `dispatch_webhook` |

---

## Planned endpoints

Recording start/stop/list/download and admin recording management are implemented in handler code but **not registered** in `server.go`. See [planned-features.md](./planned-features.md).

---

## Key DTOs

### LoginResponse

```json
{
  "user": { "id", "email", "name", "provider", "avatarUrl", "emailVerifiedAt" },
  "tokens": { "accessToken", "refreshToken" }
}
```

### RoomSettings

```json
{
  "allowChat": true,
  "allowVideo": true,
  "allowAudio": true,
  "requireApproval": false,
  "e2ee": false,
  "isPersistent": false
}
```

### OverviewResponse

Returned by `GET /api/admin/overview`. Includes health, KPIs, 7-day trend, room composition, attention items, recent activity. See `models/stats.go`.

For complete request/response shapes and status codes, see `.agents/skills/bedrud-api/SKILL.md` or Swagger.