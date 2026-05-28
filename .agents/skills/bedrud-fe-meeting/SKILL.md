---
name: bedrud-fe-meeting
description: Live meeting room — components, chat, audio processing, rendering architecture.
license: Apache License
---

# Bedrud Frontend Meeting

React 19 SPA. `apps/web/`. LiveKit Components + Web Audio API.

---

## Meeting Context — Architecture

`MeetingProvider` creates two nested React contexts for render isolation:

**Room context** (`MeetingRoomContext`) — slow-changing metadata:
- `roomId`, `roomName`, `adminId`, `currentUserId`
- `isCreator`, `isAdmin`, `isModerator` (derived from accesses)
- `isServerDeafened`, `isSelfDeafened`
- `toggleSelfDeafen()` — mute mic + broadcast via participant metadata
> **TODO oncoming feature:** Recording functionality is planned for a future release.
- **Recording state:** `isRecording`, `isRecordingStarting`, `isRecordingStopping`, `toggleRecording()` — start/stop via `/api/rooms/{id}/recording/start|stop`. `recordingsAllowed` (room-level gate), `recordingsEnabled` (system-level setting)

**Chat context** (`MeetingChatContext`) — fast-changing messages:
- `chatMessages: ChatMessage[]`, `systemMessages: SystemMessage[]`
- `sendChat(text, attachments?)` — `localParticipant.publishData()` + local echo
- `unreadCount`, `markRead()`

**Data channel:** `RoomEvent.DataReceived` for topics `"chat"` and `"system"`.
**System events:** kick, ban, ask_unmute, ask_camera, spotlight, deafen, undeafen, room_deleted, room_ended, room_closed.

---

## Components

| Component | File | Props | Purpose |
|-----------|------|-------|---------|
| `MeetingProvider` | `MeetingContext.tsx` | `roomId, roomName, adminId, children` | Root meeting context |
| `ParticipantTile` | `ParticipantTile.tsx` | `participant, totalCount, index, isPinned?, onTogglePin?` | Video/avatar tile |
| `ParticipantGrid` | `ParticipantGrid.tsx` | `pinnedIdentities: Set, onTogglePin` | Responsive grid (1/2/3/4 cols) |
| `SpotlightView` | `SpotlightView.tsx` | `participant, onClose` | Full-screen 16:9 spotlight |
| `ScreenShareTile` | `ScreenShareTile.tsx` | `trackRef` | Screen share video |
| `FocusLayout` | `FocusLayout.tsx` | `pinnedIdentities: Set, onTogglePin` | Split: main grid + bottom filmstrip |
| `ControlsBar` | `ControlsBar.tsx` | `onLeave` | Floating bottom bar: mic, cam, screen share, deafen, leave, push-to-talk. Shows `RecordingButton` when moderator + `recordingsAllowed && recordingsEnabled` |
| `MeetingControls` | `MeetingControls.tsx` | `onNavigate` | Wraps ControlsBar. Creator: "end for everyone" |
| `MeetingHeader` | `MeetingHeader.tsx` | `meetId` | Top bar: LIVE badge, room name, clock, connection status, recording badge (red dot) when active |
| `MeetingPanels` | `MeetingPanels.tsx` | `navigate` | Side panel toggling (chat/participants/room info). Lazy-loaded. Info toggle button between chat & participants |
| `MeetingSoundEffects` | `MeetingSoundEffects.tsx` | — | Null-render. Join/leave beeps, chat ding, muted-mic speech beep |
| `ChatPanel` | `ChatPanel.tsx` | `onClose` | Right sliding panel: header + message list + input |
| `ChatToastNotifier` | `ChatToastNotifier.tsx` | `chatOpen` | Floating toasts for incoming chat when panel closed |
| `KickDetector` | `KickDetector.tsx` | `onKicked` | Null-render. Listens `PARTICIPANT_REMOVED` disconnect |
| `ParticipantsList` | `ParticipantsList.tsx` | `onClose` | Right panel: avatar, name, role badges, mic/cam icons |
| `ParticipantContextMenu` | `ParticipantContextMenu.tsx` | `participant, isPinned?, onTogglePin?` | Right-click context menu |
| `ParticipantMenuButton` | `ParticipantContextMenu.tsx` | `participant, isPinned?, onTogglePin?` | 3-dot dropdown for participants |
| `ParticipantMenuContent` | `ParticipantContextMenu.tsx` | `participant, isPinned?, onTogglePin?`, render props | Role mgmt, kick/ban, mute/volume, WebRTC stats |
| `AskActionBanner` | `AskActionBanner.tsx` | — | Fixed bottom banner: mod ask_unmute/ask_camera |
| `BeforeUnloadLock` | `BeforeUnloadLock.tsx` | — | Prevent tab close during meeting |
| `AudioProcessorManager` | `AudioProcessorManager.tsx` | — | Attach noise suppression on connect, switch mid-meeting |
| `DeviceSelector` | `DeviceSelector.tsx` | `kind: 'audioinput'|'videoinput'|'audiooutput'` | Media device selection dropdown |
| `MeetingErrorBoundary` | `MeetingErrorBoundary.tsx` | `children` | Error boundary wrapping LiveKit room |
| `SecureContextBanner` | `SecureContextBanner.tsx` | — | Warning when not in secure context |
| `RecordingButton` | `RecordingButton.tsx` | `isRecording, isStarting, isStopping, onToggle, isMobile?` | Red circle/stop button. Spinner during transition. Pulsing red indicator dot when active |
| `RecordingList` | `RecordingList.tsx` | `recordings: RecordingItem[]` | Recording list with type badge, duration, file size, download button (ready) / spinner (processing) / failed badge. Filters transient states |
| `RoomInfoPanel` | `RoomInfoPanel.tsx` | `roomId, onClose` | Room metadata panel. Fetches recordings from `/api/rooms/{roomId}/recordings` (10s polling). Uses `RecordingList` internally |

