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

// Remote debug (beddev.* via Traefik): HMR remounts meeting components and kills LiveKit PC.
const isRemoteDebug = extraAllowedHosts.length > 0

const config = defineConfig({
  resolve: {
    tsconfigPaths: true,
    dedupe: ['react', 'react-dom'],
    alias: [
      { find: /^@\/(.*)$/, replacement: path.join(appRoot, 'src/$1') },
      { find: /^#\/(.*)$/, replacement: path.join(appRoot, 'src/$1') },
      ...excalidrawAliases(appRoot),
    ],
  },
  optimizeDeps: {
    include: [
      '@livekit/components-react',
      '@livekit/krisp-noise-filter',
      'livekit-client',
      'roughjs',
      'jotai',
      'perfect-freehand',
      'yjs',
      'y-protocols/sync',
      'lib0/encoding',
      'lib0/decoding',
    ],
  },
  plugins: [
    devtools({ eventBusConfig: { port: DEV_PORT_DEVTOOLS } }),
    tailwindcss(),
    tanstackStart(),
    viteReact(),
  ],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: [],
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
  server: {
    port: DEV_PORT_WEB,
    // WireGuard mode: Traefik reaches Vite via tunnel IP (10.0.0.2).
    host: process.env.BEDRUD_DEV_BIND_HOST || undefined,
    // Remote debug (devcli remote run) — Traefik forwards public host to local Vite.
    allowedHosts: ['localhost', '127.0.0.1', ...extraAllowedHosts],
    // Prevent Vite HMR from tearing down WebRTC while testing on the public URL.
    hmr: isRemoteDebug ? false : undefined,
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
          if (id.includes('/components/meeting/MeetingContext')) {
            return 'meeting-context'
          }
          if (id.includes('/node_modules/react/') || id.includes('/node_modules/react-dom/')) {
            return 'react-vendor'
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
            !id.includes('/node_modules/@jitsi/rnnoise-wasm/')
          ) {
            return 'vendor'
          }
        },
      },
    },
  },
})

export default config
