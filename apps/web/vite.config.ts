import path from 'node:path'
import { fileURLToPath } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'
import { excalidrawAliases } from './src/vendor/excalidraw/aliases'

const appRoot = path.dirname(fileURLToPath(import.meta.url))

const DEV_PORT_WEB = 7070
const DEV_PORT_API = 7071
// Local make dev only — embedded LiveKit on :7072. Remote debug uses server /livekit (not this proxy).
const DEV_PORT_LIVEKIT = 7072
const DEV_PORT_DEVTOOLS = 7074

const extraAllowedHosts = (process.env.BEDRUD_ALLOWED_HOSTS ?? '')
  .split(',')
  .map((host) => host.trim())
  .filter(Boolean)

// Remote debug (public host via Traefik): HMR remounts meeting components and kills LiveKit PC.
const isRemoteDebug = extraAllowedHosts.length > 0

/**
 * Deps pulled in when the whiteboard (vendored Excalidraw) first loads.
 * Must be pre-optimized at server start — otherwise Vite re-optimizes mid-session,
 * full-reloads the page, and can leave dual React/Yjs instances (invalid hook call /
 * "Yjs was already imported").
 */
const EXCALIDRAW_RUNTIME_DEPS = [
  'canvas-roundrect-polyfill',
  'browser-fs-access',
  'roughjs',
  'perfect-freehand',
  'points-on-curve',
  'pako',
  'pica',
  'image-blob-reduce',
  'nanoid',
  'clsx',
  'lodash.debounce',
  'lodash.throttle',
  'jotai',
  'jotai-scope',
  'tunnel-rat',
  'fuzzy',
  'es6-promise-pool',
  '@braintree/sanitize-url',
  'pwacompat',
  'radix-ui',
  '@codemirror/commands',
  '@codemirror/language',
  '@codemirror/state',
  '@codemirror/view',
  '@lezer/highlight',
]

