---
name: bedrud-email-cerberus
description: Cerberus hybrid HTML email templates — copy, customize, register, dark mode, Outlook compatibility.
license: Apache License
---

# Bedrud Cerberus Email Templates

Go module `bedrud`. `internal/queue/templates/cerberus-hybrid.html` is the canonical Cerberus hybrid baseline.

**Rule: Never edit `cerberus-hybrid.html`.** Always `cp` to a new file and edit the copy. Original is upstream reference for re-merging Cerberus updates.

---

## Go Template Engine — Not Fiber View

The email system uses **Go `html/template` directly** — not Fiber's `c.Render()` or any template framework.

### How Templates Load

```go
//go:embed templates/*.html templates/*.txt
var emailTemplatesFS embed.FS

// In handler_email.go NewSendEmailHandler:
h.tmpls = make(map[string]*template.Template)
for _, name := range []string{"welcome", "room_invite", "password_reset",
    "password_changed", "verify_email", "generic"} {
    t, err := template.ParseFS(emailTemplatesFS, "templates/"+name+".html")
    // ...
    h.tmpls[name] = t
}
```

### How Templates Render

```go
var buf bytes.Buffer
tmpl.Execute(&buf, payload.TemplateData) // payload.TemplateData is map[string]any
return buf.String()
```

### Data Flow

1. `SendEmailPayload{To, Subject, TemplateName, TemplateData map[string]any}` enqueued as job
2. `loadBranding()` fetches `SystemSettings` from DB, overlays on `config.yaml` defaults
3. `injectBranding()` merges `InstanceName`, `SupportEmail`, `InstanceURL`, `HeaderBg`, `ButtonBg`, `Preheader` into `TemplateData`
4. Template executes with the merged `map[string]any` — all vars are top-level keys like `{{.InstanceName}}`
5. Auto-escaped by `html/template` — safe against XSS, works in style attrs and hrefs

---

## Template Anatomy & Section Reference

`cerberus-hybrid.html` (680px max-width, hybrid grid) has these named sections. Use as building blocks:

| Section | Lines (approx) | Purpose | Use For |
|---------|---------------|---------|---------|
| Preheader | 105-111 | Hidden preview text (`{{.Preheader}}`) | All emails |
| Email Header | 126-133 | Logo image, `{{.InstanceName}}` fallback | All emails |
| Hero Image, Flush | 136-143 | Full-width bleed image | Welcome, announcements |
| 1 Column Text + Button | 146-177 | Body text + CTA button | Verify email, password reset |
| Background Image w/ Text | 179-207 | Bulletproof bg image + text overlay | Promotional |
| 2 Even Columns | 209-264 | Side-by-side content (hybrid grid) | Features, room details |
| 3 Even Columns | 266-345 | Triple columns (hybrid grid) | Pricing, gallery |
| Thumbnail Left, Text Right | 348-398 | Image + text beside each other | Room invites with preview |
| Thumbnail Right, Text Left | 401-452 | Same, flipped alignment | Alternating layouts |
| Clear Spacer | 455-459 | Vertical whitespace | Separation |
| 1 Column Text | 462-476 | Simple text block | Notifications |
| Footer | 479-489 | webversion, unsubscribe, address | All emails |
| Full Bleed Background | 492-525 | Full-width colored section outside container | Callout banners |

### Email Types → Section Mapping

| Template | Sections Used |
|----------|--------------|
| `welcome` | Preheader + Header + Hero + 1-Column Text+Button + Footer |
| `verify_email` | Preheader + Header + 1-Column Text+Button + Footer |
| `password_reset` | Preheader + Header + 1-Column Text+Button + Footer |
| `password_changed` | Preheader + Header + 1-Column Text + Footer |
| `room_invite` | Preheader + Header + 1-Column Text+Button + Thumbnail Layout + Footer |
| `generic` | Preheader + Header + Key-Value Table + Footer (fallback when template name unknown) |

---

## Branding Variables Available

