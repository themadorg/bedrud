# Debugging Guide

Common development issues and how to fix them.

---

## Server won't start

### Config file not found

```
configuration file not found: config.yaml
```

```bash
cp server/config.local.yaml.example server/config.yaml
# or
CONFIG_PATH=server/config.local.yaml make dev-api
```

### JWT / session secret missing

```
jwtSecret is required
```

Set `auth.jwtSecret` and `auth.sessionSecret` in config (32+ chars).

### LiveKit API secret too short

```
LiveKit API secret is too short (N chars, need at least 32)
```

```bash
openssl rand -hex 32   # set as livekit.apiSecret
```

### CORS misconfiguration

```
CORS misconfiguration: allowCredentials=true with wildcard origins
```

Set explicit `cors.allowedOrigins` (e.g. `http://localhost:3000`) when `allowCredentials: true`.

### Port already in use

```bash
# Find process on 8090
ss -tlnp | grep 8090
# or change port
SERVER_PORT=8091 CONFIG_PATH=config.local.yaml make dev-api
```

### Cannot bind port 80 (non-root)

Set `server.httpPort: "8080"` in config or `SERVER_HTTP_PORT=8080`.

---

## Authentication issues

### 401 Missing authorization

- Check `Authorization: Bearer <token>` header or `access_token` cookie
- Token may be expired — call `POST /api/auth/refresh`

### 403 Email not verified

- `requireEmailVerification: true` in config
- Verify via email link or `go run ./cmd/bedrud user promote` won't help — use admin verify or `POST /api/auth/verify`

### 403 Account is deactivated

- User `IsActive = false` — `go run ./cmd/bedrud user enable --email ...`

### OAuth redirect fails

- Check `auth.*.redirectUrl` matches provider console
- `auth.frontendURL` must match where browser lands after callback

---

## LiveKit / WebRTC

### Cannot connect to room

1. Is embedded LiveKit running? `make dev-livekit` or `make dev-server`
2. Check `livekit.host` — dev default: `http://localhost:8090/livekit`
3. Check `livekit.internalHost`: `http://127.0.0.1:7880`
4. For external LK: set `livekit.external: true` and configure webhook manually

### Webhook not firing

Embedded LK auto-configures webhook to `http://localhost:<port>/api/livekit/webhook`.

For external LiveKit, set webhook URL in LiveKit dashboard to `https://<domain>/api/livekit/webhook`.

---

## Database

### Migrations not applied

```bash
go run ./cmd/bedrud db migrate
go run ./cmd/bedrud db status
```

### SQLite locked / queue jobs stuck

SQLite serializes writes. Symptoms: jobs stay `pending`.

- Reduce `queue.concurrency` to 1 (default)
- Consider PostgreSQL for production
- Check `QUEUE_POLL_INTERVAL` isn't too aggressive under load

### Reset local SQLite DB

```bash
rm server/bedrud-local.db
make dev-api   # recreates on startup
```

---

## Queue / async jobs

### Jobs stay pending

```bash
# Check queue stats (as superadmin)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8090/api/admin/queue
```

- Worker starts in `server.go` — check logs for worker errors
- Failed jobs: check `last_error` column in `jobs` table

### Faster polling for debugging

```bash
QUEUE_POLL_INTERVAL=100 make dev-api
```

---

## Frontend dev

### API calls fail from :3000

- Backend must run on :8090
- Web dev server proxies `/api` — check Vite/TanStack proxy config
- CORS: add `http://localhost:3000` to `cors.allowedOrigins`

### Blank page after `make build`

Frontend not embedded. Run `make build-embed` or full `make build`.

### routeTree.gen.ts out of date

Restart `bun run dev` — file auto-regenerates.

---

## Logs

Server uses Zerolog to stdout. Set verbose logging:

```yaml
logger:
  level: "debug"
```

SQL query logs appear at `debug` level.

---

## Useful debug commands

```bash
# Health
curl http://localhost:8090/api/health
curl http://localhost:8090/api/ready

# Public settings
curl http://localhost:8090/api/auth/settings

# Config validation
go run ./cmd/bedrud config validate

# LiveKit stats (needs superadmin token)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8090/api/admin/livekit/stats
```