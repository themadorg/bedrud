# Web Development

Guide for the React web app at `apps/web/`.

---

## Stack

| Tool | Purpose |
|------|---------|
| React 19 | UI |
| TanStack Start + Router | SSR, file-based routing |
| TanStack Query | Server state / API caching |
| Zustand | Client state (7 stores) |
| TailwindCSS v4 | Styling |
| shadcn/ui | Components (`components/ui/`) |
| Bun | Package manager + runtime |
| Biome | Lint + format (not ESLint/Prettier) |

---

## Setup

```bash
make init          # installs bun deps
make dev-web       # :3000 with HMR
```

Requires API at `:8090` — run `make dev-api` or `make dev-server` in another terminal, or use `make dev` for both.

---

## Project layout

```
apps/web/
├── src/
│   ├── routes/           # File-based routes (TanStack Router)
│   ├── components/       # React components
│   │   ├── ui/           # shadcn primitives
│   │   └── meeting/      # Live meeting room UI
│   ├── lib/              # api.ts, utils, hooks
│   └── stores/           # Zustand stores
├── routeTree.gen.ts      # Auto-generated — never edit
├── AGENTS.md             # UI conventions (read before UI work)
└── DESIGN.md             # Web-specific design tokens
```

Path alias: `#/*` → `./src/*`

---

## Common tasks

### Add a page

1. Create `src/routes/my-page.tsx` (or nested `src/routes/dashboard/my-page.tsx`)
2. Export route with `createFileRoute`:

```tsx
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/my-page')({
  component: MyPage,
})

function MyPage() {
  return <div>...</div>
}
```

3. `routeTree.gen.ts` regenerates on dev server start

### Call the API

Use `authFetch` from `#/lib/api.ts`:

```tsx
import { useQuery } from '@tanstack/react-query'
import { authFetch } from '#/lib/api'

function useRooms() {
  return useQuery({
    queryKey: ['rooms'],
    queryFn: () => authFetch('/api/room/list'),
  })
}
```

### Add shared types

Prefer `packages/api-types/` for DTOs shared with other TS apps:

```bash
cd packages/api-types
# edit src/index.ts
```

---

## Commands

| Command | Purpose |
|---------|---------|
| `bun run dev` | Dev server |
| `bun run check` | Biome lint + `tsc` (CI) |
| `bun run build` | Production build → `build/` |
| `bun run lint` | Biome check only |
| `bun run lint:fix` | Auto-fix |
| `bun run typecheck` | TypeScript only |

From repo root: `make dev-web`, `cd apps/web && bun run check`.

---

## UI rules (summary)

Read [`apps/web/AGENTS.md`](../../apps/web/AGENTS.md) and [`DESIGN.md`](../../DESIGN.md) before UI work.

- Use shadcn `Button`, `Input`, `Card`, etc. — not raw HTML
- Tailwind tokens (`bg-primary`, `text-muted-foreground`) — no hardcoded hex
- No gradient text, no animated aurora blobs
- `cn()` from `#/lib/utils` for conditional classes
- Meeting room styles in `components/meeting/meeting.css`

### Add a shadcn component

```bash
cd apps/web
bunx shadcn@latest add <component-name>
```

---

## Build integration

Production build is embedded into the Go binary:

```
apps/web/build/*  →  server/frontend/  →  //go:embed in ui.go
```

Always run `make build` (not just `bun run build`) for a deployable single binary.

---

## Meeting whiteboard (Yjs)

Shared Excalidraw canvas sync uses **Yjs** over LiveKit data channel (`whiteboard-yjs` topic), not full-scene JSON broadcast.

See **[Whiteboard Yjs sync](./whiteboard-yjs.md)** for architecture, file map, anti-flicker rules, and debugging.

---

## Agent skills

For detailed frontend maps, load `.agents/skills/bedrud-frontend/SKILL.md` or sub-skills:

- `bedrud-fe-platform` — routing, API client
- `bedrud-fe-state` — Zustand stores
- `bedrud-fe-meeting` — meeting room
- `bedrud-fe-admin` — admin dashboard