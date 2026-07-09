---
name: bedrud-fe-meeting
description: Live meeting room — LiveKit stage, chat (polls/reactions), whiteboard (Yjs + vendored Excalidraw), YouTube watch, presence, audio, recording scaffolding.
license: Apache License
---

# Bedrud Frontend Meeting

React 19 SPA. `apps/web/`. LiveKit Components + Web Audio + client data-channel stage (not HTTP stage stubs).

**Code roots**
| Path | Role |
|------|------|
| `apps/web/src/components/meeting/` | Room shell, tiles, controls, context |
| `…/meeting/chat/` | Chat UI + wire utils (polls, reactions, emoji, persistence) |
| `…/meeting/whiteboard/` | Shared whiteboard runtime (Yjs, LiveKit provider, Excalidraw bind) |
| `…/meeting/youtube/` | YouTube watch overlay + share dialog |
| `…/meeting/presence/` | Grid presence cursors (wire ready; currently disabled) |
| `…/meeting/stage/` | Exclusive stage owner via LiveKit data topic `stage` |
| `apps/web/src/routes/m.$meetId.tsx` | Join/reconnect/welcome + provider tree |
| `apps/web/src/lib/*` | Audio processor, publish readiness, PTT, device storage, sounds |
| `apps/web/src/vendor/excalidraw/` | Vendored Excalidraw 0.18 hybrid (whiteboard only) |

---

## Provider tree (`m.$meetId.tsx`)

```
LiveKitRoom
└── MeetingErrorBoundary
    └── MeetingStageProvider          ← stage wire (exclusive owner)
        └── MeetingProvider           ← room + chat contexts
            └── YoutubeWatchProvider
                └── WhiteboardWatchProvider
                    ├── StageJoinNotifier
                    ├── BeforeUnloadLock, KickDetector, AskActionBanner
                    ├── AudioProcessorManager, MeetingRoomAudioRenderer
                    ├── MeetingSoundEffects
                    └── MeetingRoomShell
                        ├── MeetingLayout | YoutubeWatchOverlay | WhiteboardOverlay | StageScreenShareOverlay
                        ├── MeetingPresenceCursors (disabled)
                        ├── ParticipantVideoSidebar (when stage active)
                        ├── MeetingHeader
                        └── MeetingPanels → ControlsBar / Chat / Participants / RoomInfo
                    └── YoutubeShareDialog
```

Stage content replaces the participant grid (`MeetingLayout` returns `null` when `stage` is set). Overlays render the active stage kind.

---

## Meeting Context — Architecture

`MeetingProvider` → two nested contexts for render isolation.

### Room context (`useMeetingRoomContext`)

| Field | Notes |
|-------|--------|
| `roomId`, `roomName`, `adminId`, `currentUserId` | From join + JWT/user store |
| `isPublic`, `setRoomIsPublic` | Room visibility |
| `isCreator`, `canManageRoomAccess` | Host or superadmin |
| `isAdmin`, `isModerator` | From user `accesses` |
| `isServerDeafened`, `isSelfDeafened`, `toggleSelfDeafen` | Server system msg vs local toggle; mic muted on deafen |
| `isParticipantDeafened`, `getParticipantDisplayName`, `getParticipantAvatarUrl` | Metadata + peer maps |
| `isRecording`, `isRecordingStarting`, `isRecordingStopping`, `toggleRecording` | State exposed; see **Recording** |
| `recordingsAllowed`, `recordingsEnabled` | Dual gate; currently forced off (see Recording) |

### Chat context (`useMeetingChatContext`)

| Field | Notes |
|-------|--------|
| `chatMessages`, `systemMessages` | Caps: 400 chat / 100 system in memory |
| `sendChat(text, attachments?, poll?)` | Reliable DC + local echo + retry |
| `votePoll(messageId, optionId)` | Topic `chat`, type `poll_vote` |
| `reactToMessage(messageId, emoji)` | Topic `chat`, type `reaction` (one emoji per voter, toggle) |
| `unreadCount`, `markRead()` | Panel open → markRead |

**Legacy:** `useMeetingContext()` merges room + chat.

### Data-channel topics (client wire)

