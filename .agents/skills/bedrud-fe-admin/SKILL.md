---
name: bedrud-fe-admin
description: Admin dashboard — route guards, data tables, overview widgets, settings tabs, queue, recordings stub.
license: Apache License
---

# Bedrud Frontend Admin

React 19 SPA. `apps/web/`. TanStack Query 5 + Recharts. Path alias `#/*` → `./src/*`.

---

## Access model

| Role | Admin shell | Mutations | Settings |
|------|-------------|-----------|----------|
| `superadmin` / `admin` | yes | full | yes |
| `moderator` | yes (`isReadOnly`) | blocked in UI | redirected away |
| others | redirected → `/dashboard` | — | — |

**Layout guard** — `routes/dashboard/admin.tsx`:
- `loader`: fetch `/api/auth/me` if needed; allow `superadmin` | `admin` | `moderator`; else → `/dashboard` (unauth → `/auth`)
- `AdminContext`: `{ isReadOnly, isModerator, currentUserId }` via `useAdminContext()`
- Moderators: `isReadOnly: true` (kick/mute/suspend/delete/role UI hidden)

**Settings extra guard** — `admin/settings.tsx` `beforeLoad`: moderators and non-admin → `/dashboard/admin`

**Sidebar nav** — `routes/dashboard.tsx` `ADMIN_NAV`: Overview, Queue, Users, Rooms, Settings. Recordings nav entry commented (`// TODO oncoming feature`).

---

## Admin Routes

All under `/dashboard/admin/` (layout `admin.tsx` → `<Outlet />`).

| Route | File | Purpose |
|-------|------|---------|
| `/dashboard/admin` | `admin/index.tsx` | Overview → `<AdminOverviewPage />` |
| `/dashboard/admin/queue` | `admin/queue.tsx` | Queue stats → `<QueueStatsPage />` |
| `/dashboard/admin/recordings` | `admin/recordings.tsx` | **Stub** — “Coming in a future release” |
| `/dashboard/admin/rooms` | `admin/rooms.tsx` | Rooms list: filters, bulk suspend/close, create room (`?create=true`) |
| `/dashboard/admin/rooms/$roomId` | `admin/rooms_.$roomId.tsx` | Live room detail: meta, bitrate chart (3s poll), participants mute/kick, suspend/delete, persistent toggle |
| `/dashboard/admin/rooms/events` | `admin/rooms_.events.tsx` | Room events log: type/date/search + `RoomEventsTable` |
| `/dashboard/admin/users` | `admin/users.tsx` | Users list: filters, bulk ban/promote/delete |
| `/dashboard/admin/users/$userId` | `admin/users_.$userId.tsx` | User detail: hero, role/status, force-logout, delete, rooms + sessions tabs |
| `/dashboard/admin/users/recent-signups` | `admin/users_.recent-signups.tsx` | Recent signups: provider/date filters + `RecentSignupsTable` |
| `/dashboard/admin/settings` | `admin/settings.tsx` | System settings (per-tab draft + save) |

**Route file note:** pathless-underscore files (`rooms_.$roomId`) map to nested URL segments without nesting layouts.

---

## Key APIs (by page)

| Page | Endpoints |
|------|-----------|
| Overview | `GET /api/admin/overview` (poll 60s) |
| Queue | `GET /api/admin/queue` (poll 10s) |
| Rooms list | `GET /api/admin/rooms?limit=1000&status=…`, bulk `POST …/bulk/suspend|close`, create `POST /api/room/create` |
| Room detail | `GET …/rooms/:id/participants` (3s), kick/mute, `POST …/suspend`, `DELETE …/rooms/:id`, `PUT …/rooms/:id` (settings/persistent) |
| Room events | `GET /api/admin/rooms/events?page&limit&q&type&dateFrom&dateTo` |
| Users list | `GET /api/admin/users?limit=1000`, bulk `POST …/bulk/ban|promote|delete` |
| User detail | `GET …/users/:id`, `GET …/sessions?page&limit`, `PUT …/status`, `PUT …/accesses`, `DELETE …/users/:id`, `POST …/force-logout` |
| Recent signups | `GET /api/admin/users/recent?…` |
| Settings | `GET/PUT /api/admin/settings`, validate `POST …/settings/validate`, test email `POST …/settings/send-test-email` |
| Invite tokens | `GET/POST /api/admin/invite-tokens`, delete by id |
| Webhooks | `GET/POST /api/admin/webhooks`, `PUT/DELETE …/:id`, `POST …/:id/test` |
| Cert info | `GET /api/admin/cert-info` (Server tab) |

