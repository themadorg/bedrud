---
name: bedrud-email-cerberus
description: Cerberus hybrid HTML email templates — copy, customize, register, dark mode, Outlook compatibility.
license: Apache License
---

# Bedrud Cerberus Email Templates

Go module `bedrud`. Canonical upstream baseline:

`server/internal/queue/templates/cerberus-hybrid.html`

**Rule: Never edit `cerberus-hybrid.html`.** Always `cp` to a new file and edit the copy. Original is the Cerberus hybrid reference for re-merging upstream updates.

Production templates live beside it under `server/internal/queue/templates/` and are loaded by `handler_email.go` via `//go:embed`.

---

## Files

| File | Role |
|------|------|
| `server/internal/queue/templates/cerberus-hybrid.html` | Untouched Cerberus hybrid baseline (680px, hybrid grid, demo sections) |
| `welcome.html` + `.txt` | Welcome after register |
| `verify_email.html` + `.txt` | Email verification |
| `password_reset.html` + `.txt` | Password reset link |
| `password_changed.html` + `.txt` | Password-changed security notice |
| `room_invite.html` + `.txt` | Room invite (template registered; **not enqueued by any handler yet**) |
| `generic.html` + `.txt` | Fallback key/value dump for unknown template names |
| `server/internal/queue/handler_email.go` | Parse/register templates, branding, SMTP send |
| `server/internal/queue/job.go` | `SendEmailPayload` |
| `server/config/config.go` | `EmailConfig` / `EmailTemplateConfig` |
| `server/internal/models/settings.go` | Admin DB overrides for branding/subjects/preheaders |

Registered names (must match disk + parse loop):

```text
welcome | room_invite | password_reset | password_changed | verify_email | generic
```

---

## Go Template Engine — Not Fiber View

Uses **Go `html/template` directly** — not Fiber `c.Render()`.

### Load (`NewSendEmailHandler`)

```go
//go:embed templates/*.html templates/*.txt
var emailTemplatesFS embed.FS

for _, name := range []string{
    "welcome", "room_invite", "password_reset",
    "password_changed", "verify_email", "generic",
} {
    t, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".html")
    // on error: warn + skip HTML (falls back toward plaintext / generic)
    h.tmpls[name] = t

    pt, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".txt")
    if err == nil {
        h.plainTmpls[name] = pt
    }
}
```

### Render

```go
tmpl.Execute(&buf, payload.TemplateData) // map[string]any
```

### Job payload

```go
type SendEmailPayload struct {
    To           string         `json:"to"`
    Subject      string         `json:"subject"`
    TemplateName string         `json:"template_name"`
    TemplateData map[string]any `json:"template_data,omitempty"`
}
```

Job type: `"send_email"` (wired in `server/internal/server/server.go`).

### Data flow

1. Handler enqueues `SendEmailPayload` (`queue.Enqueue(..., "send_email", payload)`).
2. Worker runs `NewSendEmailHandler`.
3. `loadBranding(ctx, db, cfg)` merges defaults ← `config.yaml` `email.templates` ← `SystemSettings` row `1`.
4. `resolveSubject(branding, templateName, payload.Subject)`: DB/config subject map wins over payload subject.
5. `injectBranding(...)` fills missing keys on `TemplateData`: `InstanceName`, `SupportEmail`, `InstanceURL`, `HeaderBg`, `ButtonBg`, `Preheader`.
6. HTML from named template (or `generic` / `renderFallback`); multipart MIME via `utils.BuildMessage`.
7. If SMTP unset (`smtpHost` empty or port 0): log body + data, return nil (no error).

Caller-supplied `TemplateData` keys are **not** overwritten by branding.

---

## Who Enqueues What

| Template | Source | Typical `TemplateData` | Default `Subject` in enqueue |
|----------|--------|------------------------|------------------------------|
| `welcome` | `auth_handler.go` Register | `Name`, `LoginURL` | `"Welcome to Bedrud"` |
| `verify_email` | `auth_handler.go` `enqueueVerificationEmail`; `users.go` AdminResendVerification | `Name`, `VerifyURL` | `"Verify your Bedrud email"` / admin resend variant |
| `password_reset` | `auth_handler.go` `enqueuePasswordResetEmail` | `ResetURL`, `IPAddress` | `"Reset your Bedrud password"` |
| `password_changed` | `auth_handler.go` ChangePassword + ResetPassword success | `IPAddress` (optional `UserAgent` supported by template, not set by handlers today) | `"Your Bedrud password was changed"` |
| `room_invite` | **none yet** | expected: `InviterName`, `RoomName`, `JoinURL` | — |
| `generic` | fallback only | any keys (branding keys filtered out of table rows) | payload subject |

---

## Production Template Anatomy