| Topic | Purpose |
|-------|---------|
| `chat` (`MEETING_CHAT_TOPIC`) | Chat, chunks, reactions, poll votes |
| `system` | Kick/ban/ask/spotlight/deafen/room_* |
| `presence` | `deafen_state`, `profile_changed` |
| `stage` | Exclusive stage set/clear/sync (youtube \| whiteboard \| screenshare) |
| `youtube` | Legacy/session wire still present; **active YouTube sync is stage `stage_youtube_sync`** |
| `whiteboard` | Scene/session packets (chunked) — secondary to Yjs |
| `whiteboard-yjs` | Yjs sync protocol over LiveKit (primary canvas state) |
| `whiteboard-pointer` | Remote laser/pointer on canvas |
| `whiteboard-follow` | Viewport follow |
| `meeting-pointer` | Grid presence cursors |

**System events:** `kick`, `ban`, `ask_unmute`, `ask_camera`, `spotlight`, `deafen`, `undeafen`, `room_deleted`, `room_ended`, `room_closed`.

**Chat wire types:** `chat` \| `chat_chunk_meta` \| `chat_chunk` \| `reaction` \| `poll_vote`. Chunks when payload > ~60KB (`CHAT_DATA_SAFE_BYTES`).

**Persistence:** `useChatPersistence` → `sessionStorage` key `chat:{roomId}`; TTL from public settings (`chatMessageTTLHours`, default 2160h); cap 200 persisted / 400 initial load.

---

## Stage (LiveKit data channel — not HTTP)

> Server stage HTTP is stub. FE stage is **only** LiveKit topic `stage` (`stage/stageWire.ts` + `MeetingStageContext`).

**Kinds:** `youtube` | `whiteboard` | `screenshare` — single owner at a time.

| Wire type | Role |
|-----------|------|
| `stage_set` | Claim / publish stage |
| `stage_clear` | Owner release |
| `stage_request` / `stage_state` | Late-joiner sync |
| `stage_youtube_sync` | Host playhead/playing updates |

**API surface (`useMeetingStage`)**

| Method | Behavior |
|--------|----------|
| `claimStage(kind, meta?)` | Returns error string if stage taken by other; else publishes `stage_set` |
| `clearStage()` | Owner clears |
| `updateYoutubeStage(playing, currentTime)` | Host sync |
| `isOwner` | Local identity === stage.ownerIdentity |
| `youtubeSyncNonce` | Bumps on remote youtube sync |

| File | Purpose |
|------|---------|
| `stage/MeetingStageContext.tsx` | Provider, publish queue, join-state request |
| `stage/stageWire.ts` | Encode/parse, labels, session keys |
| `stage/StageJoinNotifier.tsx` | Toast for late joiners (20s window) |
| `stage/StageScreenShareOverlay.tsx` | Full-area screenshare when stage kind is screenshare |
| `stage/waitForScreenShare.ts` | Wait for local screen track after claim |

**Controls:** Screen share claims `screenshare` stage; whiteboard/YouTube claim their kinds from ControlsBar → More menu (gated by experimental prefs).

---

## Whiteboard

Experimental feature. Enable: Settings → Experimental → `whiteboardEnabled` (`experimental-preferences` store). One-time disclaimer via `WhiteboardExperimentalGate` / `whiteboardDisclaimerAcknowledged`.

**Runtime model**
1. Host claims stage `whiteboard` → all clients get session from stage.
2. `WhiteboardWatchProvider` creates one `Y.Doc` + `LiveKitYjsProvider` per whiteboard owner (not recreated on `updatedAt` republish).
3. Participants accept/decline session; host auto-accepts after claim.
4. `WhiteboardOverlay` mounts `MeetingSharedWhiteboard` (lazy Excalidraw) bound via `bindExcalidrawToYDoc`.

### Whiteboard files