const config = defineConfig({
  resolve: {
    tsconfigPaths: true,
    // Single shared copies for anything Excalidraw + whiteboard collab touch.
    // Do NOT alias react → CJS index.js (breaks Vite SSR: "module is not defined").
    dedupe: [
      'react',
      'react-dom',
      'react/jsx-runtime',
      'react/jsx-dev-runtime',
      'yjs',
      'lib0',
      'y-protocols',
      'jotai',
      'jotai-scope',
      'livekit-client',
      '@livekit/components-react',
    ],
    alias: [
      { find: /^@\/(.*)$/, replacement: path.join(appRoot, 'src/$1') },
      { find: /^#\/(.*)$/, replacement: path.join(appRoot, 'src/$1') },
      ...excalidrawAliases(appRoot),
    ],
  },
  optimizeDeps: {
    // Crawl whiteboard entry at startup so first open doesn't discover new deps.
    entries: [
      'src/client.tsx',
      'src/routes/**/*.{ts,tsx}',
      'src/components/meeting/whiteboard/excalidrawLazy.tsx',
      'src/components/meeting/whiteboard/MeetingSharedWhiteboard.tsx',
    ],
    include: [
      'react',
      'react-dom',
      'react/jsx-runtime',
      'react/jsx-dev-runtime',
      '@livekit/components-react',
      '@livekit/krisp-noise-filter',
      'livekit-client',
      'yjs',
      'y-protocols/sync',
      'y-protocols/awareness',
      'lib0',
      'lib0/encoding',
      'lib0/decoding',
      ...EXCALIDRAW_RUNTIME_DEPS,
    ],
    // Keep vendored package *names* out of prebundle as npm packages — they resolve
    // to apps/web/src/vendor/excalidraw source via aliases (project patches).
    exclude: [
      '@excalidraw/excalidraw',
      '@excalidraw/element',
      '@excalidraw/common',
      '@excalidraw/math',
      '@excalidraw/utils',
      '@excalidraw/fractional-indexing',
      '@excalidraw/laser-pointer',
    ],
  },
  // TanStack Start SSR runner must share the same optimized graph.
  ssr: {
    optimizeDeps: {
      include: ['react', 'react-dom', 'react/jsx-runtime', 'yjs', 'jotai', 'jotai-scope', 'lib0'],
    },
    // Don't externalize these so SSR doesn't load a second copy vs client.
    noExternal: ['yjs', 'lib0', 'y-protocols', 'jotai', 'jotai-scope'],
  },
  plugins: [
    devtools({ eventBusConfig: { port: DEV_PORT_DEVTOOLS } }),
    tailwindcss(),
    // Custom client entry without React.StrictMode — required for stable LiveKit/WebRTC.
    tanstackStart({
      client: {
        entry: path.join(appRoot, 'src/client.tsx'),
      },
    }),
    viteReact(),
  ],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: [],
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
    exclude: ['**/node_modules/**', '**/vendor/excalidraw/**'],
  },
  server: {
    port: DEV_PORT_WEB,
    // WireGuard mode: Traefik reaches Vite via tunnel IP (10.0.0.2).
    host: process.env.BEDRUD_DEV_BIND_HOST || undefined,
    // Remote debug (devcli remote run) — Traefik forwards public host to local Vite.
    allowedHosts: ['localhost', '127.0.0.1', ...extraAllowedHosts],
    // Prevent Vite HMR from tearing down WebRTC while testing on the public URL.
    hmr: isRemoteDebug ? false : undefined,
    // Warm whiteboard entry so deps are discovered before the user opens it.
    warmup: {
      clientFiles: [
        './src/components/meeting/whiteboard/excalidrawLazy.tsx',
        './src/components/meeting/whiteboard/MeetingSharedWhiteboard.tsx',
      ],
    },
    proxy: {
      '/api': `http://localhost:${DEV_PORT_API}`,
      '/uploads': `http://localhost:${DEV_PORT_API}`,
      // Proxy signaling directly to LiveKit (strip /livekit prefix).
      // Do NOT chain through the Go API — double WS proxy breaks /rtc/v1/validate.
      '/livekit': {
        target: `http://127.0.0.1:${DEV_PORT_LIVEKIT}`,
        changeOrigin: true,
        ws: true,
        rewrite: (path) => path.replace(/^\/livekit/, ''),
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 6000,
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          // Never force React/Yjs into isolated chunks — dual copies break hooks & Y.Doc.
          if (
            id.includes('/node_modules/react/') ||
            id.includes('/node_modules/react-dom/') ||
            id.includes('/node_modules/yjs/') ||
            id.includes('/node_modules/lib0/') ||
            id.includes('/node_modules/jotai/')
          ) {
            return
          }
          if (id.includes('/components/meeting/MeetingContext')) {
            return 'meeting-context'
          }
          if (id.includes('/node_modules/@tanstack/')) {
            return 'tanstack-vendor'
          }
          if (id.includes('/node_modules/livekit-client/')) {
            return 'livekit-client-vendor'
          }
          if (id.includes('/node_modules/@livekit/components-react/')) {
            return 'livekit-components-vendor'
          }
          if (id.includes('/node_modules/recharts') || id.includes('/node_modules/d3-')) {
            return 'charts-vendor'
          }
          if (id.includes('/node_modules/@radix-ui/')) {
            return 'ui-vendor'
          }
          if (
            id.includes('/node_modules/react-markdown') ||
            id.includes('/node_modules/remark') ||
            id.includes('/node_modules/unified') ||
            id.includes('/node_modules/rehype') ||
            id.includes('/node_modules/hast') ||
            id.includes('/node_modules/mdast') ||
            id.includes('/node_modules/micromark') ||
            id.includes('/node_modules/vfile')
          ) {
            return 'markdown-vendor'
          }
          if (id.includes('/src/vendor/excalidraw/')) {
            return 'excalidraw-vendor'
          }
          if (
            id.includes('/node_modules/') &&
            !id.includes('/node_modules/@livekit/krisp-noise-filter/') &&
            !id.includes('/node_modules/@jitsi/rnnoise-wasm/') &&
            !id.includes('/node_modules/@excalidraw/mermaid-to-excalidraw/') &&
            !id.includes('/node_modules/@excalidraw/markdown-to-text/') &&
            !id.includes('/node_modules/mermaid/') &&
            !id.includes('/node_modules/@mermaid-js/')
          ) {
            return 'vendor'
          }
        },
      },
    },
  },
})

export default config