All injected by `injectBranding()` in `handler_email.go`. Sources: `config.yaml` → SystemSettings DB table (admin UI overrides).

### Core Branding

| Var | Config YAML | DB Field | Example | Used In |
|-----|------------|----------|---------|---------|
| `{{.InstanceName}}` | `email.templates.instanceName` | `EmailInstanceName` | `"Bedrud"` | Header `<h1>`, body intro |
| `{{.SupportEmail}}` | `email.templates.supportEmail` | `EmailSupportEmail` | `"help@example.com"` | Footer contact link |
| `{{.InstanceURL}}` | `email.templates.instanceUrl` | `EmailInstanceURL` | `"https://bedrud.org"` | Footer link |
| `{{.HeaderBg}}` | `email.templates.headerBgColor` | `EmailHeaderBg` | `"#1a1a2e"` | Header `<td>` background |
| `{{.ButtonBg}}` | `email.templates.buttonBgColor` | `EmailButtonBg` | `"#e11d48"` | Primary button background |
| `{{.Preheader}}` | `email.templates.preheaderText` (per-template) | `EmailPreheader*` | `"Verify your email..."` | Hidden preview `div` |

### Per-Template Custom Vars

Passed via `payload.TemplateData` when enqueueing the job. Not auto-injected — must be in the enqueue call.

| Var | Used In | Source |
|-----|---------|--------|
| `{{.Name}}` | welcome, verify_email | User's display name |
| `{{.VerifyURL}}` | verify_email | Verification link |
| `{{.ResetURL}}` | password_reset | Password reset link |
| `{{.RoomName}}` | room_invite | Room name being shared |
| `{{.RoomURL}}` | room_invite | Direct join link |
| `{{.InviterName}}` | room_invite | Who invited them |

### How Branding Resolves (Priority: DB > Config)

1. Hardcoded defaults in `loadBranding()` (InstanceName = "Bedrud", HeaderBg = "#1a1a2e", ButtonBg = "#e11d48")
2. Overlaid with `config.yaml` `email.templates.*` values
3. Overlaid with `SystemSettings` DB table row 1 values (set via admin panel)
4. Template keys that are already set in `TemplateData` are **not** overwritten (caller can override branding per-email)

---

## Dark Mode Patterns

Cerberus uses `@media (prefers-color-scheme: dark)` inside `<style>` blocks. Bedrud's Cerberus copy must preserve this pattern.

### Structure

```css
/* Dark Mode Styles : BEGIN */
@media (prefers-color-scheme: dark) {
    .email-bg {
        background: #111111 !important;
    }
    .darkmode-bg {
        background: #222222 !important;
    }
    h1, h2, h3, p, li, .darkmode-text,
    .email-container a:not([class]) {
        color: #F7F7F9 !important;
    }
    td.button-td-primary,
    td.button-td-primary a {
        background: #ffffff !important;
        border-color: #ffffff !important;
        color: #222222 !important;
    }
    .footer td {
        color: #aaaaaa !important;
    }
    .darkmode-fullbleed-bg {
        background-color: #0F3016 !important;
    }
}
/* Dark Mode Styles : END */
```

### Utility Classes (already defined in cerberus-hybrid.html)

| Class | Dark Mode Effect | Apply To |
|-------|-----------------|----------|
| `.darkmode-bg` | `background: #222222` | `<td>` with white/light background in light mode |
| `.darkmode-text` | `color: #F7F7F9` | Text elements that need color override |
| `.email-bg` | `background: #111111` | `<body>` and `<center>` (outer canvas) |
| `.darkmode-fullbleed-bg` | `background-color: #0F3016` | Full-bleed colored sections |

### How to Apply in Your Template Copy

Every `<td>` with a light background color in light mode must also have the appropriate dark mode class:

```html
<!-- Light mode: white bg. Dark mode: #222 via .darkmode-bg -->
<td style="background-color: #ffffff;" class="darkmode-bg">
```

Text colors should be defined inline for light mode and overridden via class or element selector for dark mode:

