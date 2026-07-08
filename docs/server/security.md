# Security

Security mechanisms in the Bedrud server.

**Public docs:** [Authentication](https://bedrud.org/en/docs/backend/authentication) · [User Roles](https://bedrud.org/en/docs/guides/roles) · [Behind a Proxy/CDN](https://bedrud.org/en/docs/guides/behind-proxy) · [Internal TLS](https://bedrud.org/en/docs/guides/internal-tls). Full map: [public-docs.md](./public-docs.md).

---

## Authentication

| Mechanism | Details |
|-----------|---------|
| Password hashing | bcrypt via `golang.org/x/crypto` |
| JWT | HMAC-SHA256, issuer/audience `bedrud`, configurable access TTL |
| Refresh rotation | Old refresh blocked in DB on `POST /auth/refresh` |
| Access revocation | In-memory hash set on logout (lost on restart) |
| Passkeys | WebAuthn via `go-passkeys`, counter replay protection |
| OAuth | Goth providers, session secret for CSRF state |

---

## Authorization

- Hierarchical RBAC: `superadmin` > `admin` > `moderator` > `user`
- Room-level checks in `handlers/room_auth.go` (creator, room admin, global admin)
- Guest accounts restricted (`RejectGuest` middleware available)
- Deactivated users blocked via in-memory ban set + `IsActive` DB field

---

## Email verification gate

When enabled, `RequireEmailVerified` checks DB on every protected request (not JWT claim alone). Prevents:

- Unverified users accessing rooms/admin
- Stale JWT bypass after admin un-verifies account

---

## Rate limiting

Per-IP sliding window on auth, guest, resend, and API endpoints. **Must configure `behindProxy` or `trustedProxies`** when behind a reverse proxy — otherwise all users share the proxy IP bucket.

---

## CSRF considerations

- `RequireBearerForMutations()` available for cookie-only mutation rejection
- OAuth uses Gorilla session store with secure cookie flags when TLS enabled
- CORS: `allowCredentials=true` refuses wildcard origins at startup (fail-closed)

---

## Input validation

| Area | Protection |
|------|------------|
| Request body | 2 MB Fiber body limit |
| Room names | `ValidateRoomName()` — slug rules, length 3–63 |
| Chat uploads | MIME whitelist (png/jpeg/gif/webp), size cap, SHA-256 filenames |
| Chat file serve | Path traversal check on `/uploads/chat/*` |
| Preferences JSON | Max 4 KB, valid JSON |
| LiveKit webhook | LiveKit JWT signature validation |

---

## TLS

| Mode | Notes |
|------|-------|
| Manual | Cert/key pair validated at startup |
| Self-signed | Ed25519 default, auto-renewal at 30 days |
| ACME | Let's Encrypt, HTTP-01 on :80, HTTPS on :443 |
| Embedded LK TURN/TLS | Port 5349, uses server cert when TLS enabled |

---

## Secrets management

- `config.yaml` secrets never committed (use `.example` templates)
- Admin API masks secrets in `GET /admin/settings` (`******`)
- LiveKit API secret minimum 32 chars enforced at embedded LK startup
- JWT secret minimum 32 chars recommended (warns if shorter)

---

## Webhook security

Outbound webhooks (`dispatch_webhook` job) support HMAC signing with per-endpoint secret. Admin can rotate secrets without deleting endpoint.

Inbound LiveKit webhooks validated with LiveKit API key/secret JWT — no app auth middleware.