# Settings System

How Bedrud merges `config.yaml` with database `SystemSettings` — and what admins can change at runtime.

**Model:** `server/internal/models/settings.go`  
**Repository:** `server/internal/repository/settings_repository.go`  
**Admin API:** `server/internal/handlers/admin_handler.go`

**Public docs:** [Configuration](https://bedrud.org/en/docs/getting-started/configuration) · [Admin Dashboard](https://bedrud.org/en/docs/guides/admin-dashboard) — [`getting-started/configuration.mdx`](../../apps/site/src/content/docs/en/getting-started/configuration.mdx). Full map: [public-docs.md](./public-docs.md).

---

## Two-layer configuration

```
┌─────────────────────┐     ┌──────────────────────┐
│   config.yaml       │     │  system_settings     │
│   (file, startup)   │     │  (DB singleton id=1) │
│                     │     │                      │
│  Secrets, defaults  │────►│  Admin overrides     │
│  Env var overrides  │     │  Runtime toggles     │
└─────────────────────┘     └──────────────────────┘
              │                         │
              └───────────┬─────────────┘
                          ▼
              GetEffectiveSettings()
              (merged view for handlers)
```

### When each layer wins

| Scenario | Winner |
|----------|--------|
| String/secret empty in DB | `config.yaml` value used |
| String/secret set in DB | DB value used |
| Boolean toggles (registration, guest login) | DB value (explicit) |
| Some booleans (TLS) | **Known limitation** — config can override DB `false` |

The `mergeFromConfig` function fills zero/empty DB fields from config. Comment in code notes that boolean merge can surprise operators who set `serverEnableTls=false` in admin but have `enableTls: true` in config.

---

## SystemSettings fields (grouped)

### Registration and auth

| Field | Default | Purpose |
|-------|---------|---------|
| `registrationEnabled` | true | Allow new signups |
| `tokenRegistrationOnly` | false | Require invite token |
| `passkeysEnabled` | true | WebAuthn endpoints |
| `guestLoginEnabled` | true | Guest login endpoint |

### OAuth providers

Per provider (Google, GitHub, Twitter):

- `*ClientID`, `*ClientSecret`, `*RedirectURL`

Merged from config when DB fields empty. Goth re-initialized when admin updates settings.

### JWT and sessions

| Field | Notes |
|-------|-------|
| `jwtSecret` | Falls back to config; required at startup from config even if DB empty |
| `sessionSecret` | OAuth/passkey sessions |
| `tokenDuration` | Access token hours |
| `frontendUrl` | OAuth redirect target |

### Server and instance

| Field | Purpose |
|-------|---------|
| `serverPort`, `serverHost`, `serverDomain` | Public URL construction |
| `serverEnableTls`, `serverCertFile`, `serverKeyFile`, `serverUseAcme` | TLS (mostly config-driven at startup) |
| `behindProxy` | Trusted proxy / rate limit IP |
| `serverName` | Instance display name |

### LiveKit

| Field | Purpose |
|-------|---------|
| `livekitHost` | WS URL for clients |
| `livekitApiKey`, `livekitApiSecret` | Token signing |
| `livekitExternal` | Skip embedded server |

### CORS

`corsAllowedOrigins`, `corsAllowedHeaders`, `corsAllowedMethods`, `corsAllowCredentials`, `corsMaxAge`

Admin can update; Fiber CORS middleware reads from **startup config** — some CORS changes may require restart.

### Chat uploads

| Field | Purpose |
|-------|---------|
| `chatUploadBackend` | `disk`, `inline`, `s3` |
| `chatUploadMaxBytes` | Per-file limit |
| `chatUploadInlineMax` | Max size for base64 inline |
| `chatUploadDiskDir` | Local storage path |
| S3 fields | Endpoint, bucket, region, keys, public URL |

### Quotas

| Field | Default | Purpose |
|-------|---------|---------|
| `maxParticipantsLimit` | 1000 | Cap per room |
| `maxRoomsPerUser` | 100 | Rooms per user |
| `maxUploadBytesPerUser` | 500 MB | Chat upload quota |
| `globalDiskThresholdBytes` | 0 | Instance-wide disk cap (0 = unlimited) |

### Chat retention (advisory)

| Field | Default | Purpose |
|-------|---------|---------|
| `chatMaxMessageCount` | 10000 | Client-side trim limit |
| `chatMessageTTLHours` | 2160 (90d) | Client-side TTL |

These are returned to frontend via public settings — not enforced server-side on data channel messages.

### Recordings (planned)

`recordingsEnabled`, `recordingMaxDurationMins`, `recordingMaxFileSizeMB` — model exists; feature not wired.

### Email branding

See [email-webhooks.md](./email-webhooks.md) for full list of `email*` fields.

### SMTP (in settings model)

SMTP host/port/credentials can also be stored in DB for admin editing. Handler merge applies same empty→config fallback.

---

## Repository API

```go
// Raw DB row (creates id=1 if missing)
settingsRepo.GetSettings()

// DB + config.yaml merge
settingsRepo.GetEffectiveSettings()

// Admin save (always id=1)
settingsRepo.SaveSettings(s)

// Attach config for merge
settingsRepo.SetConfig(cfg)  // called once in server.go
```

---

## Admin endpoints

| Endpoint | Access | Purpose |
|----------|--------|---------|
| `GET /api/admin/settings` | superadmin | Full settings (includes secrets) |
| `PUT /api/admin/settings` | superadmin | Update any field |
| `POST /api/admin/settings/validate` | superadmin | Test OAuth/SMTP/LiveKit connectivity |
| `GET /api/settings/public` | public | Safe subset for frontend (no secrets) |

`GetPublicSettings` strips secrets and returns only what the web app needs: registration flags, OAuth client IDs (not secrets), LiveKit host, upload limits, branding, etc.

---

## What requires restart

| Change | Restart needed? |
|--------|-----------------|
| Registration toggle | No |
| OAuth credentials | Partial — Goth may need re-init (handler does this on save) |
| JWT secret | Yes — invalidates all tokens |
| HTTP port / TLS | Yes — listener configured at startup |
| CORS origins | Possibly — middleware wired at startup |
| LiveKit external toggle | Yes — subprocess started at boot |
| Queue poll interval | Yes — worker options set at startup |

---

## Config-only settings (not in DB)

Some `config.yaml` sections have **no** `SystemSettings` counterpart:

- `rateLimit` — rate limiter thresholds
- `auth.requireEmailVerification` — enforced in middleware at startup
- `auth.verificationEmailCooldownMins`
- `queue` — worker poll interval, concurrency
- `logger.level`

These are env/config only. See [configuration.md](./configuration.md).

---

## Operator workflow

1. **First boot:** Copy `config.local.yaml` → `config.yaml`, set secrets
2. **Production:** Set `AUTH_JWT_SECRET`, `AUTH_SESSION_SECRET`, database URL via env
3. **Runtime tuning:** Use admin UI → writes `system_settings` id=1
4. **Validate:** `POST /admin/settings/validate` before going live
5. **Public frontend:** Reads `GET /api/settings/public` on load

---

## Related docs

- [configuration.md](./configuration.md) — full config.yaml reference
- [auth-flows.md](./auth-flows.md) — how settings gate registration/OAuth
- [email-webhooks.md](./email-webhooks.md) — email branding fields
- [database-schema.md](./database-schema.md) — `system_settings` table