All six production HTML templates are **simplified Cerberus shells** (not full demo layouts):

| Template | Sections used |
|----------|---------------|
| `welcome` | Preheader + Header pill + 1-Column Text+Button (`LoginURL`) + Footer |
| `verify_email` | Preheader + Header + 1-Column Text+Button (`VerifyURL`) + monospace URL fallback + Footer |
| `password_reset` | Preheader + Header + 1-Column Text+Button (`ResetURL`) + optional IP + Footer |
| `password_changed` | Preheader + Header + 1-Column Text (no button) + optional IP/UserAgent + Footer |
| `room_invite` | Preheader + Header + 1-Column Text+Button (`JoinURL`) + Footer |
| `generic` | Preheader + Header + 1-Column key/value table + Footer |

Shared production patterns:

- Max width **680px** + MSO fixed-width table.
- Header is **text pill** (`{{.InstanceName}}` + `.header-name-pill`), not a logo image.
- CTA uses `button-td` / `button-td-primary` + `{{.ButtonBg}}`.
- Optional fields wrapped in `{{if .Field}}…{{end}}`.
- Footer: contact / instance URL when set; no Cerberus webversion/unsubscribe demo copy.

---

## Cerberus Baseline Section Map

`cerberus-hybrid.html` section markers (line numbers approximate; search `<!-- … : BEGIN -->`):

| Section | ~Lines | Purpose | Typical use |
|---------|--------|---------|-------------|
| Visually Hidden Preheader | 256–260 | Inbox preview (static demo text in baseline) | Replace with `{{.Preheader}}` in copies |
| Preview Text Spacing Hack | 263–267 | Zero-width padding after preheader | Keep |
| Email Header | 284–290 | Logo / brand | Replace with InstanceName pill |
| Hero Image, Flush | 292–298 | Full-width bleed image | Announcements |
| 1 Column Text + Button | 300–332 | Body + CTA | Most transactional emails |
| Background Image with Text | 334–369 | VML + CSS bg image | Promotional |
| 2 Even Columns | 371–430 | Hybrid 2-col | Feature pairs |
| 3 Even Columns | 432–515 | Hybrid 3-col | Gallery / pricing |
| Thumbnail Left, Text Right | 517–571 | Image + text (`dir=rtl` swap) | Media + copy |
| Thumbnail Right, Text Left | 573–627 | Flipped alignment | Alternating rows |
| Clear Spacer | 629–635 | Vertical gap | Separation |
| 1 Column Text | 637–649 | Text only | Notices |
| Email Footer | 654–666 | Demo footer | Replace with SupportEmail/InstanceURL |
| Full Bleed Background | 675–701 | Full-width band outside container | Callouts |

Baseline preheader is **static Cerberus copy**, not `{{.Preheader}}`. Wire Go vars only in template **copies**.

---

## Branding Variables

Injected by `injectBranding()` when key absent. Priority: **DB `SystemSettings` > config.yaml > code defaults**.

### Core (auto-injected)

| Var | Config YAML | DB field | Default | Used |
|-----|-------------|---------|---------|------|
| `{{.InstanceName}}` | `email.templates.instanceName` | `EmailInstanceName` | `"Bedrud"` | Header, body |
| `{{.SupportEmail}}` | `email.templates.supportEmail` | `EmailSupportEmail` | `""` | Footer `mailto:` |
| `{{.InstanceURL}}` | `email.templates.instanceUrl` | `EmailInstanceURL` | `""` | Footer link |
| `{{.HeaderBg}}` | `email.templates.headerBgColor` | `EmailHeaderBg` | `"#1a1a2e"` | Header `td` bg |
| `{{.ButtonBg}}` | `email.templates.buttonBgColor` | `EmailButtonBg` | `"#e11d48"` | CTA button |
| `{{.Preheader}}` | `email.templates.preheaderText.<name>` | `EmailPreheader*` | only if configured | Hidden preview |

Code defaults in `loadBranding`: InstanceName=`Bedrud`, HeaderBg=`#1a1a2e`, ButtonBg=`#e11d48`. No hardcoded per-template preheaders/subjects — those come from config/DB or enqueue `Subject`.

### Per-template vars (enqueue only)

| Var | Templates | Notes |
|-----|-----------|-------|
| `{{.Name}}` | welcome, verify_email | Display name |
| `{{.LoginURL}}` | welcome | Optional; button omitted if empty/missing |
| `{{.VerifyURL}}` | verify_email | Button + monospace copy block |
| `{{.ResetURL}}` | password_reset | Reset CTA |
| `{{.IPAddress}}` | password_reset, password_changed | Optional security context |
| `{{.UserAgent}}` | password_changed | Optional; template-ready, handlers do not pass yet |
| `{{.InviterName}}` | room_invite | Who invited |
| `{{.RoomName}}` | room_invite | Optional room label |
| `{{.JoinURL}}` | room_invite | Join CTA (**not** `RoomURL`) |

