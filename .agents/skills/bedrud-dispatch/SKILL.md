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
| model, schema, migration, GORM, DB, repository, test db, DTO, SQLite, Postgres, stats, verification event | `bedrud-data` |
| auth, login, register, JWT, token, passkey, WebAuthn, OAuth, middleware, rate limit, session, cookie | `bedrud-auth` |
> **TODO oncoming feature:** Recording functionality is planned for a future release.
| handler, route, HTTP, endpoint, server bootstrap, entrypoint, server.go, main.go, Fiber, LiveKit webhook, lkutil, cert handler, overview handler, preferences handler, recording handler, recordings enabled middleware | `bedrud-http` |
| queue, job, worker, scheduler, cron, background, async, cleanup service, room cleanup, chat upload, storage, SMTP, email (sending), dispatch webhook, process recording, webhook handler, recording handler | `bedrud-jobs` |
| email template, Cerberus, HTML email, dark mode email, hybrid grid, email design, Outlook email, transactional email template | `bedrud-email-cerberus` |
| embedded livekit, livekit binary, livekit server, TURN, TLS setup, node IP, realtime | `bedrud-realtime` |
| install, uninstall, debian, systemd, OpenRC, CLI user, promote, demote, TLS cert, key gen, utils, outbound IP, safe I/O | `bedrud-ops-cli` |

## Frontend Tasks

| Task keywords | Load skill |
|---------------|------------|
| route, router, TanStack Router, API client, HTTP fetch, build config, vite, tsconfig, biome, package.json, types, handle-auth-success, admin overview hook, queue stats hook, WebAuthn helper | `bedrud-fe-platform` |
| Zustand, store, state, auth store, user store, theme store, audio preferences, recent rooms, video preferences, participant overrides | `bedrud-fe-state` |
| meeting, LiveKit room, chat, participant tile, grid, spotlight, screen share, controls, audio processor, RNNoise, meeting sounds, chat grouping, MeetingProvider, MeetingContext | `bedrud-fe-meeting` |
| admin dashboard, admin route, user table, room table, queue stats, overview widget, settings tab, invite token, admin guard | `bedrud-fe-admin` |
| UI, shadcn, component, style, theme.css, Tailwind, cn(), error parser, palette, avatar, button, dialog, card, input, design system | `bedrud-fe-ui-foundation` |

## API Reference Tasks

| Task keywords | Load skill |
|---------------|------------|
| API auth, JWT flow, register, login, passkey endpoint, verify email, OAuth, preferences, public settings, health | `bedrud-api-auth` |
| API room, create room, join, guest join, moderation, kick, ban, mute, promote, demote, online count, chat upload | `bedrud-api-rooms` |
| API admin, admin user, admin room, admin queue, admin settings, invite token admin, bulk action, overview endpoint | `bedrud-api-admin` |
| DTO, type definition, struct, Go type, request shape, response shape, source file index, Swagger, Scalar | `bedrud-api-types` |

## Fallback

If unclear, load `bedrud-http` (most common task target) + `bedrud-data` (foundation).
