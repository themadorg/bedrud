# Vendored Excalidraw (0.18.0)

Editable in-tree copy of the Excalidraw monorepo packages used by the meeting whiteboard.

- Source of truth: `apps/web/src/vendor/excalidraw/packages/`
- `apps/web/context/excalidraw/` is a read-only upstream reference (gitignored)

Packages:

| Package | Path |
|---------|------|
| `@excalidraw/excalidraw` | `packages/excalidraw/` |
| `@excalidraw/common` | `packages/common/src/` |
| `@excalidraw/element` | `packages/element/src/` |
| `@excalidraw/math` | `packages/math/src/` |
| `@excalidraw/utils` | `packages/utils/src/` |
| `@excalidraw/fractional-indexing` | `packages/fractional-indexing/src/` |
| `@excalidraw/laser-pointer` | `packages/laser-pointer/src/` |

Vite resolves `@excalidraw/*` imports to these paths (see `vite.config.ts`).