| File | Purpose |
|------|---------|
| `WhiteboardWatchContext.tsx` | Session, Y.Doc, accept/decline, start/stop |
| `whiteboard-watch-context.ts` | Context types + `useWhiteboardWatch` |
| `WhiteboardOverlay.tsx` | Layout chrome + hooks (pointer/follow/locks) |
| `MeetingSharedWhiteboard.tsx` | Lazy Excalidraw + Yjs bind + menu/pan/cursors |
| `WhiteboardExperimentalGate.tsx` | Disclaimer gate UI |
| `WhiteboardExperimentalDialog.tsx` | Disclaimer dialog |
| `WhiteboardMainMenu.tsx` | Custom Excalidraw main menu actions |
| `whiteboardMenuActions.ts` | Paste, select-all, grid/snap/zen/view/stats |
| `livekitYjsProvider.ts` | Yjs provider over LiveKit (`whiteboard-yjs`) |
| `excalidrawYjsBinding.ts` | Excalidraw ↔ Y.Doc elements/files |
| `yjsWire.ts` | Chunked Yjs binary packets |
| `whiteboardWire.ts` | Scene/session JSON wire + chunking (safe 60KB) |
| `excalidrawSceneUtils.ts` | Scene normalize/signature helpers |
| `useWhiteboardPointerSync.ts` + `whiteboardPointerWire.ts` | Topic `whiteboard-pointer` |
| `useWhiteboardFollowSync.ts` + `whiteboardFollowWire.ts` | Topic `whiteboard-follow` |
| `useWhiteboardElementLocks.ts` + `whiteboardElementLocks.ts` | Y.Map `locks` on shared doc |
| `whiteboardCursorSync.ts` | Local DOM cursor ↔ Excalidraw collaborators |
| `whiteboardRightDragPan.ts` | RMB drag pan |
| `whiteboardKeyboard.ts` | Esc deselect |
| `whiteboardTextDirection.ts` | RTL text alignment |
| `whiteboardToolCursors.ts` | Tool-specific CSS cursors |
| `whiteboardSyncSettings.ts` | Sync-related prefs helpers |
| `whiteboardTeardown.ts` | Release cursors on unmount |

### Vendored Excalidraw

Source: `apps/web/src/vendor/excalidraw/README.md` (0.18 hybrid).

| Package | Path under `packages/` |
|---------|------------------------|
| `@excalidraw/excalidraw` | `excalidraw/` |
| `@excalidraw/common` | `common/src/` |
| `@excalidraw/element` | `element/src/` |
| `@excalidraw/math` | `math/src/` |
| `@excalidraw/utils` | `utils/src/` |
| `@excalidraw/fractional-indexing` | `fractional-indexing/src/` |
| `@excalidraw/laser-pointer` | `laser-pointer/src/` |

Aliases: `src/vendor/excalidraw/aliases.ts` → Vite `vite.config.ts` + `tsconfig.json` paths. Meeting owns runtime wiring; do **not** dump full vendor tree into skills — only integration surface above.

---

## YouTube watch

Experimental: `youtubeEnabled` in experimental preferences.

| File | Purpose |
|------|---------|
| `YoutubeWatchContext.tsx` | Session from stage; share/stop; `publishSync` → stage |
| `youtube-watch-context.ts` | Context value + hook |
| `YoutubeWatchOverlay.tsx` | IFrame player overlay; host heartbeat ~2.5s; remote drift 1.5s |
| `YoutubeShareDialog.tsx` | URL/ID input → `shareVideo` |
| `useYoutubePlayer.ts` | YT IFrame API player lifecycle |
| `loadYoutubeIframeApi.ts` | Script load + ready helpers |
| `parseYoutubeVideoId.ts` | URL / bare ID parse |
| `youtubeWire.ts` | Topic `youtube` packet types (legacy / parallel to stage) |

Host play/pause/seek → `updateYoutubeStage`. Non-hosts apply remote session + `remoteSyncNonce`.

---

## Presence cursors

| File | Purpose |
|------|---------|
| `presence/MeetingPresenceCursors.tsx` | Publish/subscribe normalized grid pointers; portal to `#meet-presence-layer` |
| `presence/meetingPointerWire.ts` | Topic `meeting-pointer`; encode/decode; surface norm helpers |

**Status:** `enabled = false` hard-coded in `MeetingPresenceCursors` (wire + UI ready, not active). Same pattern as viewport pan (`MeetingViewportPanProvider` also `enabled = false`).

Uses `meetingViewportTransform` + avatar colors from `chat/chatGrouping`.

---

## Recording

FE retains full context surface and component shells. **UI is not mounted** in the live controls path today:

