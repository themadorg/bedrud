# Static assets

Generated files in this directory are never edited by hand.

| File | Source | Regenerate |
|---|---|---|
| `icon-maskable-512.png` | `../scripts/app-icon.svg` | `bun run pwa-icons` |
| `apple-touch-icon.png` | `../scripts/app-icon.svg` | `bun run pwa-icons` |

`manifest.json`, `document-head.ts` and `theme.css` must agree on the light surface colour;
`src/pwa-manifest.test.ts` and `src/lib/document-head.test.ts` fail when they drift.
