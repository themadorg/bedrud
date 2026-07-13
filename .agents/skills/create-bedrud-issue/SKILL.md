---
name: create-bedrud-issue
description: >
  Create a GitHub issue on themadorg/bedrud using .github ISSUE_TEMPLATE forms and
  gh CLI. Sets Issue Type (Bug/Feature/Task), Priority, surface labels, research,
  screenshots. Triggers: create issue, open issue, file bug, feature request,
  GitHub issue, report bug, /create-bedrud-issue.
license: Apache License
---

# Create Bedrud Issue

> **Canonical path for repo agents:** `.agents/skills/create-bedrud-issue/`  
> Grok TUI also loads `.grok/skills/create-bedrud-issue/` — keep both in sync when editing.

Open a high-quality **GitHub issue** on `themadorg/bedrud` with `gh`, matching
`.github/ISSUE_TEMPLATE/*` and `CONTRIBUTING.md`. Do **not** invent free-form
issues when a template fits. Blank issues are disabled (`blank_issues_enabled: false`).

**Sources of truth (always re-read if templates may have changed):**
- `.github/ISSUE_TEMPLATE/*.yaml` — form fields, titles, intended labels
- `.github/ISSUE_TEMPLATE.md` — optional shared sections (Summary / AC / DoD)
- `.github/ISSUE_TEMPLATE/config.yaml` — no blank issues; security → private advisory
- `CONTRIBUTING.md` § Reporting Issues
- `references/issue-templates.md` in this skill — field map + label mapping

---

## 0. Preconditions

```bash
gh auth status
# must be logged in with repo access to themadorg/bedrud
```

Run from repo root (or pass `-R themadorg/bedrud`). Prefer current git remote.

**Security gate:** If the user is reporting a **live vulnerability / exploit / secret
leak**, stop. Do **not** open a public issue. Point them to GitHub Security Advisories
(repo → Security → Advisories) or the contact in `config.yaml`. Only use the
`[SECURITY]` template for **security improvements / hardening** that are safe to
discuss publicly.

**Discussions gate:** Pure questions with no actionable bug/feature/task → suggest
Discussions instead of an issue (`config.yaml` contact link).

---

## 1. Classify the issue type

Pick exactly one **form template** and map it to a GitHub **Issue Type** (`Bug` | `Feature` | `Task`).  
Issue Type is a first-class GitHub field (not a label). **Always set it** with
`gh issue create --type …` / `gh issue edit --type …`.

| Intent keywords | Form template (`gh --template`) | Title prefix | **GitHub `--type`** | File |
|-----------------|----------------------------------|--------------|---------------------|------|
| bug, broken, error, crash, wrong behavior | `Bug Report` | `[BUG]: ` | **`Bug`** | `bug.yaml` |
| feature, enhancement, want, add support | `Feature Request` | `[FEATURE]: ` | **`Feature`** | `feature.yaml` |
| task, implement, chore, engineering work | `Task` | `[TASK]: ` | **`Task`** | `task.yaml` |
| docs, documentation, guide, README | `Documentation` | `[DOCS]: ` | **`Task`** | `docs.yaml` |
| epic, initiative, multi-task program | `Epic` | `[EPIC]: ` | **`Task`** (or Feature if product-facing) | `epic.yaml` |
| slow, perf, latency, memory, bottleneck | `Performance Issue` | `[PERF]: ` | **`Bug`** if regression; else **`Task`** | `performance.yaml` |
| security hardening (public-safe only) | `Security Issue` | `[SECURITY]: ` | **`Bug`** if vuln/defect; else **`Task`** | `security.yaml` |
| research, investigate, spike, explore | `Spike / Research` | `[SPIKE]: ` | **`Task`** | `spike.yaml` |

List enabled types anytime:

```bash
gh api repos/themadorg/bedrud/issue-types --jq '.[].name'
# → Task, Bug, Feature
```

If type is ambiguous, **ask the user** (Bug / Feature / Task). Do not guess.

---

## 2. Clarify with the user (when unclear)

Ask only what is still missing. Prefer one focused multi-part question over a long
interrogation. Use `ask_user_question` when choices are discrete; free text in chat
when open-ended.

### Always resolve before filing

| Area | If missing… |
|------|-------------|
| Type | Ask Bug / Feature / Task (+ form template from §1) |
| Problem / goal | Ask what failed or what outcome they want |
| Scope | Which surface: server / web / site / desktop / android / ios / agents / deploy |
| Priority | Ask or infer (§4 Priority). Default **Medium** if user is fine with it |
| Security public-safe? | Confirm not a private vuln report |