| Piece | Status |
|-------|--------|
| `MeetingContext` recording fields | Present (`isRecording*`, `toggleRecording`, gates) |
| `toggleRecording()` | No-op body (ready for API wiring) |
| `recordingsEnabled` | Forced `false` after public-settings fetch |
| `m.$meetId` `recordingsAllowed` | Hardcoded `false` (join field ignored) |
| `RecordingButton` | Exports + `btnRecordingCn`; component returns `null` |
| `RecordingList` | Returns `null` |
| ControlsBar / MeetingHeader | Recording button/badge slots removed |
| `RoomInfoPanel` | Connection/stats dialog only — no recording list poll |

**Intended API usage** (match backend handler shapes when re-enabled):

| Method | Path | Caller |
|--------|------|--------|
| `POST` | `/api/rooms/{roomId}/recording/start` | `toggleRecording` when off |
| `POST` | `/api/rooms/{roomId}/recording/stop` | `toggleRecording` when on |
| `GET` | `/api/rooms/{roomId}/recordings` | Room info / list UI |

**Join fields:** `settings.recordingsAllowed`, `activeRecordingId` (types on join response).

**Gates (when wired):** moderator + `recordingsAllowed` (room) + `recordingsEnabled` (system/public settings).

Do not re-add skill-level “TODO oncoming feature” banners — treat scaffolding + API as the real surface; re-mount UI when wiring is restored.

---

## Top-level meeting components

| Component | File | Purpose |
|-----------|------|---------|
| `MeetingProvider` | `MeetingContext.tsx` | Room + chat contexts |
| `MeetingRoomShell` | `MeetingRoomShell.tsx` | Header/panels/sidebar/presence shell |
| `MeetingUILayoutProvider` | `MeetingUILayoutContext.tsx` | Chat/participants dock insets for overlays/controls |
| `MeetingViewportPanProvider` | `MeetingViewportPan.tsx` | Pan/zoom grid (currently disabled) |
| `ParticipantTile` | `ParticipantTile.tsx` | Video/avatar tile |
| `ParticipantGrid` | `ParticipantGrid.tsx` | Responsive multi-col grid |
| `ParticipantAvatar` | `ParticipantAvatar.tsx` | Avatar image/initials |
| `ParticipantVideoSidebar` | `ParticipantVideoSidebar.tsx` | Filmstrip when stage active |
| `SpotlightView` | `SpotlightView.tsx` | Full-screen 16:9 spotlight |
| `ScreenShareTile` | `ScreenShareTile.tsx` | Screen share video element |
| `FocusLayout` | `FocusLayout.tsx` | Pinned main + bottom strip |
| `ControlsBar` | `ControlsBar.tsx` | Cam, screenshare, leave, mic/PTT, deafen, audio menu, whiteboard/YouTube, settings |
| `MeetingControls` | `MeetingControls.tsx` | ControlsBar + creator end-for-everyone dialog (`DELETE /api/room/{id}`) |
| `MeetingHeader` | `MeetingHeader.tsx` | Status (transport/chat ready), room slug, elapsed/clock, info toggle |
| `MeetingPanels` | `MeetingPanels.tsx` | Chat/participants/info toggles + panels |
| `MeetingSoundEffects` | `MeetingSoundEffects.tsx` | Join/leave/chat/muted-mic sounds |
| `ChatPanel` | `ChatPanel.tsx` | Sidebar chat; pin/stick; image upload `POST /api/room/{id}/chat/upload` |
| `ChatToastNotifier` | `ChatToastNotifier.tsx` | Toasts when chat closed |
| `KickDetector` | `KickDetector.tsx` | Kick + room deletion system events |
| `ParticipantsList` | `ParticipantsList.tsx` | Roster panel |
| `ParticipantContextMenu` / `ParticipantMenuButton` / `ParticipantMenuContent` | `ParticipantContextMenu.tsx` | Role, kick/ban, mute/volume, stats |
| `AskActionBanner` | `AskActionBanner.tsx` | ask_unmute / ask_camera banner |
| `BeforeUnloadLock` | `BeforeUnloadLock.tsx` | Tab close guard |
| `AudioProcessorManager` | `AudioProcessorManager.tsx` | Attach/switch noise suppression on connect |
| `DeviceSelector` | `DeviceSelector.tsx` | Device dropdown helper |
| `MeetingErrorBoundary` | `MeetingErrorBoundary.tsx` | LiveKit room error boundary |
| `SecureContextBanner` | `SecureContextBanner.tsx` | Non-secure context warning |
| `LiveKitTransportFallback` | `LiveKitTransportFallback.tsx` | Prefer-relay remount on ICE failure |
| `MeetingRoomAudioRenderer` | `MeetingRoomAudioRenderer.tsx` | Remote audio + volume/mute overrides |
| `MeetingWelcomeScreen` | `MeetingWelcomeScreen.tsx` | Pre-join mic/cam/device + presence count |
| `WelcomePresenceBackdrop` | `WelcomePresenceBackdrop.tsx` | Welcome-screen ambient presence |
| `MeetLoadingScreen` | `MeetLoadingScreen.tsx` | Spinner states |
| `RoomAccessDialog` | `RoomAccessDialog.tsx` | Public/private access for managers |
| `RoomInfoPanel` | `RoomInfoPanel.tsx` | Connection quality, transport, chat channel readiness |
| `RecordingButton` | `RecordingButton.tsx` | Shell (null render) |
| `RecordingList` | `RecordingList.tsx` | Shell (null render) |
| `DeafenHeadphonesIcon` | `DeafenHeadphonesIcon.tsx` | Deafen icon |
| `useMeetingMicKeyboard` | `useMeetingMicKeyboard.ts` | Mic toggle + PTT keyboard |
| `useCameraTrackPublication` | `useCameraTrackPublication.ts` | Local camera pub helper |
| `useFocusTrap` | `useFocusTrap.ts` | Panel focus trap |
| `meetingConnectionStats` | `meetingConnectionStats.ts` | WebRTC stats helpers |
| `meetingViewportTransform` | `meetingViewportTransform.ts` | Pan/zoom math |
| `meetingParticipantProfile` | `meetingParticipantProfile.ts` | Fetch peer profile for names/avatars |
| `meeting.css` | `meeting.css` | Tile/speak/chat scroll tokens |

