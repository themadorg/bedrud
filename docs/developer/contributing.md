# Contributing

How to submit changes to Bedrud. Also see [`CONTRIBUTING.md`](../../CONTRIBUTING.md) at the repo root.

---

## Workflow

1. Fork the repository on GitHub
2. Clone your fork: `git clone https://github.com/<you>/bedrud.git`
3. Create a branch from `master`:
   ```bash
   git checkout -b feature/my-feature
   ```
4. Make changes, commit, push
5. Open a pull request against `themadorg/bedrud` `master`

---

## Branch naming

| Prefix | Use |
|--------|-----|
| `feature/` | New functionality |
| `fix/` | Bug fixes |
| `docs/` | Documentation only |
| `refactor/` | Code structure, no behavior change |

---

## Commit messages

Format: `<action> <what> for <why>`

Actions: `add`, `update`, `delete` (lowercase).

```
add room archive endpoint for soft-delete support
update auth middleware for email verification gate
delete legacy migration for schema v2 cleanup
fix guest join rate limit for proxy deployments
```

Use complete sentences. Describe what changed and why.

---

## Code style

| Language | Tool | Command |
|----------|------|---------|
| Go | `gofmt` | Automatic on save; `make fmt` at repo root |
| TypeScript/React | Biome | `cd apps/web && bun run lint` |
| Kotlin | Android Studio defaults | — |
| Swift | Xcode defaults | — |
| Python (agents) | ruff | Per agent `requirements.txt` |

### Go conventions (server)

- **Repository pattern** — handlers never query DB directly
- **No `fmt.Println`** in production code — use Zerolog (`log.Info()`, `log.Error()`)
- **Error responses** — `c.Status(...).JSON(fiber.Map{"error": "..."})`
- **File naming** — snake_case (`room_handler.go` is rare; usually `room.go` in handlers/)
- **Tests** — `*_test.go` next to source; use `testutil.SetupTestDB()` for DB tests

### TypeScript conventions (web)

- Path alias `#/*` never `../src/*`
- shadcn/ui components from `@/components/ui/`
- No inline `style={}` for static values — Tailwind + `cn()`
- See [`apps/web/AGENTS.md`](../../apps/web/AGENTS.md) and [`DESIGN.md`](../../DESIGN.md)

---

## Before submitting a PR

Run the same checks CI runs:

```bash
# Server (required for any backend change)
cd server
go vet ./...
go build ./...
go test -v -count=1 ./...

# Web (required for any frontend change)
cd apps/web
bun run check
bun run build

# Site (if you changed apps/site)
cd apps/site
bun run check
bun run typecheck:astro
bun run build
```

Optional but recommended:

```bash
cd server && golangci-lint run
```

---

## PR description

Include:

- **What** changed (bullet list)
- **Why** (problem or feature motivation)
- **How to test** (steps a reviewer can follow)
- **Screenshots** for UI changes

---

## Documentation changes

- Server reference: `docs/server/`
- Developer guides: `docs/developer/`
- User-facing docs site: `apps/site/src/content/docs/en/` (+ sidebar in `sidebar.ts`)

If you add a user-facing doc page, add a sidebar entry in `apps/site/src/content/docs/sidebar.ts`.

---

## Scope guidelines

- Keep PRs focused — one feature or fix per PR when possible
- No drive-by refactors unrelated to the task
- Match existing patterns in the file you're editing

---

## License

Contributions are licensed under [Apache License 2.0](../../LICENSE).