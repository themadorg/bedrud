---
name: bedrud-fe-admin
description: Admin dashboard — route guards, data tables, overview widgets, settings tabs.
license: Apache License
---

# Bedrud Frontend Admin

React 19 SPA. `apps/web/`. TanStack Query 5 + Recharts.

---

## Admin Routes

All under `/dashboard/admin/` layout with `beforeLoad` guard redirecting non-admin users to `/dashboard`.

| Route | File | Purpose |
|-------|------|---------|
| `/dashboard/admin` | `admin/index.tsx` | Overview: KPI cards, activity chart, recent signups, room breakdown |
| `/dashboard/admin/queue` | `admin/queue.tsx` | Queue stats dashboard |
| `/dashboard/admin/rooms` | `admin/rooms.tsx` | Rooms table: search, sort, pagination, force-close |
| `/dashboard/admin/rooms/$roomId` | `admin/rooms_.$roomId.tsx` | Room detail: stats, bitrate chart, participants table |
| `/dashboard/admin/rooms/events` | `admin/rooms_.events.tsx` | Room events log with type filter |
| `/dashboard/admin/users` | `admin/users.tsx` | Users table: search, active/banned toggle, promote |
| `/dashboard/admin/users/$userId` | `admin/users_.$userId.tsx` | User detail: hero card, actions, rooms list |
| `/dashboard/admin/users/recent-signups` | `admin/users_.recent-signups.tsx` | Recent signups with provider/date filters |
| `/dashboard/admin/settings` | `admin/settings.tsx` | System settings: 7 tabs |

---

## Admin Components — `src/components/admin/`

### Data Tables

| Component | File | Purpose |
|-----------|------|---------|
| `UserTable` | `UserTable.tsx` | Users table: provider badge, role, active toggle, skeleton |
| `RoomTable` | `RoomTable.tsx` | Rooms table: visibility badge, status, max participants |
| `RoomEventsTable` | `RoomEventsTable.tsx` | Room events log with type filter, pagination |
| `RecentSignupsTable` | `RecentSignupsTable.tsx` | Recent user signups table |
| `DataTableSearch` | `DataTableSearch.tsx` | Search input for data tables |
| `DataTablePagination` | `DataTablePagination.tsx` | Pagination controls |
| `DataTableFacetedFilter` | `DataTableFacetedFilter.tsx` | Faceted filter dropdown |
| `DataTableFilterChips` | `DataTableFilterChips.tsx` | Active filter chips |
| `DataTableToolbar` | `DataTableToolbar.tsx` | Toolbar container |
| `DataTableBulkBar` | `DataTableBulkBar.tsx` | Bulk selection indicator |
| `useTableState` | `useTableState.ts` | Hook: pagination, sorting, search, filter state |

### Action Components

| Component | File | Purpose |
|-----------|------|---------|
| `AdminBulkBar` | `AdminBulkBar.tsx` | Bulk action bar for room/user tables |
| `AdminControlBar` | `AdminControlBar.tsx` | Admin room list control bar |
| `AlertConfirmDialog` | `AlertConfirmDialog.tsx` | Confirmation dialog for destructive actions |
| `RowActionsDropdown` | `RowActionsDropdown.tsx` | Row-level actions dropdown |
| `QueueStatsPage` | `queue-stats.tsx` | Queue health dashboard |

### Overview Widgets — `admin/overview/`

| Component | Purpose |
|-----------|---------|
| `KpiCard` | Single KPI metric card (value, label, trend) |
| `KpiRow` | Horizontal row of KPI cards |
| `ActivityChart` | Room creation area chart (7d, Recharts) |
| `HealthStrip` | Server health indicators |
| `NeedsAttention` | Items needing admin attention |
| `RecentEvents` | Recent system events feed |
| `RecentSignups` | Recent user signups widget |
| `RoomComposition` | Room type/status breakdown |
| `DetailTable` | Generic detail table |

### Settings Tabs — `admin/settings/`

| Component | Purpose |
|-----------|---------|
| `GeneralTab` | Registration mode, invite tokens |
| `AuthTab` | Passkeys, OAuth provider config |
| `LiveKitTab` | LiveKit host, API key/secret |
| `ServerTab` | Port, TLS, ACME, proxy |
| `CORSTab` | CORS origins, headers, methods |
| `ChatTab` | Upload backend, max size, S3 config |
| `LoggingTab` | Log level |
| `InviteTokensSection` | Invite token list + create + delete |
| `shared.tsx` | Shared helpers, masked input fields |
