# Meeting Whiteboard — Multiplayer Collaboration

Shared Excalidraw canvas for all meeting participants. Scene state syncs over **Yjs** (CRDT); ephemeral presence (cursors, selection, follow/viewport) uses separate LiveKit data-channel topics.

Reference: `apps/web/context/y-excalidraw/` (patterns only — **do not import**). Production code lives in `apps/web/src/components/meeting/whiteboard/`.

See also: [Whiteboard Yjs sync](./whiteboard-yjs.md) for transport and binding details.

---

## Architecture

```
LiveKitRoom
  └─ WhiteboardWatchProvider          # Y.Doc + LiveKitYjsProvider per session
       └─ WhiteboardOverlay
            ├─ useWhiteboardElementLocks   # Y.Map locks — edit ownership
            ├─ useWhiteboardPointerSync    # cursors + remote selection
            ├─ useWhiteboardFollowSync     # follow + viewport relay
            └─ MeetingSharedWhiteboard
                 └─ bindExcalidrawToYDoc   # scene CRDT + settings
```

### LiveKit topics

| Topic | Reliable | Payload |
|-------|----------|---------|
| `whiteboard-yjs` | yes | Yjs sync (binary, chunked) |
| `whiteboard-pointer` | no | Cursor, button, selection, color |
| `whiteboard-follow` | mixed | Follow/unfollow + viewport bounds |

---

## Y.Doc schema

```typescript
Y.Doc
├── elements: Y.Map<id, OrderedExcalidrawElement>
├── order:    Y.Array<id>
├── files:    Y.Map<id, BinaryFileData>
├── settings: Y.Map  // viewBackgroundColor, gridModeEnabled
└── locks:    Y.Map<id, { identity, username, ts }>
```

`createWhiteboardYDoc()` in `livekitYjsProvider.ts` initializes all maps.

---

## Shared canvas (Yjs)

- **Local → remote:** `excalidrawYjsBinding` debounces scene writes (60 ms), batches provider flush (120 ms), immediate flush on pointer-up.
- **Remote → local:** Observes `elements`, `order`, `files`, `settings`, `locks`. Merges with lock-aware policy (below).
- **Background:** `settings.viewBackgroundColor` and `settings.gridModeEnabled` sync for all participants via `whiteboardSyncSettings.ts`.

---

## Element locks (edit ownership)

While you touch, draw, select, or edit text on a shape, you **hold the lock** for that element id in `locks`.

| Event | Lock behavior |
|-------|----------------|
| Select element(s) | Acquire lock (claims from previous owner) |
| Create new shape / stroke | Acquire lock on new id |
| Text wysiwyg open | Hold lock on `editingTextElement` |
| Deselect / pointer-up after draw | Release lock if no longer held |
| Participant disconnect | `releaseAllLocksForIdentity` |

**Merge rules** (`whiteboardElementLocks.ts`):

- Locked by you → your local copy wins on merge.
- Locked by someone else → their Yjs copy wins; your local edits to that id are stripped before push.
- Unlocked → version / stroke-length merge (`pickNewerElement`).

Files: `whiteboardElementLocks.ts`, `useWhiteboardElementLocks.ts`.

---

## Remote cursors & selection

`useWhiteboardPointerSync` publishes ~30 fps on `whiteboard-pointer`:

- Pointer position + tool (pointer / laser)
- `selectedElementIds` (remote selection outlines)
- `color` from `avatarColor(identity)`

Received packets update Excalidraw `collaborators` (`isCollaborating` enabled on canvas).

Inspired by y-excalidraw Awareness `pointer` + `selectedElementIds`, adapted for LiveKit (no separate Awareness server).

---

## Follow & viewport

`useWhiteboardFollowSync` on `whiteboard-follow`:

1. **Follow change** — when user follows/unfollows in Excalidraw UI, relay to target’s `followedBy` set.
2. **Viewport** — when someone follows you, you broadcast visible `sceneBounds` (~30 fps). Follower runs `zoomToFitBounds`.

Disconnect cleanup removes stale `followedBy` / `userToFollow` entries.

---

## File map

| File | Role |
|------|------|
| `WhiteboardWatchContext.tsx` | Session, `ydoc`, provider lifecycle |
| `livekitYjsProvider.ts` | Yjs ↔ LiveKit |
| `excalidrawYjsBinding.ts` | Excalidraw ↔ Y.Doc + lock-aware merge |
| `whiteboardElementLocks.ts` | Lock map CRUD + merge helpers |
| `useWhiteboardElementLocks.ts` | Acquire/release hook |
| `useWhiteboardPointerSync.ts` | Cursors + selection |
| `useWhiteboardFollowSync.ts` | Follow + viewport |
| `whiteboardSyncSettings.ts` | Shared background/grid keys |
| `MeetingSharedWhiteboard.tsx` | Excalidraw shell |
| `WhiteboardOverlay.tsx` | Wires all hooks |

---

## Testing

```bash
cd apps/web
bun run test src/components/meeting/whiteboard/whiteboardElementLocks.test.ts
bun run test src/components/meeting/whiteboard/yjsWire.test.ts
bun run test src/components/meeting/whiteboard/whiteboardPointerWire.test.ts
bun run test src/components/meeting/whiteboard/whiteboardFollowWire.test.ts
```

---

## Debugging

| Symptom | Check |
|---------|--------|
| Remote edits overwrite in-progress stroke | Lock held? `locks` map in Y.Doc |
| Cursor missing | Experimental whiteboard on + `whiteboard-pointer` topic |
| Background not shared | `settings` map; menu uses `pickSyncableSettings` |
| Follow viewport jumpy | Expected — bounds fit, not pixel-perfect scroll sync |
| Stale lock after crash | Locks cleared on `ParticipantDisconnected` |