```html
<p style="color: #555555;" class="darkmode-text">
    Text that is #555 in light mode, #F7F7F9 in dark mode.
</p>
```

### Buttons

The primary button pattern in Cerberus inverts in dark mode (white bg + dark text instead of dark bg + white text):

```html
<td class="button-td button-td-primary" style="border-radius: 4px; background: #222222;">
    <a class="button-a button-a-primary" href="..."
       style="background: #222222; border: 1px solid #000000; color: #ffffff;">
        Download
    </a>
</td>
```

The dark mode CSS handles the inversion automatically via `.button-td-primary` selector. Don't remove or rename these classes.

### Extending Dark Mode for Custom Sections

If your template copy adds new colored sections, add new dark mode utility classes:

```css
@media (prefers-color-scheme: dark) {
    .darkmode-card {
        background: #2a2a2a !important;
        border-color: #444444 !important;
    }
    .darkmode-border {
        border-color: #444444 !important;
    }
    .darkmode-accent {
        color: #60a5fa !important;
    }
}
```

All dark mode rules **must use `!important`** to override inline styles (email clients strip embedded styles otherwise).

### Image Swapping for Dark Mode

Since SVG is not supported in email, swap raster images by toggling display:

```html
<img src="logo-light.png" class="display-only-in-light-mode">
<!--[if !mso]><!-->
<img src="logo-dark.png" class="display-only-in-dark-mode">
<!--<![endif]-->
```

CSS: `.display-only-in-dark-mode { display: none !important; }` (reversed in `@media (prefers-color-scheme: dark)`).

**Outlook limitation:** Outlook ignores `prefers-color-scheme` AND the `[if !mso]` guard. Both images render in Outlook. Mitigations:
- Keep light-mode logo filename as default for Outlook users
- Use same image for both if Outlook audience is significant
- Accept that Outlook always shows light mode — it's the least capable client

### Removing Dark Mode Entirely

If the template copy doesn't need dark mode, remove everything between:

```css
/* Dark Mode Styles : BEGIN */
...
/* Dark Mode Styles : END */
```

And remove `color-scheme` meta tags from `<head>`. (Recommended to keep dark mode — users expect it.)

---

## Hybrid Grid System (Responsive Columns)

Cerberus hybrid uses `inline-block` + ghost tables for responsive columns without media queries. Outlook gets fixed-width tables; everyone else gets inline-block that wraps at `max-width`.

### 2-Column Pattern

```html
<!--[if mso]>
<table role="presentation" border="0" cellspacing="0" cellpadding="0" width="660">
<tr>
<td valign="top" width="330">
<![endif]-->
<div style="display:inline-block; margin: 0 -1px; width:100%; min-width:200px; max-width:330px; vertical-align:top;" class="stack-column">
    <!-- Column 1 content -->
</div>
<!--[if mso]>
</td>
<td valign="top" width="330">
<![endif]-->
<div style="display:inline-block; margin: 0 -1px; width:100%; min-width:200px; max-width:330px; vertical-align:top;" class="stack-column">
    <!-- Column 2 content -->
</div>
<!--[if mso]>
</td>
</tr>
</table>
<![endif]-->
```

### Column Widths (680px container)

| Layout | Per-Column | Notes |
|--------|-----------|-------|
| 2 columns | 330px each | 660px total, 10px padding each side |
| 3 columns | 220px each | 660px total |
| Thumbnail + Text | 220px + 440px | Use `dir="rtl"` or `dir="ltr"` for alignment swap |

### Mobile Stacking Classes

Defined in `@media screen and (max-width: 480px)`:

| Class | Effect |
|-------|--------|
| `.stack-column` | Forces `display:block; width:100%; max-width:100%` |
| `.stack-column-center` | Same as above + `text-align:center` |
| `.center-on-narrow` | Centers images, buttons, tables on mobile |
| `table.center-on-narrow` | `display:inline-block` variant for table elements |

### When to Use What

