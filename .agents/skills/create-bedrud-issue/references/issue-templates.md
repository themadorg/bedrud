# Bedrud issue template field map

Canonical layouts for issue bodies. Prefer re-reading `.github/ISSUE_TEMPLATE/*.yaml`
if this file drifts. `*` = required in the GitHub form.

**Repo:** `themadorg/bedrud`  
**Blank issues:** disabled  
**Shared optional skeleton:** `.github/ISSUE_TEMPLATE.md` (Summary, Background, Scope, AC, DoD, Notes)

---

## Issue Type + Priority + Labels

### GitHub Issue Type (required field — not a label)

| Form template | `gh … --type` |
|---------------|---------------|
| Bug Report | `Bug` |
| Feature Request | `Feature` |
| Task, Documentation, Spike, most Epics | `Task` |
| Performance / Security | `Bug` if defect; else `Task` |

```bash
gh issue create -R themadorg/bedrud --type Bug …
gh issue edit N -R themadorg/bedrud --type Feature
gh api repos/themadorg/bedrud/issue-types --jq '.[].name'
```

### Priority (required Issue Field)

Options: **Urgent** | **High** | **Medium** | **Low**

| Template Severity/Priority text | Field value |
|---------------------------------|-------------|
| Critical | Urgent |
| High | High |
| Medium | Medium |
| Low | Low |
| Unknown | Medium (or ask) |

Set with GraphQL `setIssueFieldValue` after create (see skill §4). Field id
`IFSS_kgDOAk7FTg`; option ids re-query if needed.

```bash
gh api repos/themadorg/bedrud/issues/N \
  --jq '{type: .type.name, priority: [.issue_field_values[]|select(.issue_field_name=="Priority")|.single_select_option.name][0]}'
```

### Kind labels (optional complement to Type)

| Template labels (YAML) | Use if exists | Else |
|------------------------|---------------|------|
| `bug` | `bug` | — |
| `feature` | `feature` | `enhancement` |
| `task` | `task` | omit |
| `documentation` | `documentation` | — |
| `epic` / `performance` / `security` / `spike` | same if present | omit |

### Surface / area (prefer over language labels)

| Label | Scope |
|-------|--------|
| `frontend-web` | `apps/web` — React meeting UI, screen share, chat, stage |
| `backend-core` | `server/` — API, auth, rooms, queue, LiveKit embed |
| `android` | `apps/android` |
| `desktop-windows` | Desktop — Windows |
| `desktop-mac` | Desktop — macOS |
| `desktop-linux` | Desktop — Linux |

Apply **Type + Priority + kind label + surface** (e.g. type `Bug`, priority
`Medium`, labels `bug` + `frontend-web`).

Always: `gh label list -R themadorg/bedrud` before applying labels.

---

## Bug Report (`bug.yaml`)

- **Template name:** `Bug Report`
- **Title:** `[BUG]: …`
- **Labels:** `bug`

```markdown
## Description

<!-- * What happened -->

## Expected Behavior

## Steps to Reproduce

1.
2.

## Logs / Errors

```text
```

## Version

## Severity

<!-- Critical | High | Medium | Low | Unknown -->
```

CONTRIBUTING also wants: steps, expected vs actual, environment (OS, browser, app version) — fold env into Description or Version.

**Images (skill §6.1 — GitHub `user-attachments` only, never Catbox):**

- **One screenshot:** image at **top of body**, then **one sentence**, then the template sections above. No `## Screenshots`.
- **Multiple screenshots:** after Description (or Logs), add:

```markdown
## Screenshots

![caption one](https://github.com/user-attachments/assets/…)

![caption two](https://github.com/user-attachments/assets/…)
```

---

## Feature Request (`feature.yaml`)

- **Template name:** `Feature Request`
- **Title:** `[FEATURE]: …`
- **Labels:** `feature` → fallback `enhancement`

```markdown
## Problem Statement

<!-- * What problem does this solve? -->

## Proposed Solution

## Alternatives Considered

## Impact

## Priority

<!-- Critical | High | Medium | Low | Unknown -->
```

---

## Task (`task.yaml`)

- **Template name:** `Task`
- **Title:** `[TASK]: …`
- **Labels:** `task` (often missing on remote — omit if absent)

```markdown
## Summary

<!-- * What needs to be done -->

## Background / Context

## Scope

### In scope
-
### Out of scope
-

## Technical Requirements

<!-- paths, APIs, config keys from research -->

## Acceptance Criteria

- [ ] Implementation completed
- [ ] Tests added
- [ ] Documentation updated
- [ ] Code reviewed
- [ ] <!-- extra criteria -->

## Priority

<!-- High / Medium / Low -->

## Estimate

<!-- optional: 2h / 1d / points -->
```

May merge sections from `ISSUE_TEMPLATE.md` (Definition of Done) when useful.

---

## Documentation (`docs.yaml`)

- **Template name:** `Documentation`
- **Title:** `[DOCS]: …`
- **Labels:** `documentation`

```markdown
## Documentation Issue

<!-- * What is wrong or missing -->

## Location

<!-- path, URL, or doc section -->

## Proposed Changes
```

---

## Epic (`epic.yaml`)

- **Template name:** `Epic`
- **Title:** `[EPIC]: …`
- **Labels:** `epic` (often missing — omit if absent)

```markdown
## Goal

<!-- * -->

## Scope

## Child Tasks

- [ ] #… or planned task titles

## Success Criteria
```

Child work: create separate Task issues with `gh issue create --parent <epic-number>` when supported.

---

## Performance Issue (`performance.yaml`)

- **Template name:** `Performance Issue`
- **Title:** `[PERF]: …`
- **Labels:** `performance` (often missing — omit if absent)

```markdown
## Problem Description

<!-- * What is slow / heavy -->

## Current Metrics

<!-- latency, CPU, memory, or Unknown -->

## Target Metrics
```

---

## Security Issue (`security.yaml`)

- **Template name:** `Security Issue`
- **Title:** `[SECURITY]: …`
- **Labels:** `security` (often missing — omit if absent)

**Public only** for hardening / defense-in-depth. Real vulns → private advisory, not this form.

```markdown
## Description

<!-- * Public-safe security improvement -->

## Severity

<!-- Critical | High | Medium | Low | Unknown -->

## Mitigation Plan
```

---

## Spike / Research (`spike.yaml`)

- **Template name:** `Spike / Research`
- **Title:** `[SPIKE]: …`
- **Labels:** `spike`, `research` (often missing — omit if absent)

```markdown
## Research Question

<!-- * -->

## Goals

## Expected Deliverables

<!-- e.g. design note, prototype, recommendation -->
```

---

## `gh` examples

```bash
gh issue create -R themadorg/bedrud \
  --title "[FEATURE]: …" \
  --body-file /tmp/bedrud-issue-body.md \
  --label "enhancement"

gh issue create -R themadorg/bedrud \
  --title "[BUG]: …" \
  --body-file /tmp/bedrud-issue-body.md \
  --label "bug"

# Optional template flag (name = YAML `name:`)
gh issue create -R themadorg/bedrud --template "Feature Request" ...
```
