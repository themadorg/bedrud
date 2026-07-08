# Meeting Whiteboard — Yjs Sync Architecture

The Bedrud meeting whiteboard uses **Excalidraw** for drawing and **Yjs** for real-time scene sync. Transport is LiveKit's reliable data channel (not a separate Yjs WebSocket server).

---

## Why Yjs (not full-scene JSON broadcast)

The legacy `whiteboardWire` path sent the entire Excalidraw scene as JSON on every change. That caused:

- **Flicker** — echoed partial updates overwrote the local canvas via `remoteScene` React state
- **Bandwidth spikes** — every stroke re-sent all elements + files
- **Race conditions** — debounced local vs remote timestamps fought over the same scene

Yjs stores incremental updates in a shared `Y.Doc`. Each client applies small binary diffs; the Excalidraw binding merges intelligently so local in-progress strokes are not regressed by stale echoes.

---

## High-level flow

```
┌─────────────────────────────────────────────────────────────────┐
│  Meeting route (m.$meetId.tsx)                                  │
│  LiveKitRoom → MeetingStageProvider → WhiteboardWatchProvider   │
└────────────────────────────┬────────────────────────────────────┘
                             │
         ┌───────────────────┼───────────────────┐
         ▼                   ▼                   ▼
  MeetingStageContext   WhiteboardWatchContext   WhiteboardOverlay
  (topic: stage)        (Y.Doc + provider)       (MeetingSharedWhiteboard)
         │                   │                   │
         │ claimStage        │ ydoc                │ bindExcalidrawToYDoc
         │ ('whiteboard')    ▼                   ▼
         │            LiveKitYjsProvider ←→ excalidrawYjsBinding
         │            (topic: whiteboard-yjs)      (Excalidraw API)
         └──────────────────────────────────────────────────────────
```

| Layer | Topic / transport | Responsibility |
|-------|-------------------|----------------|
| Stage | `stage` | Who owns the meeting stage (whiteboard / YouTube / screenshare) |
| Scene sync | `whiteboard-yjs` | Excalidraw elements, order, embedded files via Yjs |

Stage and scene are **separate**: opening the whiteboard claims the stage; drawing syncs over Yjs.

---

## File map (`apps/web`)

| File | Role |
|------|------|
| `src/components/meeting/whiteboard/WhiteboardWatchContext.tsx` | React context: session, `ydoc`, start/stop, `flushWhiteboardSync` |
| `src/components/meeting/whiteboard/WhiteboardOverlay.tsx` | Meeting UI shell around the canvas |
| `src/components/meeting/whiteboard/MeetingSharedWhiteboard.tsx` | Lazy Excalidraw + Yjs binding + keyboard helpers |
| `src/components/meeting/whiteboard/excalidrawYjsBinding.ts` | Excalidraw ↔ `Y.Doc` (elements map, order array, files map) |
| `src/components/meeting/whiteboard/livekitYjsProvider.ts` | Yjs sync protocol over LiveKit `publishData` |
| `src/components/meeting/whiteboard/yjsWire.ts` | Binary packet framing + chunking for large updates |
| `src/components/meeting/whiteboard/excalidrawSceneUtils.ts` | Scene normalization, element signatures |
| `src/components/meeting/whiteboard/whiteboardKeyboard.ts` | Escape-to-deselect in embedded meeting UI |
| `src/components/meeting/whiteboard/whiteboardWire.ts` | **Legacy** JSON scene broadcast (unused by meeting UI) |

---

## Y.Doc schema

```typescript
Y.Doc
├── elements: Y.Map<string, OrderedExcalidrawElement>  // id → element
├── order:    Y.Array<string>                         // z-order element ids
├── files:    Y.Map<string, BinaryFileData>           // embedded images
├── settings: Y.Map                                   // viewBackgroundColor, gridModeEnabled
└── locks:    Y.Map<id, { identity, username, ts }>   // collaborative edit ownership
```

Multiplayer cursors, follow/viewport, and lock semantics: [whiteboard-multiplayer.md](./whiteboard-multiplayer.md).

Created by `createWhiteboardYDoc()` in `livekitYjsProvider.ts`.

---

## Excalidraw ↔ Yjs binding

`bindExcalidrawToYDoc(api, doc)` in `excalidrawYjsBinding.ts`:

### Local → Yjs

1. `onExcalidrawChange` debounces writes (60 ms).
2. `flush()` on pointer-up pushes the final stroke immediately.
3. Writes use transaction origin `EXCALIDRAW_YJS_ORIGIN` so they do not echo back into Excalidraw.

### Yjs → Excalidraw

1. Observes `elements`, `order`, `files`.
2. Skips updates where `transaction.origin === EXCALIDRAW_YJS_ORIGIN`.
3. **Merge policy** — for each element id, keeps the copy with higher `version` or more draw points so LiveKit echoes cannot shorten an in-progress stroke.

### Signatures

`sceneElementsSignature()` includes element version and last point coordinates for freehand elements, so point-only changes during a stroke are detected.