- **Hybrid grid** (`div` + ghost tables): main layout columns — best for responsive + Outlook
- **Flat table rows**: single-column content (header, hero, 1-column text) — simpler, no grid needed
- **Full-bleed section**: outside `.email-container` div, wrapped in `width:100%` table — for background color that spans full viewport

### Column Number Gotchas

- MSO ghost table `width` must match sum of column `width` attributes exactly (660px for 2×330 or 3×220)
- Non-MSO divs use `min-width` + `max-width` — the `margin: 0 -1px` fixes 1px gap from inline-block whitespace
- If adding/removing columns, update both the MSO table widths and the div `max-width` values

---

## Registering a New Cerberus-based Template

Step-by-step to create a new email template using the Cerberus baseline.

### Step 1: Copy the Cerberus Template

```bash
cp server/internal/queue/templates/cerberus-hybrid.html \
   server/internal/queue/templates/{name}.html
```

Never edit `cerberus-hybrid.html`. Only edit the copy.

### Step 2: Customize the Copy

Replace Cerberus placeholder text with Go template vars:

| Cerberus Placeholder | Replace With |
|---------------------|--------------|
| `Praesent laoreet malesuada&nbsp;cursus.` | `{{.Title}}` or hardcoded subject |
| `Maecenas sed ante pellentesque...` | `{{.Message}}` or hardcoded body |
| `alt_text` | Meaningful alt text or template var |
| `https://via.placeholder.com/...` | `{{.ImageURL}}` or embedded asset |
| `Company Name` | `{{.InstanceName}}` |
| `123 Fake Street...` | Leave as example or remove |

Key integration points for branding:

```html
<!-- Header: swap static hex for {{.HeaderBg}} -->
<td style="background-color: {{.HeaderBg}};" ...>

<!-- Instance name -->
<h1>{{.InstanceName}}</h1>

<!-- Preheader -->
<div style="display:none;...">{{.Preheader}}</div>

<!-- Support email + instance URL in footer -->
{{if .SupportEmail}}<a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a>{{end}}
{{if .InstanceURL}}<a href="{{.InstanceURL}}">{{.InstanceURL}}</a>{{end}}
```

### Step 3: Create Plaintext Version (Optional)

```bash
touch server/internal/queue/templates/{name}.txt
```

If missing, plaintext is auto-generated via `stripHTML()` — readable but ugly. For professional plaintext, write a `.txt` version.

### Step 4: Register in Go Code

In `handler_email.go` `NewSendEmailHandler()`, add the template name to the list:

```go
for _, name := range []string{"welcome", "room_invite", "password_reset",
    "password_changed", "verify_email", "generic", "your_new_name"} {
```

### Step 5: Add Config Defaults (Optional)

If the template needs its own subject line or preheader default, add entries to:

```yaml
# config.yaml
email:
  templates:
    subjectLines:
      your_new_name: "Subject line for new template"
    preheaderText:
      your_new_name: "Preview text for inbox"
```

And to the SystemSettings model if admin overrides are desired.

### Step 6: Verify Build

```bash
cd server && go build ./...        # embed.FS must compile
cd server && go test ./internal/queue/...  # existing tests pass
```

### Step 7: Test Render

With SMTP disabled in config, the handler logs full HTML body as a warning:

```
email: SMTP not configured, skipping send — body logged
```

Watch for `{{.VarName}}` left unrendered (shows as `<no value>` or empty) and template syntax errors (logged as warning at startup).

---

## Testing & Debugging

### Offline (No SMTP)

Set `email.smtpHost: ""` (default). Handler logs full HTML body as structured field `"body"`. Pipe to a file:

```bash
go run . 2>&1 | grep '"body":' | head -1 | sed 's/.*"body":"//' | sed 's/".*//' > test.html
open test.html  # preview in browser
```

### Template Syntax Errors

At startup in `NewSendEmailHandler`, if `template.ParseFS` fails:

```
email: failed to parse HTML template, will fall back to plain text
```

