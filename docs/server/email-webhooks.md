# Email and Webhooks

Transactional email (SMTP) and outbound webhooks in the Bedrud backend.

**Email queue handler:** `server/internal/queue/handler_email.go`  
**Webhook queue handler:** `server/internal/queue/handler_dispatch_webhook.go`  
**Webhook admin CRUD:** `server/internal/handlers/admin_handler.go`  
**Webhook model:** `server/internal/models/webhook.go`

**Public docs:** [Webhooks Guide](https://bedrud.org/en/docs/guides/webhooks) (admin UI, event table, screenshots) — [`guides/webhooks.mdx`](../../apps/site/src/content/docs/en/guides/webhooks.mdx). Full map: [public-docs.md](./public-docs.md).

---

## Transactional email

### Architecture

Emails are **never sent synchronously** from HTTP handlers. All sends go through the `send_email` queue job:

```
HTTP handler → queue.Enqueue("send_email", payload) → Worker → SMTP
```

This keeps API responses fast and isolates SMTP failures from user-facing operations.

### SMTP configuration

Primary source: `config.yaml` `email` section.

| Field | Env var | Purpose |
|-------|---------|---------|
| `smtpHost` | `EMAIL_SMTP_HOST` | SMTP server |
| `smtpPort` | `EMAIL_SMTP_PORT` | Usually 587 or 465 |
| `smtpUser` | `EMAIL_SMTP_USER` | Auth username |
| `smtpPassword` | `EMAIL_SMTP_PASSWORD` | Auth password |
| `fromAddress` | `EMAIL_FROM_ADDRESS` | From header |
| `fromName` | `EMAIL_FROM_NAME` | Display name |
| `useTLS` | — | STARTTLS |
| `useSSL` | — | Implicit TLS |

**No SMTP configured:** Handler logs the email content and returns success. Useful for dev; production should configure SMTP before enabling email verification.

### Templates

Embedded via `//go:embed templates/*.html templates/*.txt` in `handler_email.go`:

| Template | Trigger |
|----------|---------|
| `verify_email` | Registration, resend verification |
| `welcome` | Post-verification welcome |
| `password_reset` | Forgot password |
| `password_changed` | Password change confirmation |
| `room_invite` | Room invitations |
| `generic` | Fallback |

Templates are HTML + plain-text multipart. Branding injected at render time.

### Branding and subject overrides

`SystemSettings` (admin UI) controls:

- `emailInstanceName`, `emailSupportEmail`, `emailInstanceUrl`
- `emailHeaderBg`, `emailButtonBg` — hex colors for Cerberus-style templates
- Per-template subject overrides: `emailSubjectVerify`, `emailSubjectWelcome`, etc.
- Per-template preheader overrides: `emailPreheaderVerify`, etc.

Resolution order for subject lines:

1. DB subject override (if set)
2. `config.yaml` email subject
3. Payload `subject` field
4. Hardcoded default in handler

### Enqueueing from handlers

```go
queue.Enqueue(ctx, db, "send_email", queue.SendEmailPayload{
    To:           user.Email,
    TemplateName: "verify_email",
    TemplateData: map[string]any{
        "name":      user.Name,
        "verifyUrl": verifyURL,
    },
})
```

### Admin test email

`POST /api/admin/settings/test-email` — sends a test message using current SMTP and branding settings. Requires superadmin.

### Verification cooldown

`POST /auth/verify/resend` uses:

- `ResendRateLimiter` (IP-based)
- `CooldownCache` (per-email, default 2 min from `auth.verificationEmailCooldownMins`)

Both must pass before a new `send_email` job is enqueued.

### Audit trail

`verification_events` table records email-related actions for admin review. See [auth-flows.md](./auth-flows.md).

---

## Outbound webhooks

### Purpose

Notify external systems of Bedrud events (room created, participant joined, user registered, etc.). Webhooks are **advisory** — failures do not roll back the triggering operation.

### Configuration (admin)

| Endpoint | Action |
|----------|--------|
| `GET /admin/webhooks` | List configured webhooks |
| `POST /admin/webhooks` | Create webhook (URL, events, secret) |
| `PUT /admin/webhooks/:id` | Update URL, events, enabled flag |
| `DELETE /admin/webhooks/:id` | Remove webhook |
| `POST /admin/webhooks/:id/rotate-secret` | Generate new HMAC secret |
| `POST /admin/webhooks/:id/test` | Send test payload |

Webhook model stores: URL, secret, subscribed events array, `isActive`.

### Dispatch flow

```
RoomHandler.dispatchRoomEvent()
  ├─ Load active webhooks matching event type
  └─ For each: queue.Enqueue("dispatch_webhook", WebhookPayload)
```

Worker delivers via HTTP POST with 10-second timeout.

### Payload envelope

```json
{
  "event": "room.participant_joined",
  "timestamp": "2026-06-16T12:00:00Z",
  "data": {
    "roomId": "...",
    "roomName": "...",
    "userId": "..."
  }
}
```

### Signature verification (consumer side)

```
X-Bedrud-Signature: sha256=<hex>
X-Bedrud-Event: room.participant_joined
X-Bedrud-Timestamp: 2026-06-16T12:00:00Z
```

Verify:

```go
mac := hmac.New(sha256.New, []byte(webhookSecret))
mac.Write(requestBody)
expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
```

Compare constant-time with `X-Bedrud-Signature`.

### Failure behavior

| Failure | Worker action |
|---------|---------------|
| Network error | Log warning, return nil (no retry) |
| Non-2xx response | Log warning, return nil |
| Invalid URL | Log warning, return nil |
| JSON marshal error | Log warning, return nil |

**Rationale:** Room delete must succeed even if your Slack webhook is down.

### Common events

Dispatched from `RoomHandler.dispatchRoomEvent` and auth handlers:

- `room.created`, `room.deleted`, `room.suspended`
- `room.participant_joined`, `room.participant_left`
- `user.registered` (when configured)

Exact event list depends on webhook subscription filters in admin UI.

---

## LiveKit webhooks (inbound)

Separate from outbound webhooks — these are **incoming** from LiveKit server:

`POST /api/livekit/webhook`

Handles `participant_left`, `room_finished`, egress events. See [room-lifecycle.md](./room-lifecycle.md).

---

## Related docs

- [queue-deep-dive.md](./queue-deep-dive.md) — job handlers in detail
- [settings-system.md](./settings-system.md) — email branding in SystemSettings
- [auth-flows.md](./auth-flows.md) — verification email flow
- [internal/queue.md](./internal/queue.md) — queue package