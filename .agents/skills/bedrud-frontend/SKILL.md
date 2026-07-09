---
name: bedrud-frontend
description: Full React frontend code map. Load for any UI/component/route/state/hook/stylesheet work.
license: Apache License
---

# Bedrud Web Frontend — Full Code Map

React 19 SPA. Root: `apps/web/`. TanStack Router (file-based) + TanStack Query 5 + Zustand 5 + LiveKit Components + TailwindCSS v4 + Biome + Bun.

Path aliases: `#/*` → `./src/*`, `@/*` → `./src/*`. Prefer `#/*`. Never `../src/*`.

## Leaf skills (deep detail)

| Skill | Owns |
|-------|------|
| `bedrud-fe-platform` | Routes map, `lib/api.ts`, Vite ports/proxy, vendor aliases, tooling |
| `bedrud-fe-state` | All 10 Zustand stores |
| `bedrud-fe-meeting` | LiveKit room, chat, stage, whiteboard, YouTube, presence, recording shells |
| `bedrud-fe-admin` | Admin routes, tables, overview, settings tabs, recordings stub |
| `bedrud-fe-ui-foundation` | Design tokens, shadcn primitives, settings panels, hard UI rules |

This umbrella is the scannable index. Prefer the matching leaf for implementation work.

---

## Shadcn / design rules (summary)

- Prefer `@/components/ui/*` wrappers over raw HTML
- `cn()` for dynamic classNames; no template-literal class strings
- No static `style={}` (inline only: `color-mix`, palettes, computed dims)
- **Zero border-radius** globally; no gradient text; max one static radial glow
- No hardcoded hex for structural UI — tokens in `theme.css`
- Design SOTs: `DESIGN.md`, `apps/web/AGENTS.md`, `apps/web/src/theme.css`

Full rules → `bedrud-fe-ui-foundation`.

---

## Entrypoints

| File | Role |
|------|------|
| `src/router.tsx` | `getRouter()` — TanStack Router (`scrollRestoration`, preload intent, default not-found/error) |
| `src/routes/__root.tsx` | Shell: QueryClient, ErrorBoundary, Intl, Sonner, theme flash script, auth `initialize()` |
| `src/routeTree.gen.ts` | Auto-gen — **never edit** (~32 route modules) |
| `scripts/embed.mjs` | Prod bridge: build → SSR shell → copy to `server/frontend/` |

---

## Dev ports & proxy

**Not** 3000 / 8090. Source: `apps/web/vite.config.ts`.

| Port | Role |
|------|------|
| **7070** | Vite / TanStack Start (web) |
| **7071** | Go API (`make dev`) |
| **7072** | Embedded LiveKit |
| **7074** | TanStack devtools event bus |

| Path | Target | Notes |
|------|--------|--------|
| `/api` | `localhost:7071` | API |
| `/uploads` | `localhost:7071` | Chat/media uploads |
| `/livekit` | `127.0.0.1:7072` | **Direct to LiveKit** (`ws: true`, rewrite strips `/livekit`) |

Do **not** chain `/livekit` through the Go API — double WS proxy breaks `/rtc/v1/validate`.

`VITE_API_URL` empty in dev → relative `/api` → proxy → **:7071**.

---

## Routes

### Tree

```
__root__                              QueryClient, theme, Intl, auth init
├── /                                 landing + guest join (no forced auth redirect)
├── /$                                splat → ErrorPage not-found
├── /new                              auth → POST /api/room/create → /m/$meetId
├── /auth                             layout; tokens → /dashboard
│   ├── /auth/                        guest join
│   ├── /auth/login                   email/pass + passkey + OAuth; ?redirect=
│   ├── /auth/register                name/email/pass + invite token
│   ├── /auth/callback                OAuth → GET /api/auth/me
│   ├── /auth/verify                  email verify
│   ├── /auth/forgot-password
│   └── /auth/reset-password
├── /dashboard                        layout; require tokens
│   ├── /dashboard/                   My Rooms + Recent
│   ├── /dashboard/archived/$roomId   archived room (stub)
│   ├── /dashboard/settings           settings layout
│   │   ├── /dashboard/settings/      profile
│   │   ├── /dashboard/settings/security
│   │   ├── /dashboard/settings/audio
│   │   └── /dashboard/settings/video
│   └── /dashboard/admin              admin|superadmin|moderator
│       ├── /dashboard/admin/         overview
│       ├── /dashboard/admin/queue
│       ├── /dashboard/admin/rooms
│       ├── /dashboard/admin/rooms/$roomId
│       ├── /dashboard/admin/rooms/events
│       ├── /dashboard/admin/users
│       ├── /dashboard/admin/users/$userId
│       ├── /dashboard/admin/users/recent-signups
│       ├── /dashboard/admin/recordings   placeholder
│       └── /dashboard/admin/settings     admin/superadmin only
└── /m/$meetId                        live meeting
```