### Subject resolution

`resolveSubject`: `branding.SubjectLines[templateName]` if non-empty, else `payload.Subject`.

Config keys / DB columns:

| Template | Config map key | DB subject field | DB preheader field |
|----------|----------------|------------------|--------------------|
| verify_email | `subjectLines.verify_email` | `EmailSubjectVerify` | `EmailPreheaderVerify` |
| welcome | `subjectLines.welcome` | `EmailSubjectWelcome` | `EmailPreheaderWelcome` |
| password_reset | `subjectLines.password_reset` | `EmailSubjectReset` | `EmailPreheaderReset` |
| password_changed | `subjectLines.password_changed` | `EmailSubjectChanged` | `EmailPreheaderChanged` |
| room_invite | `subjectLines.room_invite` | `EmailSubjectInvite` | `EmailPreheaderInvite` |

Example `config.yaml` (commented defaults often present):

```yaml
email:
  templates:
    instanceName: "Bedrud"
    supportEmail: ""
    instanceUrl: ""
    headerBgColor: "#1a1a2e"
    buttonBgColor: "#e11d48"
    subjectLines:
      verify_email: "Verify your Bedrud email"
      welcome: "Welcome to Bedrud"
      password_reset: "Reset your Bedrud password"
      password_changed: "Your Bedrud password was changed"
      room_invite: "You're invited to a room on Bedrud"
    preheaderText:
      verify_email: "Verify your email address to get started with Bedrud"
      # ...
```

---

## Dark Mode

Baseline and production copies use `@media (prefers-color-scheme: dark)` with **`!important`** to beat inline styles.

### Utility classes

| Class | Dark effect | Apply to |
|-------|-------------|----------|
| `.email-bg` | bg `#111` | `body` / `center` |
| `.darkmode-bg` | bg `#222` | Light content `<td>`s |
| `.darkmode-text` | text `#F7F7F9` | Body copy needing override |
| `.darkmode-fullbleed-bg` | bg `#0F3016` | Full-bleed bands (baseline) |
| `.header-name-pill` | bg `#2a2a4e` | **Bedrud production** header pill |
| `.button-td-primary` / `a` | invert to white bg / dark text | Keep class names |

Production dark CSS also recolors `h1–h3, p, li, .footer td`.

### Apply when customizing

```html
<td style="background-color:#ffffff;" class="darkmode-bg">
<p style="color:#555555;" class="darkmode-text">…</p>
```

Do not rename `button-td-primary` / `button-a-primary` — dark invert depends on them.

### Image swap (optional)

SVG unsupported in email. Toggle rasters with light/dark classes; Outlook ignores `prefers-color-scheme` and `[if !mso]` guards mean light logo is the safe default.

### Strip dark mode

Delete between `/* Dark Mode Styles : BEGIN */` … `END`, plus `color-scheme` meta/`:root`. Prefer keeping dark mode.

---

## Hybrid Grid (Baseline Only)

Cerberus hybrid: `inline-block` + MSO ghost tables. Production emails are single-column; use hybrid sections only when copying multi-col layouts from the baseline.

### 2-column sketch

```html
<!--[if mso]>
<table role="presentation" border="0" cellspacing="0" cellpadding="0" width="660">
<tr><td valign="top" width="330"><![endif]-->
<div style="display:inline-block; margin:0 -1px; width:100%; min-width:200px; max-width:330px; vertical-align:top;" class="stack-column">
  <!-- col 1 -->
</div>
<!--[if mso]></td><td valign="top" width="330"><![endif]-->
<div style="display:inline-block; margin:0 -1px; width:100%; min-width:200px; max-width:330px; vertical-align:top;" class="stack-column">
  <!-- col 2 -->
</div>
<!--[if mso]></td></tr></table><![endif]-->
```

| Layout | Per-col | Notes |
|--------|---------|-------|
| 2 columns | 330px | MSO table width 660 |
| 3 columns | 220px | MSO table width 660 |
| Thumb + text | 220 + 440 | `dir="rtl"` / `ltr` to swap |

Mobile (`max-width: 480px`): `.stack-column`, `.stack-column-center`, `.center-on-narrow`.

Gotchas: MSO widths must sum exactly; `margin: 0 -1px` kills inline-block gaps; update both MSO and div max-widths when changing columns.

---

## Register a New Template

### 1. Copy baseline (or a close production template)

```bash
cp server/internal/queue/templates/cerberus-hybrid.html \
   server/internal/queue/templates/{name}.html
# or start from welcome.html for a thinner transactional shell
```

Never edit `cerberus-hybrid.html`.

### 2. Customize copy