Fix the template syntax and rebuild. Common causes:
- Unclosed `{{range}}` / `{{if}}` / `{{with}}`
- Mismatched `{{end}}`
- Go template keyword inside HTML comment (comments aren't stripped before parse)
- `.` (dot) context changing inside `{{range}}` — use `{{$.VarName}}` to access root scope

### Auto-Generated Plaintext

If no `.txt` file exists, `stripHTML()` removes all tags. Verify it's readable:

```go
// Readable? If not, add a .txt file
fmt.Println(renderPlaintextBody(h.plainTmpls, payload))
```

### Email Client Testing

- **Browser**: Open rendered HTML in Chrome/Safari — closest to webmail (Gmail, Yahoo, Outlook.com)
- **Litmus / Email on Acid**: Paid services, test across 100+ clients
- **Manual**: Send to real addresses via SMTP-enabled config → check Gmail, Outlook, Apple Mail

---

## Common Pitfalls

### Go Templates Inside MSO Comments

**Wrong** — `{{.Var}}` inside MSO conditional comment breaks the Go template parser:

```html
<!--[if mso]>
<td width="{{.Width}}">  <!-- PARSER ERROR -->
<![endif]-->
```

**Right** — use MSO for fixed sizing, inline CSS for Go vars:

```html
<!--[if mso]>
<td width="680">
<![endif]-->
<div style="max-width: {{.MaxWidth}}px;">
```

### `html/template` Auto-Escaping

Safe: `{{.URL}}` in `<a href="...">` — Go escapes `&`, `"`, etc. correctly.
Safe: `{{.Color}}` in `style="background: ..."` — Go handles CSS context.
**Not safe** for raw HTML: `{{.HTMLContent}}` in body — will escape `<`, `>`. Use `template.HTML` type if raw HTML needed (rare for email — keep it clean).

### Outlook + max-width

Outlook ignores `max-width`. The MSO wrapper table handles this:

```html
<div style="max-width: 680px; margin: 0 auto;">
    <!--[if mso]>
    <table align="center" role="presentation" width="680"><tr><td>
    <![endif]-->
    <!-- email content -->
    <!--[if mso]>
    </td></tr></table>
    <![endif]-->
</div>
```

Always keep the MSO wrapper for any fixed-width container.

### Outlook + Dark Mode Images

Outlook renders all images — both `.display-only-in-light-mode` and `.display-only-in-dark-mode`. The `[if !mso]` guard prevents the dark mode image from appearing in Outlook HTML, but dark mode CSS classes are ignored, so the light image always shows. Accept this behavior.

### Template Not Found → Silent Fallback

If template name not registered, `renderEmailBody()` falls back to `generic` template. No error is returned. Always:

1. Add name to the list in `NewSendEmailHandler`
2. Verify with `go build ./...`
3. Check logs for "failed to parse" warnings

### `embed.FS` Path Restriction

Templates must be in `templates/` subdirectory of the queue package. Files outside `server/internal/queue/templates/` won't be embedded. The `//go:embed` directive uses `templates/*.html` — add files only to that directory.

### Range Over Map (Generic Template Pattern)

When iterating `{{range $k, $v := .}}`, remember Go template dot changes scope. Access root with `{{$.InstanceName}}`. The generic template filters out branding keys to avoid rendering them as data rows.

### Background Image in Outlook

Cerberus uses VML for bulletproof background images:

```html
<!--[if gte mso 9]>
<v:image xmlns:v="urn:schemas-microsoft-com:vml" fill="true" stroke="false"
    style="width: 680px; height: 180px;"
    src="https://example.com/bg.jpg" />
<v:rect fill="true" stroke="false" style="position: absolute; width: 680px; height: 180px;">
    <v:fill opacity="0%" color="#222222" />
<![endif]-->
```

When customizing, update both the CSS `background-image` URL and the VML `v:image src`. Mismatched paths = broken background in Outlook.

### Inline Style vs Class Precedence

In email, inline styles win over `<style>` block classes. Dark mode `!important` overrides inline. For elements that should NOT change in dark mode (e.g., a badge that stays red), apply a class and use `color: #ff0000 !important` in the dark mode block explicitly for that class.