### Map

| File | Path | Type | Guard / loader | Purpose |
|------|------|------|----------------|---------|
| `__root.tsx` | (root) | Layout | — | Shell, providers, theme, auth init |
| `index.tsx` | `/` | Leaf | init auth; me if tokens | Landing + guest join |
| `$.tsx` | `/$` | Leaf | — | Catch-all not-found |
| `new.tsx` | `/new` | Leaf | auth; loader creates room | Instant meeting |
| `auth.tsx` | `/auth` | Layout | tokens → `/dashboard` | Auth chrome |
| `auth.index.tsx` | `/auth/` | Leaf | — | Guest join |
| `auth.login.tsx` | `/auth/login` | Leaf | `redirect?` | Login |
| `auth.register.tsx` | `/auth/register` | Leaf | — | Register |
| `auth.callback.tsx` | `/auth/callback` | Leaf | — | OAuth completion |
| `auth.verify.tsx` | `/auth/verify` | Leaf | token/status/reason | Email verify |
| `auth.forgot-password.tsx` | `/auth/forgot-password` | Leaf | — | Request reset |
| `auth.reset-password.tsx` | `/auth/reset-password` | Leaf | `token` | Set password |
| `dashboard.tsx` | `/dashboard` | Layout | no tokens → `/auth`; me loader | App chrome |
| `dashboard.index.tsx` | `/dashboard/` | Leaf | — | Rooms + recent |
| `dashboard/archived_.$roomId.tsx` | `/dashboard/archived/$roomId` | Leaf | — | Archived stub |
| `dashboard/settings.tsx` | `/dashboard/settings` | Layout | — | Settings tabs |
| `dashboard/settings/index.tsx` | `/dashboard/settings/` | Leaf | — | Profile |
| `dashboard/settings/security.tsx` | `/dashboard/settings/security` | Leaf | — | Password |
| `dashboard/settings/audio.tsx` | `/dashboard/settings/audio` | Leaf | — | Audio prefs |
| `dashboard/settings/video.tsx` | `/dashboard/settings/video` | Leaf | — | Video prefs |
| `dashboard/admin.tsx` | `/dashboard/admin` | Layout | admin/superadmin/**moderator** | Admin shell + `AdminContext` |
| `dashboard/admin/index.tsx` | `/dashboard/admin/` | Leaf | — | Overview |
| `dashboard/admin/queue.tsx` | `/dashboard/admin/queue` | Leaf | — | Queue stats |
| `dashboard/admin/rooms.tsx` | `/dashboard/admin/rooms` | Leaf | — | Rooms table |
| `dashboard/admin/rooms_.$roomId.tsx` | `/dashboard/admin/rooms/$roomId` | Leaf | — | Room detail |
| `dashboard/admin/rooms_.events.tsx` | `/dashboard/admin/rooms/events` | Leaf | — | Room events |
| `dashboard/admin/users.tsx` | `/dashboard/admin/users` | Leaf | — | Users table |
| `dashboard/admin/users_.$userId.tsx` | `/dashboard/admin/users/$userId` | Leaf | — | User detail |
| `dashboard/admin/users_.recent-signups.tsx` | `/dashboard/admin/users/recent-signups` | Leaf | — | Recent signups |
| `dashboard/admin/recordings.tsx` | `/dashboard/admin/recordings` | Leaf | — | Placeholder |
| `dashboard/admin/settings.tsx` | `/dashboard/admin/settings` | Leaf | block moderator | System settings |
| `m.$meetId.tsx` | `/m/$meetId` | Leaf | join in-page | Live meeting |

**Counts:** 32 route modules · 5 layouts (`__root`, `auth`, `dashboard`, `settings`, `admin`) · rest leaves.

### Guards (summary)

| Area | Rule |
|------|------|
| `/auth/*` | Tokens → `/dashboard` |
| `/` | Does **not** redirect authenticated users |
| `/dashboard` | No tokens → `/auth` |
| `/new` | No tokens → `/auth/login?redirect=/new` |
| `/dashboard/admin` | `isSuperAdmin` or access `admin` or `moderator` |
| `/dashboard/admin/settings` | Moderator blocked; need admin/superadmin |

Full route notes → `bedrud-fe-platform`. Admin pages → `bedrud-fe-admin`. Meeting → `bedrud-fe-meeting`.

---

## Stores (Zustand 5) — 10

All in `src/lib/*.store.ts`. Full field/actions → `bedrud-fe-state`.

| File | Hook | Persist | Role |
|------|------|---------|------|
| `auth.store.ts` | `useAuthStore` | manual `auth_remember` / `auth_at` | Tokens + cookie refresh init |
| `user.store.ts` | `useUserStore` | — | `User` incl. `isSuperAdmin` / `isAdmin` |
| `theme.store.ts` | `useThemeStore` | `theme` | light/dark/system + DOM class |
| `audio-preferences.store.ts` | `useAudioPreferencesStore` | `audio-preferences` | NS, EC, AGC, gain, gate, beep, **PTT** |
| `video-preferences.store.ts` | `useVideoPreferencesStore` | `video-preferences` | Webcam mirror |
| `experimental-preferences.store.ts` | `useExperimentalPreferencesStore` | `experimental-preferences` | Whiteboard / YouTube + disclaimer |
| `interface-preferences.store.ts` | `useInterfacePreferencesStore` | `interface-preferences` | Welcome screen |
| `recent-rooms.store.ts` | `useRecentRoomsStore` | `bedrud-recent-rooms` | Cap 20 |
| `participant-overrides.store.ts` | `useParticipantOverridesStore` | — | Local mute/volume Maps |
| `profile-sync.store.ts` | `useProfileSyncStore` | — | `version` bump signal |

Related (not a store): `user-preferences.ts` deep-merges server prefs blobs → `/api/auth/preferences`.

---

## API layer

`src/lib/api.ts` — sole HTTP client:

```
api.get / post / put / delete
API_URL, class ApiError
```

| Concern | Behavior |
|---------|----------|
| Base | `VITE_API_URL` or `''` → proxy **:7071** |
| Auth | Bearer access token; CSRF meta/cookie fallback |
| Credentials | `include` (HTTP-only refresh cookie) |
| 401 | Singleton refresh → retry once → clear + `/auth` |
| Errors | `ApiError` with parsed message |

---

## Meeting (index)

Code root: `src/components/meeting/`. Route: `m.$meetId.tsx`. Deep map → `bedrud-fe-meeting`.

### Provider tree

```
LiveKitRoom
└── MeetingErrorBoundary
    └── MeetingStageProvider          ← exclusive stage (youtube | whiteboard | screenshare)
        └── MeetingProvider           ← room + chat contexts
            └── YoutubeWatchProvider
                └── WhiteboardWatchProvider
                    ├── StageJoinNotifier, BeforeUnloadLock, KickDetector, AskActionBanner
                    ├── AudioProcessorManager, MeetingRoomAudioRenderer, MeetingSoundEffects
                    └── MeetingRoomShell
                        ├── MeetingLayout | YoutubeWatchOverlay | WhiteboardOverlay | StageScreenShareOverlay
                        ├── MeetingPresenceCursors (enabled=false)
                        ├── ParticipantVideoSidebar (stage active)
                        ├── MeetingHeader + MeetingPanels
                        └── YoutubeShareDialog
```

### Stage (data channel — not HTTP)

Topic `stage` (`stage/stageWire.ts` + `MeetingStageContext`). Server stage HTTP is stub.

Kinds: `youtube` | `whiteboard` | `screenshare` — single owner. Wire: `stage_set` / `stage_clear` / `stage_request` / `stage_state` / `stage_youtube_sync`.

### Data-channel topics

| Topic | Purpose |
|-------|---------|
| `chat` | Messages, chunks, reactions, poll votes |
| `system` | kick/ban/ask/spotlight/deafen/room_* |
| `presence` | deafen_state, profile_changed |
| `stage` | Exclusive stage ownership + youtube sync |
| `youtube` | Legacy wire (active sync via stage) |
| `whiteboard` / `whiteboard-yjs` / `whiteboard-pointer` / `whiteboard-follow` | Canvas + Yjs + collab |
| `meeting-pointer` | Grid presence cursors |

### Whiteboard

Experimental (`experimental-preferences.whiteboardEnabled` + disclaimer gate).

- Host claims stage `whiteboard` → `WhiteboardWatchProvider` owns `Y.Doc` + `LiveKitYjsProvider`
- `MeetingSharedWhiteboard` lazy-loads vendored Excalidraw bound via `bindExcalidrawToYDoc`
- Runtime under `meeting/whiteboard/`; vendor is dependency only

### YouTube

Experimental (`youtubeEnabled`). Stage kind `youtube`; host heartbeat ~2.5s; remote drift 1.5s. Files under `meeting/youtube/`.

### Presence

`MeetingPresenceCursors` + `meetingPointerWire` — **wire ready, `enabled = false`**. Same for `MeetingViewportPan` (`enabled = false`).

### Chat (expanded)

Text + image upload (`POST /api/room/{id}/chat/upload`) + polls + reactions + emoji + chunking (>~60KB) + sessionStorage persistence (`chat:{roomId}`). Caps: 400 in-memory / 200 persisted. Components under `meeting/chat/`.

### Recording UI scaffolding

FE keeps context surface + shells; **UI not mounted** in live controls:

| Piece | Status |
|-------|--------|
| `MeetingContext` recording fields / `toggleRecording` | Present; toggle body no-op |
| `recordingsEnabled` / join `recordingsAllowed` | Forced off |
| `RecordingButton` / `RecordingList` | Export exists; **return `null`** |
| ControlsBar / Header | Recording slots removed |
| Intended API | `POST …/recording/start|stop`, `GET …/recordings` |

### Join sequence (brief)

1. `POST /api/room/join` or guest-join  
2. Optional welcome screen (`interface-preferences`)  
3. Prefs one-shot → stores  
4. Reconnect via `refresh-token` + transport fallback (`preferRelay`)

---

## Vendor Excalidraw

Path: `apps/web/src/vendor/excalidraw/` (0.18 hybrid). README documents packages + shims.

| Import | Resolves under `packages/` |
|--------|----------------------------|
| `@excalidraw/excalidraw` | `excalidraw/` |
| `@excalidraw/common` | `common/src/` |
| `@excalidraw/element` | `element/src/` |
| `@excalidraw/math` | `math/src/` |
| `@excalidraw/utils` | `utils/src/` |
| `@excalidraw/fractional-indexing` | `fractional-indexing/src/` |
| `@excalidraw/laser-pointer` | `laser-pointer/src/` |

Aliases: `src/vendor/excalidraw/aliases.ts` → Vite `resolve.alias` + `tsconfig` paths. Platform owns wiring; meeting owns runtime. Biome excludes `src/vendor`.

---

## Admin (index)

Access: admin/superadmin full; **moderator** shell read-only (`isReadOnly`); settings admin-only.

| Route | Purpose |
|-------|---------|
| `/dashboard/admin` | Overview KPIs / health / charts |
| `…/queue` | Queue + email queue stats |
| `…/rooms`, `…/rooms/$roomId`, `…/rooms/events` | Rooms + live detail + events |
| `…/users`, `…/users/$userId`, `…/users/recent-signups` | Users + bulk |
| `…/recordings` | **Placeholder** (“future release”) |
| `…/settings` | 9 active tabs (general…webhooks); RecordingsTab code present but not in TABS |

**Sidebar:** Overview, Queue, Users, Rooms, Settings. Recordings nav **commented out**.

No standalone `UserTable` / `RoomTable` — list UIs live in route files. Data-table primitives + overview widgets under `components/admin/`.

Full → `bedrud-fe-admin`.

---

## Components (scannable)

### App chrome

| Area | Path | Highlights |
|------|------|------------|
| Root | `components/` | `ErrorPage`, `ErrorBoundary`, `ThemeToggle` |
| Auth | `components/auth/` | `PasskeyButton`, `OAuthButtons` |
| Dashboard | `components/dashboard/` | `RoomCard`, `CreateRoomDialog`, `RoomSettingsDialog` |
| Settings | `components/settings/` | `BedrudSettingsDialog` + profile/audio/video/security/experimental/appearance panels (`tone`: default \| meeting) |
| Admin | `components/admin/` | DataTable*, overview/*, settings tabs, QueueStats, RecordingsTable stub |
| Meeting | `components/meeting/` | Shell, tiles, controls, chat/, stage/, whiteboard/, youtube/, presence/ |
| UI | `components/ui/` | **26** shadcn primitives (Button, Dialog, Command, Slider, …) |

### Meeting dependency sketch

```
m.$meetId → LiveKitRoom → MeetingStageProvider → MeetingProvider
  → Youtube/Whiteboard providers → MeetingRoomShell
      ├── Layout / stage overlays
      ├── MeetingPanels → ControlsBar (stage claims, WB/YT menu)
      │                 → ChatPanel (polls, reactions, upload)
      │                 → ParticipantsList
      └── Header / presence (disabled) / video sidebar
```

---

## Lib / hooks (index)

| Area | Modules |
|------|---------|
| Platform | `api`, `handle-auth-success`, `webauthn`, `jwt-user`, `use-public-settings`, `i18n`, `user-preferences`, `useLongPress`, `text-direction` |
| Admin | `use-admin-overview` (60s), `use-queue-stats` (10s), `types/admin.ts` |
| Meeting | `audio-processor.service`, `rnnoise-processor`, `meeting-sounds`, `livekit-publish`, `livekit-transport-type`, `meeting-device-storage`, `push-to-talk-*`, `usePinnedParticipants` |
| UI util | `utils.cn`, `errors.getErrorMessage`, `participant-palette`, `avatar-*` |

---

## Styles / theme

| File | Role |
|------|------|
| `src/theme.css` | Rose primary + teal accent tokens (light/dark) |
| `src/styles.css` | Tailwind v4 + dark variant + zero radius + keyframes |
| `src/components/meeting/meeting.css` | Meet tiles, speak bars, chat scroll, `--meet-*` |
| `src/theme.example-blue.css` | Rebrand template |

---

## Types (brief)

| Type | Where |
|------|--------|
| Store shapes | `lib/*.store.ts` → fe-state |
| `PublicSettings` | `use-public-settings.ts` (incl. chat TTL/count, `recordingsEnabled` flag) |
| `AdminUser` / `AdminRoom` / roles | `types/admin.ts` |
| Chat / system / stage | meeting components + wire modules |

---

## Build / tooling

| Item | Detail |
|------|--------|
| Scripts | `dev`, `build`, `build:embed`, `check` (biome+tsc), `test` (vitest) |
| Manual chunks | meeting-context, react, tanstack, livekit, charts, ui, markdown, **excalidraw-vendor** |
| optimizeDeps | livekit, yjs, roughjs, jotai, … |
| Biome | 2-space, width 120, single quotes, semicolons asNeeded; excludes vendor + css + routeTree |
| shadcn | `components.json` — default style, slate base, Lucide |

---

## Dependency sketch

```
router → routeTree.gen
__root → QueryClient, theme.store, auth.store.initialize, Intl, ErrorBoundary
routes → api.ts → auth.store
      → fe-admin hooks / types
      → m.$meetId → meeting tree (stage/chat/whiteboard/youtube)
vite  → ports 7070/7071/7072, /livekit direct proxy, excalidrawAliases
stores → 10 × lib/*.store.ts
```

---

## Agent dispatch

| Task | Load |
|------|------|
| Routes, ports, API client, embed, vendor aliases | `bedrud-fe-platform` |
| Any Zustand store | `bedrud-fe-state` |
| Meeting / chat / stage / WB / YT / presence / recording shells | `bedrud-fe-meeting` |
| Admin tables, overview, system settings | `bedrud-fe-admin` |
| Tokens, shadcn, settings panels, a11y chrome | `bedrud-fe-ui-foundation` |
| Endpoint shapes | `bedrud-api` (+ api-* leaves) |