### Chat Components — `src/components/meeting/chat/`

| Component | File | Props | Purpose |
|-----------|------|-------|---------|
| `ChatInput` | `ChatInput.tsx` | `onSend, onUpload, disabled?` | Auto-resize textarea, image attach, paste |
| `ChatMessageList` | `ChatMessageList.tsx` | `chatMessages, systemMessages, onScrollUnreadChange, onDrop` | Auto-follow scroll, date separators, drag-drop |
| `ChatMessageCluster` | `ChatMessageCluster.tsx` | `cluster: ClusterGroup` | Telegram-style bubble cluster |
| `ChatScrollManager` | `ChatScrollManager.tsx` | `show, unreadCount, onScrollToBottom` | Floating arrow + unread badge |

---

## Meeting Entry Flow — `m.$meetId.tsx`

The meeting route (`/m/$meetId`) handles both joining active rooms and reclaiming archived rooms.

### Join Sequence

1. **Authenticated user** → `POST /api/room/join {roomName}`
2. **Guest user** → show name dialog → `POST /api/room/guest-join {roomName, guestName}`
3. **Success** → `joinData` state → render `<LiveKitRoom>` with token

### Archived Room Recreate Flow

When `JoinRoom` returns `{status:"archived_owned", name, mode, settings}` (the archived room was created by current user), the page shows a dialog instead of connecting to LiveKit:

- **Dialog:** "This meeting has ended — `{slug}` was created by you. Start a new meeting with this name?"
- **"Start new meeting"** → `POST /api/room/create {name, mode, settings}` → navigate to `/m/{slug}` (fresh meeting)
- **"Dashboard"** → navigate away

**State:** `archivedRoom` (null or `{name, mode, settings}`) — checked before `joinError` / `joinData` in render tree.

**Edge cases:**
- Non-creator clicking archived room link → backend returns 410 Gone → `joinError` → `ErrorPage`
- Room hard-deleted (`Purge:true`) → `GetRoomByName` returns nil → 404 → `joinError`
- CreateRoom with archived slug → uniqueness check skips `deleted_at IS NOT NULL` rows → name is free

---

## Recording

> **TODO oncoming feature:** Recording functionality is planned for a future release.

Recording controlled via meeting room context state. Two gates: `recordingsAllowed` (room-level, from `Room.Settings.RecordingsAllowed`) and `recordingsEnabled` (system-level, from `SystemSettings.RecordingsEnabled`). Both must be true for moderator to see recording button.

