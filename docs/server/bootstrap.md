# Server Bootstrap

Production startup is handled by `internal/server/server.go` via `Run(configPath, version)`. Both `bedrud run` and the dev `cmd/server` follow the same initialization sequence.

**Public docs:** [Deployment](https://bedrud.org/en/docs/backend/deployment) · [Deployment Guide](https://bedrud.org/en/docs/guides/deployment) · [Server Installation](https://bedrud.org/en/docs/getting-started/installation). Full map: [public-docs.md](./public-docs.md).

---

## Startup Sequence

```
1.  Load config (config.Load)
2.  Configure Zerolog (level, console output)
3.  Validate required secrets (jwtSecret, sessionSecret)
4.  Warn if email verification enabled without SMTP
5.  Validate TLS cert pair (if TLS enabled, non-ACME)
6.  Start embedded LiveKit (unless external mode)
7.  Init session store (auth.InitializeSessionStore)
8.  Init database (database.Initialize)
9.  Run migrations (database.RunMigrations)
10. Init scheduler (scheduler.Initialize) — needs DB for job cleanup
11. Init auth providers (auth.Service.Init — Goth OAuth)
12. Init all repositories
13. Init cleanup service (RoomCleanupService)
14. Init queue worker (queue.NewWorker.Start)
15. Init auth service, challenge store, handlers
16. Create Fiber app with global middleware
17. Register API routes
18. Setup LiveKit reverse proxy at /livekit
19. Setup TLS (self-signed / manual / ACME)
20. Serve embedded SPA (ui.go embed.FS)
21. Start HTTP + HTTPS listeners
22. Graceful shutdown on SIGINT/SIGTERM
```

---

## Fiber App Setup

### Global middleware (all routes)

| Order | Middleware | Purpose |
|-------|-----------|---------|
| 1 | `recover.New()` | Panic recovery |
| 2 | `helmet.New()` | XSS, nosniff, X-Frame DENY |
| 3 | `cors.New()` | Config-driven CORS |
| 4 | Body limit 2MB | Request size cap |

### Route groups

| Prefix | Middleware | Handlers |
|--------|-----------|----------|
| `/api/auth/*` | AuthRateLimiter | auth_handler, auth (OAuth) |
| `/api/rooms/*` | Protected, APIRateLimiter | room |
| `/api/admin/*` | Protected, RequireAccess(admin/superadmin) | admin, users, queue, overview |
| `/api/livekit/webhook` | LiveKit JWT validation | livekit_webhook |
| `/livekit/*` | — | Reverse proxy to internal LK |
| `/*` (non-API) | — | Embedded SPA (index.html / shell.html) |

---

## LiveKit Integration at Startup

### Embedded mode (default)

1. `livekit.StartInternalServer()` runs in background goroutine
2. Extracts `bin/livekit-server` to temp path
3. Generates temp YAML with TURN/TLS when server TLS enabled
4. 3-second startup sleep before marking ready

### External mode (`livekit.external: true`)

- Skips embedded server startup
- Skips `/livekit` reverse proxy
- Handlers connect to `livekit.internalHost` directly

### Reverse proxy

Requests to `/livekit/*` are proxied to `livekit.internalHost` (default `http://127.0.0.1:7880`). The `/livekit` prefix is stripped before forwarding.

---

## TLS Modes

| Mode | Trigger | Behavior |
|------|---------|----------|
| None | `enableTLS: false` | HTTP only on `server.port` |
| Manual | `enableTLS: true`, cert/key files | HTTPS on `server.port`, HTTP redirect on `httpPort` |
| Self-signed | Install or cert generation | Ed25519 default, auto-renewal at 30 days |
| ACME | `useACME: true` + `domain` + `email` | Let's Encrypt via autocert |

When TLS is enabled, an additional HTTP listener on `httpPort` (default `:80`) redirects to HTTPS. Non-root users should set `httpPort: "8080"`.

---

## SPA Serving

Embedded frontend from `ui.go`:

- Static assets served via Fiber `filesystem` middleware from `embed.FS`
- `/` → `index.html`
- Non-API routes without file match → `shell.html` (client-side routing)
- API routes (`/api/*`) handled by Fiber handlers, not SPA

---

## Trusted proxy & rate limiting

Fiber `EnableTrustedProxyCheck` when `trustedProxies` set or `behindProxy: true`. Uses `X-Forwarded-For` (or `proxyHeader`) for client IP — critical for rate limiters behind nginx/Cloudflare.

Startup warns if rate limiting active without proxy config.

## Banned users preload

After DB init:

```go
inactiveUsers, _ := userRepo.GetInactiveUserIDs()
auth.LoadBannedUsersFromDB(inactiveUsers)
```

Ensures deactivated users cannot authenticate even before first admin action in this process.

## SPA serving (index vs shell)

- `/` → `frontend/index.html` (SSR homepage for SEO)
- All other non-API paths → `frontend/shell.html` (empty route shell, avoids homepage flash on `/dashboard/*`, `/m/*`)
- Falls back to `index.html` if `shell.html` missing

## ACME (Let's Encrypt)

When `useACME: true` + `domain` set:

1. HTTP-01 challenge server on `:80`
2. HTTPS listener on `:443` via `autocert.Manager`
3. Cert cache: `/var/lib/bedrud/certs`

Falls back to manual TLS if `:443` bind fails.

## Graceful Shutdown

On `SIGINT` / `SIGTERM`:

1. `app.Shutdown()` — stop accepting connections
2. Queue worker stopped via `defer queueWorker.Stop()`
3. Scheduler stopped via `defer scheduler.Stop()`
4. Database closed via `defer database.Close()`
5. Embedded LiveKit process terminates with parent

---

## Key Dependencies Wired at Bootstrap

```
server.Run()
├── config.Load
├── livekit.StartInternalServer (or skip)
├── auth.InitializeSessionStore
├── database.Initialize + RunMigrations
├── scheduler.Initialize(db, roomRepo, lkCfg)
├── auth.NewAuthService + Init (Goth)
├── repository.New* (8 repos)
├── services.NewRoomCleanupService
├── services.NewRecordingService
├── storage.NewChatUploadStore + ChatUploadTracker
├── queue.NewWorker(handlers).Start(ctx)
├── handlers.New* (auth, room, users, admin, preferences, recording, webhook)
└── fiber.New + route registration + Listen
```