### Type-specific must-haves

- **Bug:** expected vs actual, steps (or “not reproducible — observed once”), env (OS/browser/app version) if relevant
- **Feature:** problem statement first; solution is optional until problem is clear
- **Task / Epic:** goal + in/out of scope; epic needs success criteria
- **Docs:** what is wrong/missing + where (path or URL)
- **Perf:** what is slow + any numbers (or mark “unknown — need baseline”)
- **Spike:** research question + expected deliverable

Do **not** invent severity, priority, estimates, or reproduction steps. If the user
doesn’t know, put `Unknown` / omit optional fields rather than fabricating.

---

## 3. Deep research into the codebase

Before drafting, investigate enough to make the issue **actionable for maintainers**.

### Research checklist

1. **Search existing issues** — avoid duplicates:
   ```bash
   gh issue list -R themadorg/bedrud --state open --limit 30 --search "<keywords>"
   gh issue list -R themadorg/bedrud --state all --limit 20 --search "<keywords>"
   ```
   If a clear duplicate exists, show the user the URL and ask whether to comment
   there instead of opening a new issue.

2. **Locate relevant code** — use codebase search (grep/semantic), skills, and docs:
   - Load `bedrud-dispatch` / leaf skills for domain maps when helpful
   - Key maps: `AGENTS.md`, `docs/server/`, `apps/web/AGENTS.md`, handlers, stores
   - Note concrete paths, symbols, routes, config keys

3. **Capture evidence** — for bugs/perf: failing tests, error strings, log snippets,
   version from git/`package.json`/releases if known

4. **Sketch impact** — which layer (API, LiveKit, UI, mobile) and who is affected

Write research findings into the issue body under the right template sections
(Background, Technical Requirements, Notes, etc.) — **cite file paths**, not vague
“somewhere in the backend.”

If research reveals the user may have misdiagnosed type (e.g. “feature” that is
really a bug), say so and confirm before filing.

---

## 4. Draft the issue (template-faithful body)

### Title

`{PREFIX}{short imperative or noun phrase}`

Examples:
- `[BUG]: Guest join fails with 403 when email verification is off`
- `[FEATURE]: Room-level chat retention overrides`
- `[TASK]: Add queue metrics to admin overview`

Keep under ~80 chars when possible. User may override.

### Body

Reproduce **GitHub form field order** as markdown headings matching template labels.
Required fields must be non-empty. Optional fields: include when research/user
provided value; skip empty fluff.

Canonical section layouts: **`references/issue-templates.md`**.

Also pull useful structure from `.github/ISSUE_TEMPLATE.md` when it helps
(especially Task / Epic): Summary, Acceptance Criteria (`- [ ]`), Definition of Done.

### Issue Type (required — GitHub field, not a label)

| Form / intent | `--type` value (exact name, capitalised) |
|---------------|------------------------------------------|
| Bug report / defect | `Bug` |
| Feature request / enhancement | `Feature` |
| Task, docs, spike, most chores | `Task` |

```bash
gh issue create ... --type Bug
gh issue edit 37 -R themadorg/bedrud --type Bug
```

Verify after create:

```bash
gh issue view N -R themadorg/bedrud --json issueType -q .issueType.name
# must be Bug | Feature | Task — never null
```

If `issueType` is null, immediately `gh issue edit N --type …`.

### Priority (required — GitHub Issue Field)

Repo field **Priority** (single-select): `Urgent` | `High` | `Medium` | `Low`.

Map from template body when present:

| Body severity / priority | Priority field |
|--------------------------|----------------|
| Critical | `Urgent` |
| High | `High` |
| Medium | `Medium` |
| Low | `Low` |
| Unknown / omitted | Ask, or default **`Medium`** |

Set via GraphQL after create (IDs can be re-queried; names are stable):

```bash
# 1) Resolve field + option IDs (run when unsure; cache for session)
gh api graphql -f query='
query {
  repository(owner: "themadorg", name: "bedrud") {
    issueFields(first: 20) {
      nodes {
        ... on IssueFieldSingleSelect {
          id name
          options { id name }
        }
      }
    }
  }
}' --jq '.data.repository.issueFields.nodes[] | select(.name=="Priority")'

# Typical (verify with query above):
# Priority field id: IFSS_kgDOAk7FTg
# Urgent IFSSO_kgDOBAnBYg | High IFSSO_kgDOBAnBYw | Medium IFSSO_kgDOBAnBZA | Low IFSSO_kgDOBAnBZQ

# 2) Issue node id
ISSUE_ID=$(gh api graphql -f query='
  query($n:Int!) {
    repository(owner:"themadorg", name:"bedrud") { issue(number:$n) { id } }
  }' -F n=ISSUE_NUM --jq '.data.repository.issue.id')

# 3) Set Priority (example: Medium)
gh api graphql -f query='
mutation {
  setIssueFieldValue(input: {
    issueId: "'"$ISSUE_ID"'"
    issueFields: [{
      fieldId: "IFSS_kgDOAk7FTg"
      singleSelectOptionId: "IFSSO_kgDOBAnBZA"
    }]
  }) { clientMutationId }
}'

# 4) Verify
gh api repos/themadorg/bedrud/issues/ISSUE_NUM \
  --jq '{type: .type.name, priority: [.issue_field_values[] | select(.issue_field_name=="Priority") | .single_select_option.name][0]}'
```