---

## Chat components — `chat/`

| Component / module | Purpose |
|--------------------|---------|
| `ChatInput` | Textarea, image attach, paste, poll open, emoji |
| `ChatMessageList` | Scroll, date separators, drag-drop upload |
| `ChatMessageCluster` | Telegram-style clusters; reactions + polls |
| `ChatScrollManager` | Jump-to-bottom + unread badge |
| `ChatMessageContextMenu` | Copy / info / react |
| `ChatMessageInfoModal` | Message metadata + reaction voters |
| `ChatImageLightbox` | Full-size image |
| `ChatEmojiPicker` | Wrapper over `emoji-picker/` |
| `ChatReactionList` / `ChatReactionPicker` | Quick reactions UI |
| `ChatPollComposer` | Create poll (options reorder) |
| `ChatPollBubble` | Vote UI in bubble |
| `ChatPollResultsModal` | Results breakdown |
| `chatDataChannel.ts` | Wire encode/chunk/assemble |
| `chatTopic.ts` | Topic/type detection |
| `chatGrouping.ts` | Clusters, avatars, times (**not** under `src/lib/`) |
| `chatBubbleStyles.ts` | Bubble radius/chrome |
| `chatEmojiMessage.ts` | Single-emoji message detection |
| `chatReactions.ts` | Toggle/group reactions (`QUICK_REACTIONS`) |
| `chatPollUtils.ts` | Result aggregation |
| `pollOptionReorder.ts` | Composer option reorder |
| `useChatPersistence.ts` | sessionStorage load/save |
| `emoji-picker/*` | Offline emoji groups + search |

**Chat message model:** text + image attachments + optional poll + `reactions: Record<voterIdentity, emoji>` + `status?: sending|sent|failed`.

---

## Meeting entry — `m.$meetId.tsx`

### Join sequence

1. Auth → `POST /api/room/join {roomName}`
2. Guest → name dialog → `POST /api/room/guest-join {roomName, guestName}`
3. Success → optional **Welcome screen** (interface pref `showWelcomeScreen`) → `<LiveKitRoom>`
4. Prefs one-shot: `GET /api/auth/preferences` → audio/video/experimental/interface stores

### Archived recreate

Join returns `{status:"archived_owned", name, mode, settings}` → dialog → `POST /api/room/create` → re-join.

### Reconnect

- `POST /api/room/refresh-token {roomName}` with exponential backoff
- Overlay: reconnecting (30s) → disconnected + Retry/Leave
- Never `setCurrentToken` while still connected (avoids tearing data channels)
- Transport fallback: `preferRelay` remount via `LiveKitTransportFallback`