---

## Admin Components — `src/components/admin/`

### Data table primitives

| Component | File | Purpose |
|-----------|------|---------|
| `DataTableSearch` | `DataTableSearch.tsx` | Search input |
| `DataTablePagination` | `DataTablePagination.tsx` | Page + limit controls |
| `DataTableFacetedFilter` | `DataTableFacetedFilter.tsx` | Multi-select faceted filter |
| `DataTableFilterChips` | `DataTableFilterChips.tsx` | Active filter chips |
| `DataTableToolbar` | `DataTableToolbar.tsx` | Toolbar + chips from table state |
| `DataTableBulkBar` | `DataTableBulkBar.tsx` | Bulk selection bar + action buttons |
| `useTableState` | `useTableState.ts` | Client filter/sort/page/selection for `{ id }[]` items |

**Note:** There are **no** standalone `UserTable.tsx` / `RoomTable.tsx`. Users/rooms list UIs are inline in the route files.

### Domain tables / pages

| Component | File | Purpose |
|-----------|------|---------|
| `RoomEventsTable` | `RoomEventsTable.tsx` | Events rows (type, room, user, time) |
| `RecentSignupsTable` | `RecentSignupsTable.tsx` | Signup rows → user detail links |
| `RecordingsTable` | `RecordingsTable.tsx` | **Stub** + `RecordingItem` type (future) |
| `QueueStatsPage` | `queue-stats.tsx` | Queue KPIs, depth bar, email queue, recent failures |
| `ProviderBadge` | `ProviderBadge.tsx` | Auth provider badge (local/google/github/guest/passkey) |

### Action / chrome

| Component | File | Purpose |
|-----------|------|---------|
| `AdminBulkBar` | `AdminBulkBar.tsx` | Alternate bulk bar (children slot) |
| `AdminControlBar` | `AdminControlBar.tsx` | Rooms control bar + shared filter opt constants |
| `AlertConfirmDialog` | `AlertConfirmDialog.tsx` | Confirm dialog (used by users bulk) |
| `RowActionsDropdown` | `RowActionsDropdown.tsx` | Room row menu (view/edit/suspend/close/delete) |

---

## Overview Widgets — `admin/overview/`

Composed by `AdminOverviewPage` (`overview/index.tsx`). Data: `useAdminOverview()`.

| Export | File | Purpose |
|--------|------|---------|
| `AdminOverviewPage` | `index.tsx` | Layout: header, health, KPIs, chart+composition, ops panels |
| `AdminHealthStrip` | `health-strip.tsx` | Overall / TLS / realtime / DB / uptime / alerts |
| `AdminKpiRow` | `kpi-row.tsx` | 5 KPIs: totalUsers, onlineNow, totalRooms, activeSessions, pendingActions |
| `KpiCard` | `kpi-card.tsx` | Single KPI + delta |
| `AdminActivityChart` | `activity-chart.tsx` | Lazy-loaded Recharts area: roomsCreated / active / participants |
| `AdminRoomComposition` | `room-composition.tsx` | live / public / private / persistent (+ stale in type) |
| `AdminNeedsAttention` | `needs-attention.tsx` | Severity-sorted attention items |
| `AdminRecentSignups` | `recent-signups.tsx` | Widget list + link to full page |
| `AdminRecentEvents` | `recent-events.tsx` | Widget list + link to room events |
| `AdminDetailTable` | `detail-table.tsx` | **Placeholder** tabs linking to full Users/Rooms pages (not used on overview) |

---

## Settings Tabs — `admin/settings/`

Active tabs in `settings.tsx` `TABS` (9):

