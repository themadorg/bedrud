# Configuration

Bedrud reads settings from a YAML config file with environment variable overrides. The config package lives in `server/config/`.

**Public docs:** [Configuration](https://bedrud.org/en/docs/getting-started/configuration) — full operator reference with examples — [`getting-started/configuration.mdx`](../../apps/site/src/content/docs/en/getting-started/configuration.mdx). Runtime DB overrides: [settings-system.md](./settings-system.md). Full map: [public-docs.md](./public-docs.md).

---

## Config File

**Dev template:** `server/config.local.yaml.example`  
**Runtime path:** `config.yaml` (or override via `--config`, `CONFIG_PATH`, `BEDRUD_CONFIG`)

```bash
cp server/config.local.yaml.example server/config.yaml
# Edit secrets before running
```

`config.Load(path)` reads the file once (singleton via `sync.Once`). Use `config.Get()` after load; `config.GetSafe()` returns nil if not loaded.

---

## Config Sections

### `server`

HTTP listener, TLS, proxy, and capacity limits.

| Field | Default | Env | Description |
|-------|---------|-----|-------------|
| `port` | `8090` | `SERVER_PORT` | Main listener port (HTTPS when TLS enabled) |
| `httpPort` | `80` | `SERVER_HTTP_PORT` | HTTP redirect listener when TLS on. Use `8080` for non-root |
| `host` | `0.0.0.0` | — | Bind address |
| `readTimeout` | `30` | — | Read timeout (seconds) |
| `writeTimeout` | `30` | — | Write timeout (seconds) |
| `enableTLS` | `false` | `SERVER_ENABLE_TLS` | Enable HTTPS |
| `disableTLS` | `false` | — | Force-disable TLS even if `enableTLS` set |
| `certFile` | `/etc/bedrud/cert.pem` | `SERVER_CERT_FILE` | TLS certificate path |
| `keyFile` | `/etc/bedrud/key.pem` | `SERVER_KEY_FILE` | TLS private key path |
| `domain` | — | `SERVER_DOMAIN` | Production domain (ACME, passkey RP ID) |
| `email` | — | `SERVER_EMAIL` | ACME registration email |
| `useACME` | `false` | `SERVER_USE_ACME` | Let's Encrypt auto-cert |
| `trustedProxies` | — | `SERVER_TRUSTED_PROXIES` | Comma-separated proxy IPs |
| `proxyHeader` | — | — | Header for client IP behind proxy |
| `behindProxy` | `false` | — | Trusted-proxy mode (Cloudflare, nginx) |
| `certAlgorithm` | `ed25519` | `SERVER_CERT_ALGORITHM` | Self-signed key algo: `ed25519`, `ecdsa256`, `rsa2048`, `rsa4096` |
| `maxParticipantsLimit` | `1000` | `SERVER_MAX_PARTICIPANTS_LIMIT` | Hard ceiling for room capacity (0 = unlimited) |
| `maxRoomsPerUser` | `100` | `SERVER_MAX_ROOMS_PER_USER` | Active rooms per user (0 = unlimited) |

### `database`

| Field | Env | Description |
|-------|-----|-------------|
| `type` | `DB_TYPE` | `sqlite` or `postgres` |
| `path` | `DB_PATH` | SQLite file path |
| `host` | `DB_HOST` | PostgreSQL host |
| `port` | `DB_PORT` | PostgreSQL port |
| `user` | `DB_USER` | PostgreSQL user |
| `password` | `DB_PASSWORD` | PostgreSQL password |
| `dbname` | `DB_NAME` | PostgreSQL database name |
| `sslmode` | — | PostgreSQL SSL mode |
| `maxIdleConns` | — | Connection pool idle |
| `maxOpenConns` | — | Connection pool max |
| `maxLifetime` | — | Connection max lifetime (minutes) |

### `livekit`

| Field | Env | Description |
|-------|-----|-------------|
| `host` | `LIVEKIT_HOST` | Public LiveKit URL (e.g. `https://meet.example.com/livekit`) |
| `internalHost` | `LIVEKIT_INTERNAL_HOST` | Internal URL (default `http://127.0.0.1:7880`) |
| `apiKey` | `LIVEKIT_API_KEY` | LiveKit API key |
| `apiSecret` | `LIVEKIT_API_SECRET` | LiveKit API secret |
| `configPath` | `LIVEKIT_CONFIG_PATH` | External LiveKit YAML path |
| `skipTLSVerify` | — | Skip TLS verify for LK client |
| `external` | — | Skip embedded LiveKit + `/livekit` proxy |
| `nodeIP` | `LIVEKIT_NODE_IP` | Explicit RTC node IP (disables STUN) |

**Webhook:** Embedded LiveKit auto-configures webhook to `http://localhost:<port>/api/livekit/webhook`. For external LiveKit, configure manually in the LiveKit dashboard.

### `auth`

| Field | Env | Description |
|-------|-----|-------------|
| `jwtSecret` | `JWT_SECRET` | HMAC-SHA256 signing secret (min 32 chars recommended) |
| `sessionSecret` | — | Gorilla session store secret for OAuth |
| `tokenDuration` | — | Access token lifetime (hours) |
| `frontendURL` | `AUTH_FRONTEND_URL` | Frontend base URL for redirects |
| `passkeyChallengeTTL` | `AUTH_PASSKEY_CHALLENGE_TTL` | WebAuthn challenge expiry (minutes) |
| `requireEmailVerification` | `AUTH_REQUIRE_EMAIL_VERIFICATION` | Gate local auth behind email verification |
| `verificationEmailCooldownMins` | `AUTH_VERIFICATION_COOLDOWN_MINS` | Min time between resend emails |
| `verificationTokenTTLHours` | — | Verification link validity |
| `unverifiedAccountTTLHours` | `AUTH_UNVERIFIED_ACCOUNT_TTL_HOURS` | Auto-delete unverified accounts (0 = disabled) |
| `resetTokenTTLHours` | `AUTH_RESET_TOKEN_TTL_HOURS` | Password reset link validity |
| `google` / `github` / `twitter` | — | OAuth2 `clientId`, `clientSecret`, `redirectUrl` |

### `logger`

| Field | Description |
|-------|-------------|
| `level` | `debug`, `info`, `warn`, `error` |
| `outputPath` | Empty = stdout |

### `cors`

| Field | Env |
|-------|-----|
| `allowedOrigins` | `CORS_ALLOWED_ORIGINS` |
| `allowedHeaders` | `CORS_ALLOWED_HEADERS` |
| `allowedMethods` | `CORS_ALLOWED_METHODS` |
| `allowCredentials` | `CORS_ALLOW_CREDENTIALS` |
| `exposeHeaders` | `CORS_EXPOSE_HEADERS` |
| `maxAge` | `CORS_MAX_AGE` |

### `chat`

| Field | Env | Description |
|-------|-----|-------------|
| `uploads.backend` | — | `disk`, `s3`, or `inline` |
| `uploads.maxBytes` | — | Hard upload size limit (default 10 MB) |
| `uploads.inlineMaxBytes` | — | Inline base64 threshold (default 500 KB) |
| `uploads.diskDir` | — | Disk storage path (default `./data/uploads/chat`) |
| `uploads.s3.*` | — | S3 endpoint, bucket, region, keys, publicBaseUrl |
| `maxUploadBytesPerUser` | `CHAT_MAX_UPLOAD_BYTES_PER_USER` | Per-user upload quota (default 500 MB) |
| `globalDiskThresholdBytes` | `CHAT_GLOBAL_DISK_THRESHOLD_BYTES` | Global disk ceiling |
| `maxMessageCount` | `CHAT_MAX_MESSAGE_COUNT` | Messages kept per room (default 10000) |
| `messageTTLHours` | `CHAT_MESSAGE_TTL_HOURS` | Message max age (default 2160 = 90 days) |

### `recording`

| Field | Env | Description |
|-------|-----|-------------|
| `maxFileSizeMB` | — | Per-file cap (default 2048) |
| `storageDir` | — | Disk path (default `./data/recordings`) |
| `maxRecordingsPerRoom` | — | Cap per room (non-persistent) |
| `retentionHours` | `RECORDING_RETENTION_HOURS` | Retention after archive (default 720 = 30 days) |
| `cleanupIntervalHours` | `RECORDING_CLEANUP_INTERVAL_HOURS` | Scheduler cleanup interval (default 24) |

### `queue`

| Field | Env | Default | Description |
|-------|-----|---------|-------------|
| `pollInterval` | `QUEUE_POLL_INTERVAL` | `500` | Poll interval (ms) |
| `maxAttempts` | `QUEUE_MAX_ATTEMPTS` | `3` | Max retries before failed |
| `concurrency` | `QUEUE_CONCURRENCY` | `1` | Worker goroutines |

### `email`

| Field | Env | Description |
|-------|-----|-------------|
| `smtpHost` | `EMAIL_SMTP_HOST` | SMTP server |
| `smtpPort` | `EMAIL_SMTP_PORT` | SMTP port |
| `username` | `EMAIL_USERNAME` | SMTP auth user |
| `password` | `EMAIL_PASSWORD` | SMTP auth password |
| `fromAddress` | `EMAIL_FROM_ADDRESS` | From email address |
| `fromName` | `EMAIL_FROM_NAME` | From display name |
| `tlsSkipVerify` | `EMAIL_TLS_SKIP_VERIFY` | Skip TLS cert validation |
| `smtpsMode` | `EMAIL_SMTPS_MODE` | Direct TLS (port 465) |
| `templates.*` | — | Branding: instanceName, colors, subject overrides |

### `rateLimit`

Pointer fields — nil uses defaults, `0` disables.

| Field | Env | Default |
|-------|-----|---------|
| `authMaxRequests` | `RATELIMIT_AUTH_MAX` | 10/min |
| `authWindowSecs` | `RATELIMIT_AUTH_WINDOW` | 60 |
| `authResendMaxRequests` | `RATELIMIT_AUTH_RESEND_MAX` | separate resend limit |
| `authResendWindowSecs` | `RATELIMIT_AUTH_RESEND_WINDOW` | — |
| `guestMaxRequests` | `RATELIMIT_GUEST_MAX` | 5/min |
| `guestWindowSecs` | `RATELIMIT_GUEST_WINDOW` | 60 |
| `apiMaxRequests` | `RATELIMIT_API_MAX` | general API limit |
| `apiWindowSecs` | `RATELIMIT_API_WINDOW` | — |

---

## `ConfigInt` Type

YAML fields that accept both bare integers and quoted strings (with a deprecation warning for strings):

```yaml
readTimeout: 30      # preferred
readTimeout: "30"    # works, logs warning
```

---

## Required Secrets

At minimum, production requires:

- `auth.jwtSecret` — non-empty, 32+ chars recommended
- `auth.sessionSecret` — non-empty (OAuth sessions)
- `livekit.apiKey` / `livekit.apiSecret` — LiveKit credentials

If `requireEmailVerification` is true, SMTP must be configured or emails are logged only.

---

## Dual config sources

Bedrud uses two configuration layers:

1. **`config.yaml`** (file + env overrides) — bootstrap secrets, defaults, infrastructure
2. **`SystemSettings`** (DB singleton ID=1) — runtime admin panel overrides for OAuth, SMTP, CORS, limits, branding

The admin panel `PUT /api/admin/settings` updates the DB. Many handlers read `settingsRepo.GetSettings()` which may override file config for OAuth client IDs, email SMTP, etc.

`settingsRepo.SetConfig(cfg)` links the file config for fallback resolution.

---

## Config files in repo

| File | Purpose |
|------|---------|
| `config.local.yaml.example` | Dev template (copy to `config.yaml`) |
| `config/livekit.yaml.example` | External LiveKit YAML reference |
| `.env.example` | Environment variable quick reference |

---

## External LiveKit Config

Example at `server/config/livekit.yaml.example`. Set `livekit.configPath` or `LIVEKIT_CONFIG_PATH`. When `livekit.external: true`, the embedded server and `/livekit` proxy are skipped.