| Baseline placeholder | Replace with |
|----------------------|--------------|
| Long preheader paragraph | `{{.Preheader}}` |
| Logo / Company Name | Header pill `{{.InstanceName}}` + `{{.HeaderBg}}` |
| Demo body / `Praesent…` | Real copy + `{{.Var}}` |
| Button href / colors | `{{.CTAURL}}`, `{{.ButtonBg}}` |
| Footer address / webversion | `{{.SupportEmail}}`, `{{.InstanceURL}}` |

Minimal branding hooks (mirror existing templates):

```html
<div …>{{.Preheader}}</div>
<td style="background-color: {{.HeaderBg}};" class="darkmode-bg">
  <h1 class="header-name-pill">{{.InstanceName}}</h1>
</td>
<td class="button-td button-td-primary" style="background:{{.ButtonBg}};">
  <a class="button-a button-a-primary" href="{{.ActionURL}}"
     style="background:{{.ButtonBg}}; border:1px solid {{.ButtonBg}};">…</a>
</td>
{{if .SupportEmail}}<a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a>{{end}}
```

### 3. Plaintext twin

```bash
# recommended — all current templates ship .txt
# path: server/internal/queue/templates/{name}.txt
```

If `.txt` missing: parse skip is OK; `renderPlaintextBody` falls back to a stripped/fallback dump (prefer real `.txt`).

### 4. Register in Go

`handler_email.go` → `NewSendEmailHandler` name list:

```go
for _, name := range []string{
    "welcome", "room_invite", "password_reset",
    "password_changed", "verify_email", "generic",
    "your_new_name",
} {
```

### 5. Config / admin (optional)

Add `subjectLines` / `preheaderText` keys under `email.templates`. For admin UI overrides, extend `SystemSettings` + `settings_repository` mapping (same pattern as verify/welcome/reset/changed/invite).

### 6. Enqueue from a handler

```go
queue.Enqueue(ctx, database.GetDB(), "send_email", queue.SendEmailPayload{
    To:           user.Email,
    Subject:      "Fallback subject if config/DB unset",
    TemplateName: "your_new_name",
    TemplateData: map[string]any{"Name": user.Name, "ActionURL": url},
})
```

### 7. Verify

```bash
cd server && go build ./...
cd server && go test ./internal/queue/ -count=1
```

Parse failures log: `email: failed to parse HTML template, will fall back to plain text`.

---

## Testing & Debugging

### Offline (no SMTP)

Empty `email.smtpHost` → handler warns and logs full HTML under field `"body"`:

```text
email: SMTP not configured, skipping send — body logged
```

### Tests

`handler_email_test.go` covers: parse/render all HTML + TXT, branding inject, unknown → generic, welcome without LoginURL, verify URL fallback block, password_changed UserAgent optional, stripHTML, BuildMessage.

### Syntax pitfalls

- Unclosed `{{if}}` / `{{range}}` / `{{with}}`
- Go actions inside `<!--[if mso]>` comments can break parsing when dynamic
- Inside `{{range}}`, root scope is `{{$.Var}}`
- Unknown template name → silent **generic** (or `renderFallback` pre dump) — no hard error

---

## Common Pitfalls

### Go actions inside MSO comments

Avoid dynamic widths in MSO conditionals; use fixed MSO sizes and Go vars in normal HTML/CSS.

### `html/template` escaping

Safe in `href` and `style` color contexts. Raw HTML in body escapes tags — use `template.HTML` only if intentional.

### Outlook + max-width

Keep MSO 680 wrapper around `.email-container`.

### Outlook + dark mode images

Light image wins; accept dual-render risk.

### Unknown template → generic

Always add name to parse list **and** ship `templates/{name}.html`.

### `embed.FS` path

Only files under `server/internal/queue/templates/` matching `*.html` / `*.txt` embed. Wrong dir = missing at runtime.

### Generic range filter

`generic.html` skips branding keys when ranging the map:

`Preheader`, `InstanceName`, `SupportEmail`, `InstanceURL`, `HeaderBg`, `ButtonBg`.

### VML background images (baseline)

Update both CSS `background-image` and MSO `v:image` `src` together.

### Inline vs class

Inline wins unless dark CSS uses `!important`. For colors that must not invert, force them again inside the dark media query.

---

## SMTP notes (handler)

| Config | Meaning |
|--------|---------|
| `smtpHost` / `smtpPort` | Required to send |
| `username` / `password` | Optional PLAIN auth |
| `fromAddress` | Default `noreply@bedrud` if empty |
| `fromName` | Default `Bedrud` |
| `tlsSkipVerify` | Skip cert verify |
| `smtpsMode` | Direct TLS (e.g. 465) |
| Send timeout | 30s |

See also skill `bedrud-jobs` for queue wiring; this skill is template + email handler focused.