---

## LiveKit Yjs provider

`LiveKitYjsProvider` implements the [y-protocols sync](https://github.com/yjs/y-protocols) handshake over LiveKit:

| Setting | Value |
|---------|-------|
| Data topic | `whiteboard-yjs` |
| Channel | reliable `publishData` |
| Outbound debounce | 120 ms (merged via `Y.mergeUpdates`) |
| Pointer-up flush | `provider.flush()` + `binding.flush()` |
| Publish queue | sequential, 8 ms gap between packets |
| Large updates | binary chunking in `yjsWire.ts` (56 KB safe payload) |

### Origins

| Symbol | Meaning |
|--------|---------|
| `LIVEKIT_YJS_ORIGIN` | Update applied from network — may be sent back as sync reply |
| `EXCALIDRAW_YJS_ORIGIN` | Local Excalidraw write — never re-applied to canvas |

### Sync on join

On provider construction and at 800 ms / 2 s / 4 s, `requestSync()` sends sync step 1 so late joiners receive the document state.

---

## React lifecycle (anti-flicker)

`WhiteboardWatchProvider` creates **one** `Y.Doc` per whiteboard session:

- **Create** when `stage.kind === 'whiteboard'` and no provider exists yet
- **Destroy** when stage leaves whiteboard (`clearStage` / host disconnect)
- **Do not** recreate on `stage.updatedAt` republishes — that was a major flicker source

Late joiners rely on Yjs sync, not a fresh empty document.

---

## Provider tree (meeting room)

```
LiveKitRoom
└── MeetingProvider
    └── MeetingStageProvider
        └── YoutubeWatchProvider
            └── WhiteboardWatchProvider   ← ydoc lives here
                ├── MeetingRoomShell
                │   ├── WhiteboardOverlay → MeetingSharedWhiteboard(ydoc)
                │   └── MeetingPanels → ControlsBar (start/stop whiteboard)
                └── …
```

`useWhiteboardWatch()` must be called inside `WhiteboardWatchProvider` to avoid duplicate React context instances.

---

## Controls

| Action | Where |
|--------|-------|
| Open whiteboard | ControlsBar → `startWhiteboard()` → `claimStage('whiteboard')` |
| Close whiteboard | Host close button → `stopWhiteboard()` → `clearStage()` |
| Draw / edit | Excalidraw → binding → Y.Doc → provider → LiveKit |
| Escape deselect | `whiteboardKeyboard.ts` (capture-phase listener) |

---

## Dependencies

**In-tree (editable):** Excalidraw monorepo packages live under `apps/web/src/vendor/excalidraw/packages/` — not the npm `@excalidraw/excalidraw` package. Vite resolves `@excalidraw/*` to this source via `src/vendor/excalidraw/aliases.ts`.

**npm (sync + optional features):**

```json
"yjs": "^13.6.31",
"y-protocols": "^1.0.7",
"@excalidraw/mermaid-to-excalidraw": "2.2.2"
```

Vite `optimizeDeps.include`: `yjs`, `y-protocols/sync`, `lib0/encoding`, `lib0/decoding`, `roughjs`, `jotai`, `perfect-freehand`.

`apps/web/context/excalidraw/` is a read-only upstream reference (gitignored). Copy changes from there into `src/vendor/excalidraw/` when upgrading.

---

## Tests

```bash
cd apps/web
bun run test src/components/meeting/whiteboard/yjsWire.test.ts
bun run test src/components/meeting/whiteboard/whiteboardKeyboard.test.ts
bun run typecheck
```

---

## Debugging

| Symptom | Likely cause | Check |
|---------|--------------|-------|
| Canvas flickers | Full-scene JSON path or Y.Doc recreated on `updatedAt` | Confirm `MeetingSharedWhiteboard` uses `ydoc`, not `remoteScene` |
| Partial stroke on release | Missing pointer-up flush | `binding.flush()` + `flushWhiteboardSync()` in `onPointerUp` |
| `useWhiteboardWatch` error | Hook outside provider or duplicate context module | Import provider/hook from `@/components/meeting/whiteboard/WhiteboardWatchContext` |
| `[LiveKitYjsProvider] failed to publish` | Data channel flood or size limit | Binary `yjsWire` chunking; sequential publish queue |
| Remote sees empty board | Sync step not completed | Provider `requestSync` timers; reliable topic `whiteboard-yjs` |

Enable dev logging: errors are logged in DEV for publish failures and malformed packets.

---

## Legacy `whiteboardWire`

`whiteboardWire.ts` (topic `whiteboard`, JSON payloads) remains in the repo for reference but is **not wired** to the meeting UI after the Yjs migration. Do not mix both transports in the same session.

---

## Related docs

- [Web development](./web-development.md) — `apps/web` toolchain
- [Room lifecycle](../server/room-lifecycle.md) — meeting rooms + LiveKit
- [Excalidraw + Yjs](https://docs.yjs.dev/) — upstream CRDT docs