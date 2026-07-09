# Vendored Excalidraw (0.18.0 hybrid)

Editable in-tree copy of the Excalidraw monorepo packages used by the meeting whiteboard.

- Source of truth: `apps/web/src/vendor/excalidraw/packages/`
- `apps/web/context/excalidraw/` is a read-only upstream reference (gitignored via `/context/` in `apps/web/.gitignore`)

## Packages

| Package | Path |
|---------|------|
| `@excalidraw/excalidraw` | `packages/excalidraw/` |
| `@excalidraw/common` | `packages/common/src/` |
| `@excalidraw/element` | `packages/element/src/` |
| `@excalidraw/math` | `packages/math/src/` |
| `@excalidraw/utils` | `packages/utils/src/` |
| `@excalidraw/fractional-indexing` | `packages/fractional-indexing/src/` |
| `@excalidraw/laser-pointer` | `packages/laser-pointer/src/` |

Vite resolves `@excalidraw/*` imports to these paths (see `aliases.ts` + `vite.config.ts`).

## Compatibility shims

The tree is a **hybrid**: split packages (`common`, `element`, …) plus some files that still use legacy relative imports (`../constants`, `../element/*`, …).

To keep Vite resolution working:

| Shim | Purpose |
|------|---------|
| `packages/excalidraw/constants.ts` (and `utils`, `utility-types`, `points`, `random`, `queue`, `emitter`) | Re-export from `packages/common/src/` |
| `packages/excalidraw/fractionalIndex.ts`, `frame.ts` | Re-export from `packages/element/src/` |
| `packages/excalidraw/element` → `../element/src` | Symlink so `../element/*` relative imports resolve |

## Restored modules (required for production build)

These were missing from the initial vendor (or blocked by over-broad gitignore):

**Context** (`packages/excalidraw/context/`):

- `tunnels.ts` — portal tunnels for MainMenu / Footer / sidebars
- `ui-appState.ts` — `UIAppStateContext` / `useUIAppState`

**Data** (`packages/excalidraw/data/`):

- `image.ts`, `encode.ts`, `encryption.ts`, `filesystem.ts`
- `EditorLocalStorage.ts`, `resave.ts`, `url.ts`
- Core modules (`blob`, `json`, `restore`, `library`, …) aligned with upstream package-import style where needed

## Gitignore notes

- `apps/web/.gitignore`: use `/context/` (root only), not bare `context/`, or vendor context is ignored.
- Repo root `.gitignore`: use `/data` (root only), not bare `data`, or nested datasets like `emoji-picker/data/` are ignored.
