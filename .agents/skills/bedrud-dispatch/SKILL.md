---
name: bedrud-dispatch
description: Skill router — classify task → load correct sub-skill.
license: Apache License
---

# Bedrud Skill Dispatch

Load by task type. Routes to focused leaf skill.

---

## Backend Tasks

| Task keywords | Load skill |
|---------------|------------|
| model, schema, migration, GORM, DB, repository, test db, DTO (Go model), SQLite, Postgres, stats, verification event, Recording model, Webhook model, Job model | `bedrud-data` |
| auth service, JWT, token, passkey, WebAuthn (service), OAuth (Goth service), session store, challenge store, middleware, rate limit, RecordingsEnabled middleware, ban set, token revoke | `bedrud-auth` |
| handler, route, HTTP, endpoint, server bootstrap, entrypoint, server.go, main.go, Fiber, LiveKit webhook handler, lkutil, cert handler, overview handler, preferences handler, recording handler, avatar upload handler, forgot-password handler, reset-password handler | `bedrud-http` |
| queue, job, worker, scheduler, cron, background, async, cleanup service, room cleanup, chat upload storage, RecordingService, RecordingStore, process_recording, recording_delete, SMTP send (handler), dispatch webhook, avatar storage files | `bedrud-jobs` |
| email template, Cerberus, HTML email, dark mode email, hybrid grid, email design, Outlook email, transactional email template | `bedrud-email-cerberus` |
| embedded livekit, livekit binary, livekit server, TURN, TLS setup (LK), node IP, realtime, LIVEKIT_MANAGED, generateTempConfig | `bedrud-realtime` |
| install, uninstall, debian, systemd, OpenRC, SysV, CLI user, promote, demote, roomcli, room list/close/suspend (CLI), TLS cert gen/renew, key gen, utils, outbound IP, safe I/O, invite-token CLI, config CLI, settings CLI, db migrate CLI | `bedrud-ops-cli` |

**Recording (backend):** Handlers + service + store + queue payloads are **shipped**; prod route/worker wiring may still be partial/commented. Route by concern:

| Concern | Skill |
|---------|--------|
| HTTP handler / routes / DTO serialization | `bedrud-http` |
| RecordingService, RecordingStore, process_recording / recording_delete jobs, retention scheduler | `bedrud-jobs` |
| GORM model / repository | `bedrud-data` |
| RecordingsEnabled middleware | `bedrud-auth` |

---

## Frontend Tasks

| Task keywords | Load skill |
|---------------|------------|
| route, router, TanStack Router, API client, HTTP fetch, build config, vite, tsconfig, biome, package.json, types (platform), handle-auth-success, admin overview hook, queue stats hook, WebAuthn helper, excalidraw vendor aliases, vite resolve alias, forgot-password page, reset-password page, auth routes | `bedrud-fe-platform` |
| Zustand, store, state, auth store, user store, theme store, audio preferences, video preferences, recent rooms, participant overrides, experimental preferences, interface preferences, push-to-talk prefs, profile-sync store | `bedrud-fe-state` |
| meeting, LiveKit room, chat, poll, reaction, participant tile, grid, spotlight, screen share, controls, audio processor, RNNoise, Krisp, meeting sounds, chat grouping, MeetingProvider, MeetingContext, whiteboard, Yjs, Excalidraw (runtime), youtube watch, presence cursors, stage (data channel), push-to-talk (PTT), recording button/list scaffolding, m.$meetId | `bedrud-fe-meeting` |
| admin dashboard, admin route, user table, room table, queue stats page, overview widget, settings tab, invite token UI, admin guard, admin recordings stub, RecordingsTab stub | `bedrud-fe-admin` |
| UI, shadcn, component, style, theme.css, Tailwind, cn(), error parser, palette, avatar (UI primitive), button, dialog, card, input, design system, settings panel, ExperimentalSettingsPanel, PushToTalkKeyCapture | `bedrud-fe-ui-foundation` |

**Recording (frontend):** Context/API shells exist; control UI is largely unmounted/null. FE meeting work → `bedrud-fe-meeting`. Admin placeholder page/table/tab → `bedrud-fe-admin`.

**Stage:** Client LiveKit data topic `stage` (youtube \| whiteboard \| screenshare) → `bedrud-fe-meeting`. HTTP stage bring/remove are 501 stubs → `bedrud-http` / `bedrud-api-rooms`.

**Excalidraw:** Vendor aliases / Vite chunks → `bedrud-fe-platform`. Whiteboard bind / Yjs / overlay runtime → `bedrud-fe-meeting`.

---

## API Reference Tasks

| Task keywords | Load skill |
|---------------|------------|
| API auth, JWT flow, register, login, passkey endpoint, verify email, OAuth endpoint, preferences, public settings, health, avatar endpoint, forgot-password, reset-password | `bedrud-api-auth` |
| API room, create room, join, guest join, moderation, kick, ban, mute, promote, demote, online count (room), chat upload, presence, stage stub (HTTP 501), recording start/stop/list (room-scoped) | `bedrud-api-rooms` |
| API admin, admin user, admin room, admin queue, admin settings, invite token admin, bulk action, overview endpoint, admin webhooks, admin recordings | `bedrud-api-admin` |
| DTO, type definition, struct, Go type, request shape, response shape, source file index, Swagger, Scalar, RecordingDTO | `bedrud-api-types` |

---

## Fallback

If unclear, load `bedrud-http` (most common task target) + `bedrud-data` (foundation).