**API calls:**
- `POST /api/rooms/{roomId}/recording/start` — starts recording (called by `toggleRecording()`)
- `POST /api/rooms/{roomId}/recording/stop` — stops recording
- `GET /api/rooms/{roomId}/recordings` — fetches recording list (polled by `RoomInfoPanel` every 10s)

**RecordingItem shape:**
```
{ id, recordingType, durationMs, fileSize, fileUrl?, status:'pending'|'started'|'processing'|'completed'|'failed',
  error?, downloadStatus:'processing'|'ready'|'failed' }
```

**UI visibility:**
- `RecordingButton` — in `ControlsBar`, only for moderators when both gates pass. Circle/stop icon, spinner during transition, pulsing red dot when active
- Recording badge — in `MeetingHeader`, visible to **all** participants when `isRecording && recordingsAllowed && recordingsEnabled`
- `RoomInfoPanel` — opened via info toggle in `MeetingPanels`. Shows room ID + `RecordingList` (download buttons for completed, spinner for processing, failed badge for errors)

## Audio Processing

### `src/lib/audio-processor.service.ts`

`AudioProcessorService` (class singleton). Manages noise suppression lifecycle on LiveKit `LocalAudioTrack`.
Methods: `attach(track, mode)`, `switchMode(mode, opts?)`, `setEchoCancellation(enabled)`, `detach()`.
Dynamic imports: `@livekit/krisp-noise-filter`, `rnnoise-processor`.
`AudioProcessorService.isKrispSupported()` — Chromium + AudioWorklet + secure context.

### `src/lib/rnnoise-processor.ts`

`RNNoiseProcessor` — LiveKit `TrackProcessor` using RNNoise WASM via AudioWorklet. FRAME_SIZE=480 (10ms@48kHz). Dynamic import `@jitsi/rnnoise-wasm` (~8MB, code-split).

### `src/lib/meeting-sounds.ts`

Synthesized via Web Audio API. Singleton AudioContext.
`playJoin()` — rising chime (660→880Hz). `playLeave()` — descending (660→440Hz).
`playChat()` — soft pop (1200→1500Hz). `playMutedBeep()` — short buzz (340Hz square).

### `src/lib/chatGrouping.ts`

`groupMessages(chatMsgs, systemMsgs)` — Flat arrays → `DisplayItem[]`. 5-min gap = new cluster.
Helpers: `avatarColor(identity)`, `avatarInitials(name)`, `relativeTime(ts)`, `absoluteTime(ts)`.

---

## Component Dependency Graph

```
MeetingPanels
├── MeetingControls
│   └── ControlsBar → DeviceSelector + RecordingButton (moderator only)
├── ChatPanel (lazy)
│   ├── ChatInput
│   └── ChatMessageList
│       ├── ChatMessageCluster
│       └── ChatScrollManager
├── ParticipantsList (lazy)
│   └── ParticipantContextMenu / ParticipantMenuButton
│       └── ParticipantMenuContent
├── RoomInfoPanel
│   └── RecordingList (fetches /api/rooms/{id}/recordings with 10s polling)
├── ChatToastNotifier
└── MeetingSoundEffects (null)

ParticipantGrid → ParticipantTile → ParticipantContextMenu / ParticipantMenuButton
FocusLayout → ParticipantTile + ScreenShareTile

Standalone (no custom deps):
  AskActionBanner, BeforeUnloadLock, MeetingErrorBoundary, AudioProcessorManager,
  KickDetector, MeetingHeader (renders recording badge via context), SpotlightView,
  SecureContextBanner, ScreenShareTile, RecordingButton
```

---

## Styles

Meeting-specific CSS in `components/meeting/meeting.css`:
- `.meet-tile` — base tile, `position: relative; overflow: hidden`
- `.meet-tile.meet-speaking` — speaking glow via primary-colored box-shadow
- `@keyframes meet-speak-bar` — waveform animation (4px ↔ 18px)
- `@keyframes meet-connecting-spin` — loading spinner
- `.meet-chat-scroll` — custom thin scrollbar

Meeting room colors: `bg-[#0c0c16]/90`, `text-white/*`, `border-white/[0.07]`.
Inline `style={}` only for: `color-mix`, palette-based avatar colors, computed dimensions, `isSpeaking` animation delays.