Option ID cheat-sheet (re-query if mutation fails):

| Priority | `singleSelectOptionId` |
|----------|------------------------|
| Urgent | `IFSSO_kgDOBAnBYg` |
| High | `IFSSO_kgDOBAnBYw` |
| Medium | `IFSSO_kgDOBAnBZA` |
| Low | `IFSSO_kgDOBAnBZQ` |
| Field | `fieldId`: `IFSS_kgDOAk7FTg` |

Also keep Priority (and Severity for bugs) in the **body** text for humans reading the template.

### Labels (kind + surface)

Always apply a **kind label** (when it exists) **and** a **surface/area** label.
Only use labels that exist (`gh label list -R themadorg/bedrud`).

**Kind labels** complement Issue Type (Type is authoritative for Bug/Feature/Task):

| Intent | Label | Notes |
|--------|-------|-------|
| Bug | `bug` | Match `--type Bug` |
| Feature | `enhancement` (or `feature` if present) | Match `--type Feature` |
| Docs | `documentation` | Type still `Task` |
| Other | omit kind label if none fit | Type still set |

#### Surface / area (prefer over language labels)

| Surface | Label | When |
|---------|-------|------|
| Web meeting UI, React, `apps/web` | `frontend-web` | Screen share, chat, stage, settings UI, Vite/web client |
| Go API, auth, rooms, queue, LiveKit embed, `server/` | `backend-core` | Handlers, GORM, JWT, webhooks, embedded LK |
| Android app | `android` | `apps/android` |
| Desktop Windows | `desktop-windows` | Windows desktop build/bugs |
| Desktop macOS | `desktop-mac` | macOS desktop |
| Desktop Linux | `desktop-linux` | Linux desktop |

Rules:

1. Pick **one primary surface** (multi-surface only if the work truly spans them —
   e.g. API + web contract → `backend-core` + `frontend-web`).
2. Prefer surface labels over stack labels (`javascript`, `go`, `rust`). Use language
   labels only as **extra** when the issue is narrowly about that toolchain (e.g. a
   Go race), not for normal product bugs.
3. If surface is unclear, **ask** (web / server / android / which desktop OS).
4. Preview must list **Issue Type**, **Priority**, and labels; create with:

```bash
gh issue create ... --type Bug --label bug --label frontend-web
# then set Priority via GraphQL (§ above)
```

```bash
gh label list -R themadorg/bedrud --limit 100
```

Never pass a label `gh` will reject.

### Assignees / milestone / parent

Only set if the user requested them. Epics may later use `--parent` for child issues.

### Images / screenshots (required when user shared any)

If the user attaches or pastes **any image** (chat attachment, `[Image #N]`, screenshot
path, drag-and-drop, etc.), you **must** post those images on the GitHub issue — not
only describe them in prose.

**Hosting:** use **GitHub-native** embeds only
(`https://github.com/user-attachments/assets/...` via `gh image`).  
**Never** use Catbox, Imgur, 0x0.st, gists, release assets, or other third-party hosts.

Rules:

1. **Collect every image** from the user message/thread that is evidence for this
   issue (UI screenshots, error dialogs, device photos of the screen, etc.).
2. **Do not invent** images. Only attach what the user (or your research tools)
   actually provided.
3. **Save locally** to a stable path before upload, e.g.
   `/tmp/bedrud-issue-img-1.png` (preserve format: png/jpg/webp/gif).