| id | Label | Component | Notes |
|----|-------|-----------|-------|
| `general` | General | `GeneralTab` | Reg mode open/invite/closed; **auto-save** via `onPatch`. Embeds `InviteTokensSection` |
| `auth` | Authentication | `AuthTab` | Passkeys, guest login, JWT/session, Google/GitHub/Twitter OAuth |
| `livekit` | LiveKit | `LiveKitTab` | Host, API key/secret, external flag + validate |
| `server` | Server | `ServerTab` | Port/host/domain/TLS/ACME/proxy, limits, cert status |
| `email` | Email | `EmailTab` | Branding, subjects/preheaders, SMTP, send-test-email |
| `cors` | CORS | `CorsTab` | Origins/headers/methods/credentials/maxAge |
| `chat` | Chat | `ChatTab` | Upload backend disk/S3/inline, retention, quotas |
| `logging` | Logging | `LoggingTab` | logLevel |
| `webhooks` | Webhooks | `WebhookSection` | CRUD + test; **no** settings fields (`TAB_FIELDS.webhooks = []`) |

**Not in active TABS (stub code remains):**
- `RecordingsTab` (`recordings-tab.tsx`) — stub placeholder; import still exported from `index.ts`; tab entry + `TabsContent` commented (`// TODO oncoming feature`)
- `SystemSettings` still has `recordingsEnabled`, `recordingMaxDurationMins`, `recordingMaxFileSizeMB` (future)

### Save model
- Per-tab drafts → sticky Save only sends current tab’s changed fields
- General uses immediate `handlePatch` (no draft)
- Local validation: `validateLocalSettings` from `shared.tsx`; server validate button hits `/api/admin/settings/validate`

### Shared settings helpers

| Export | File | Purpose |
|--------|------|---------|
| `Section`, `Field`, `TextInput`, `Toggle`, `ValidateButton`, `validateLocalSettings` | `shared.tsx` | Form chrome + client validation |
| `SystemSettings`, `InviteToken`, `RegMode` | `types.ts` | Settings domain types |
| barrel | `index.ts` | Re-exports tabs + shared + types |

---

## Lib hooks & types

### `src/lib/use-admin-overview.ts`

`useAdminOverview()` → `GET /api/admin/overview`, `refetchInterval: 60_000`.

Key types: `AdminOverview`, `OverviewHealth`, `TLSStatus`, `OverviewKPIs`, `KpiEntry`, `DayActivity`, `RoomComposition`, `AttentionItem`, `RecentUser`, `RoomEvent`, `AdminRoomEvent`, `InstanceInfo`.

### `src/lib/use-queue-stats.ts`

`useQueueStats()` → `GET /api/admin/queue`, `refetchInterval: 10_000`.

`QueueStats`: pending/active/done24h/failed24h/total/maxDepth/oldestPending/recentFailures/rates + email: `pendingEmail`, `failedEmail24h`, `lastSendError`, `lastSendErrorAt`.

### `src/types/admin.ts`

| Export | Purpose |
|--------|---------|
| `AdminUser` | List/detail user DTO |
| `AdminRoom` | List room DTO (+ optional settings, owner fields) |
| `ROLE_OPTS` | superadmin / admin / moderator / user / guest |
| `ROLE_ACCESS_MAP` | role → `accesses[]` payload |
| `detectRole`, `getRoleLabel` | Role helpers for badges/selects |

---

## Page behavior notes

**Rooms list**
- Sub-tabs: All Rooms | Room events
- Filters: visibility, status (also server query), capacity, created
- Bulk: suspend, close (queued jobs)
- Create via `CreateRoomDialog` (`isAdmin`); overview “Create room” navigates with `?create=true`

**Room detail**
- Live participants + rolling bitrate history (60 samples ≈ 3 min)
- Actions gated by `!isReadOnly`: mute, kick, suspend, delete, toggle `isPersistent`
- Recordings section removed (TODO comment)

**Users list**
- Sub-tabs: All Users | Recent sign-ups
- Filters: provider, role, status, created
- Bulk ban/promote/delete; excludes `currentUserId`; delete is queued

**User detail**
- Role `Select` + confirm; active/banned toggle; force-logout + email-confirm delete
- Tabs: Rooms (activity chart + list) | Room Sessions (paginated)

**Settings**
- Superadmin/admin only
- Masked secrets in inputs; OAuth empty → falls back to config.yaml (server-side)

---

## Future / stubs (do not invent full UI)

| Location | Status |
|----------|--------|
| `/dashboard/admin/recordings` | Placeholder page |
| `RecordingsTable` | Placeholder component + `RecordingItem` type |
| `RecordingsTab` | Placeholder; not wired in settings TABS |
| Sidebar Recordings nav | Commented out |
| Webhook event `recording.completed` | Listed, marked TODO |
| Room detail recordings section | Removed with TODO |