### Layout

- No stage: `ParticipantGrid` or `FocusLayout` (pins + spotlight system events)
- Stage active: grid hidden; overlays + optional `ParticipantVideoSidebar`

---

## Related lib (meeting)

| Module | Role |
|--------|------|
| `lib/audio-processor.service.ts` | Krisp / RNNoise / browser attach-switch |
| `lib/rnnoise-processor.ts` | RNNoise TrackProcessor (WASM, code-split) |
| `lib/meeting-sounds.ts` | `playJoin/Leave/Chat/MutedBeep` |
| `lib/livekit-publish.ts` | Publish readiness, ICE/TURN helpers, `MEETING_CHAT_TOPIC`, diagnostics |
| `lib/livekit-transport-type.ts` | P2P vs relay detection |
| `lib/meeting-device-storage.ts` | Persist selected mic/cam IDs |
| `lib/push-to-talk-key.ts` | PTT key normalize/format |
| `lib/push-to-talk-participant.ts` | Metadata PTT flags + mute indicators |
| `lib/participant-overrides.store.ts` | Local mute/volume per identity |
| `lib/audio-preferences.store.ts` | Noise mode, EC, AGC, PTT |
| `lib/video-preferences.store.ts` | Video prefs |
| `lib/experimental-preferences.store.ts` | Whiteboard/YouTube flags + disclaimer |
| `lib/interface-preferences.store.ts` | Welcome screen, etc. |
| `lib/usePinnedParticipants.ts` | Pin set for focus layout |

---

## Component dependency graph

```
m.$meetId
└── LiveKitRoom
    └── MeetingStageProvider
        └── MeetingProvider
            └── YoutubeWatchProvider + WhiteboardWatchProvider
                └── MeetingRoomShell
                    ├── MeetingPresenceCursors
                    ├── MeetingLayout → ParticipantGrid | FocusLayout → ParticipantTile
                    ├── YoutubeWatchOverlay | WhiteboardOverlay | StageScreenShareOverlay
                    ├── ParticipantVideoSidebar
                    ├── MeetingHeader → RoomInfoPanel (via panels)
                    └── MeetingPanels
                        ├── MeetingControls → ControlsBar
                        │     ├── DeviceSelector, PTT, deafen
                        │     ├── claimStage(screenshare|…)
                        │     └── whiteboard / YouTube menu
                        ├── ChatPanel
                        │     ├── ChatInput (+ poll composer, emoji)
                        │     └── ChatMessageList → ChatMessageCluster
                        │           ├── ChatPollBubble, ChatReactionList
                        │           └── context menu / lightbox
                        ├── ParticipantsList → ParticipantContextMenu
                        └── ChatToastNotifier

WhiteboardOverlay
└── MeetingSharedWhiteboard ← Y.Doc ← LiveKitYjsProvider
    + pointer / follow / element locks hooks
```

---

## Styles

`components/meeting/meeting.css` + CSS vars (`--meet-*`):
- `.meet-tile`, `.meet-tile.meet-speaking`, speak-bar / connecting-spin keyframes
- `.meet-chat-scroll`, `.meet-room`, `.meet-dialog`, prejoin panels
- Dock helpers: `meetRightInsetClass`, `meetControlsDockClass` from layout context

Prefer tokens over hardcoded hex; inline `style` only for computed transforms, avatar palette, dynamic glow.

---

## Agent notes

1. **Stage is client wire** — do not invent HTTP stage APIs; use `MeetingStageProvider` / `stageWire`.
2. **Whiteboard runtime lives in meeting/** — vendor Excalidraw is dependency only; Yjs + LiveKit provider are Bedrud-owned.
3. **Experimental gates** — whiteboard/YouTube require settings flags before ControlsBar shows actions.
4. **Chat is richer than text** — polls, reactions, chunking, image upload, sessionStorage persistence.
5. **Recording** — document context/API shells; UI currently null/not mounted; no skill “TODO oncoming” banners.
6. **Presence / viewport pan** — implemented, hard-disabled (`enabled = false`).
7. **Path aliases** — `#/*` → `src/*`, `@/*` → `src/*` (meeting often uses `@/components/meeting/...`).
)