4. **Layout by count** (apply after upload when you have real `user-attachments` URLs):

   **Exactly one screenshot — put it at the top of the body:**
   ```markdown
   ![short alt text](https://github.com/user-attachments/assets/<uuid>)

   One sentence explaining what the screenshot shows.

   ## Description
   ...
   ```
   - Image is the **first** content in the body (before template headings).
   - **Exactly one** sentence under the image — no multi-paragraph caption, no
     `## Screenshots` heading for a single image.

   **Two or more screenshots — use a Screenshots section:**
   ```markdown
   ## Screenshots

   ![caption A](https://github.com/user-attachments/assets/<uuid-a>)

   ![caption B](https://github.com/user-attachments/assets/<uuid-b>)
   ```
   Place `## Screenshots` after Description (or after Logs for bugs), with a short
   alt/caption per image. Optional one-line note under an image only if needed.

5. In the draft/preview, note image count + captions; use placeholders until upload
   if creating the issue first.
6. After `gh issue create` (or before, if preferred), **upload with `gh image`** and
   rewrite the body with real URLs (§6.1). Never leave text-only when images exist.
7. If upload fails, retry once; if still failing, tell the user to drag-drop onto the
   issue in the browser — **do not** fall back to Catbox/external hosts.

Security: skip images that clearly contain secrets (tokens, passwords, private
keys). Redact or ask the user before posting.

---

## 5. Confirm with the user (required)

Show a **preview** before creating:

1. Title
2. Form template + **GitHub Issue Type** (`Bug` | `Feature` | `Task`)
3. **Priority** (`Urgent` | `High` | `Medium` | `Low`)
4. Labels (kind + surface)
5. Full body (or body file path)
6. Research notes / related issue links
7. **Images to attach** (count + captions), or “none”
8. Commands that will run (`gh issue create --type …`, then priority mutation)

Ask: create as-is, edit, or cancel. Do **not** run `gh issue create` until the user
approves (unless they already said “just file it” / “no more confirmation”).

---

## 6. Create with `gh` (fast path)

Prefer body file to avoid shell escaping bugs:

```bash
# write draft to temp file, then:
gh issue create \
  -R themadorg/bedrud \
  --title "[BUG]: concise title" \
  --body-file /tmp/bedrud-issue-body.md \
  --type Bug \
  --label "bug" \
  --label "frontend-web"
# → prints issue URL; capture ISSUE_NUM

# REQUIRED: set Priority (see §4 Priority GraphQL)
# REQUIRED if type missing: gh issue edit N --type Bug
```

Optional:

```bash
gh issue create -R themadorg/bedrud --template "Bug Report" --type Bug --title "..." --body-file ...
gh issue create ... --type Feature --label enhancement --label frontend-web
gh issue create ... --type Task --label backend-core
gh issue create ... --assignee "@me"
gh issue create ... --parent 123          # sub-issue of epic
```

After success:

1. Confirm `issueType` is set; if null → `gh issue edit N --type Bug|Feature|Task`
2. Set **Priority** via `setIssueFieldValue` (§4)
3. If the user shared images → **§6.1 upload and embed** (required)
4. Print the issue URL + type + priority
5. Offer next steps only if useful

**Do not** use `-w` (browser) unless the user asks. Prefer non-interactive CLI.

### 6.1 Upload user images (GitHub `user-attachments` only)

`gh issue create` cannot attach binaries. Use the **`gh image`** extension
([drogers0/gh-image](https://github.com/drogers0/gh-image)) — same flow as browser
drag-and-drop → `https://github.com/user-attachments/assets/<uuid>`.

```bash
# Install once if missing
gh extension install drogers0/gh-image

# From repo root (or pass --repo). Prints markdown: ![name](https://github.com/user-attachments/assets/…)
gh image /tmp/bedrud-issue-img-1.png --repo themadorg/bedrud

# Multiple files → one markdown line per file
gh image /tmp/img-a.png /tmp/img-b.png --repo themadorg/bedrud
```

Auth: uses browser GitHub session cookie by default (not `gh auth token` alone).
Headless/CI: `GH_SESSION_TOKEN` or `gh image extract-token` (treat like a password).

**Do not use:** Catbox, Imgur, gist, raw.githubusercontent commits, release assets,
or `uploads.github.com/.../issues/.../images` with PAT (unreliable / not the embed path).

#### Assemble body by image count

**One image** — image first, one sentence, then template body:

```bash
ISSUE_NUM=37
OWNER_REPO=themadorg/bedrud
MD="$(gh image /tmp/bedrud-issue-img-1.png --repo "$OWNER_REPO")"   # ![file](url)

# Strip leading ![file](url) to rebuild with better alt if needed
IMG_URL="$(printf '%s' "$MD" | sed -n 's/.*](\(.*\))/\1/p')"

{
  echo "![Google Meet shows Also share tab audio when sharing a Chromium tab](${IMG_URL})"
  echo ""
  echo "Google Meet’s tab-share dialog includes the Also share tab audio toggle (enabled here)."
  echo ""
  # remainder of template body (Description, Expected, …) without a Screenshots section
  cat /tmp/bedrud-issue-body-rest.md
} > /tmp/bedrud-issue-body-final.md

gh issue edit "$ISSUE_NUM" -R "$OWNER_REPO" --body-file /tmp/bedrud-issue-body-final.md
```

**Multiple images** — full template body + `## Screenshots`:

```bash
# Capture each embed line
mapfile -t MDS < <(gh image /tmp/a.png /tmp/b.png --repo themadorg/bedrud)

{
  cat /tmp/bedrud-issue-body-rest.md
  echo ""
  echo "## Screenshots"
  echo ""
  printf '%s\n\n' "${MDS[@]}"
} > /tmp/bedrud-issue-body-final.md

gh issue edit "$ISSUE_NUM" -R "$OWNER_REPO" --body-file /tmp/bedrud-issue-body-final.md
```

Optional: create the issue with `$(gh image …)` already inlined in `--body` so no edit
pass is needed:

```bash
gh issue create -R themadorg/bedrud --title "..." --body "$(cat <<EOF
![alt]($(gh image /tmp/shot.png --repo themadorg/bedrud | sed -n 's/.*](\(.*\))/\1/p'))

One sentence caption.

## Description
...
EOF
)"
```

(Prefer writing a body file; shell nesting of `gh image` is easy to get wrong.)

Verify: body contains `github.com/user-attachments/assets/` and **not** catbox/imgur/etc.

---

## 7. Quality bar (before approve)

- [ ] Correct template + title prefix
- [ ] **GitHub Issue Type planned** (`Bug` | `Feature` | `Task`) — not labels alone
- [ ] **Priority planned** (`Urgent` | `High` | `Medium` | `Low`)
- [ ] Required template fields filled from user + research (not invented)
- [ ] Ambiguities asked and resolved
- [ ] Duplicate search done
- [ ] Code pointers / paths included where relevant
- [ ] Labels exist on the repo (**kind + surface**, e.g. `bug` + `frontend-web`)
- [ ] Not a private security disclosure
- [ ] User confirmed create (or explicitly waived)
- [ ] **If user shared image(s): listed in preview and plan to upload (§6.1)**

### After create quality bar

- [ ] Issue URL printed
- [ ] **`issueType` is Bug/Feature/Task** (not null) — fix with `gh issue edit --type`
- [ ] **Priority field set** (verify via `issue_field_values`)
- [ ] **All user-shared images on GitHub `user-attachments`** (via `gh image`, never Catbox)
- [ ] **One image → top of body + one sentence; multiple → `## Screenshots`**
- [ ] Captions/alt text match what the screenshot shows

---

## Quick playbooks

### Bug (fast)

1. Classify → Bug Report + **`--type Bug`**
2. Ask: actual, expected, steps, version/env; **severity → Priority**
3. Collect any user screenshots for §6.1
4. `gh issue list --search` + grep code for error paths
5. Draft `[BUG]: …` with Description / Expected / Steps / Logs / Version / Severity
6. Confirm → `gh issue create --type Bug` → set Priority → upload images if any

### Feature (fast)

1. Classify → Feature Request + **`--type Feature`**
2. Ask: problem, priority, surface (web/server/…)
3. Research existing related APIs/UI; note gaps
4. Draft `[FEATURE]: …` with Problem / Solution / Alternatives / Impact / Priority
5. Confirm → create `--type Feature` + labels + Priority field
6. Upload any user mockups/screenshots (§6.1)

### Task from conversation

1. Summarize work as Task + **`--type Task`**
2. Research files; fill Technical Requirements + Acceptance Criteria + Priority
3. Confirm → create → set Priority → attach images

---

## Anti-patterns

- Filing without reading `.github/ISSUE_TEMPLATE/*.yaml`
- Inventing reproduction steps or severity
- Public issue for a real vulnerability
- Duplicate of an open issue without checking
- Vague body (“fix auth”) with no paths, behavior, or AC
- Creating labels or assigning people without being asked
- Skipping user confirmation on create
- **Setting only the `bug` label and leaving GitHub Issue Type null**
- **Skipping Priority field** (body “Medium” alone is not enough)
- **Describing a user screenshot in text but not attaching it to the GitHub issue**
- **Creating the issue and forgetting §6.1 image upload**
- **Hosting screenshots on Catbox / Imgur / other non-GitHub CDNs**
- **Single screenshot buried under a `## Screenshots` heading instead of at the top**
- **Multi-sentence essays under a single top screenshot (keep one sentence)**
