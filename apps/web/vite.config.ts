import tailwindcss from '@tailwindcss/vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

const config = defineConfig({
  resolve: {
    tsconfigPaths: true,
  },
  plugins: [devtools(), tailwindcss(), tanstackStart(), viteReact()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: [],
  },
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8090',
      '/livekit': {
        target: 'http://localhost:8090',
        ws: true,
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 6000,
    rollupOptions: {
      output: {
        manualChunks(id: string) {
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
          if (id.includes('/node_modules/') &&
            !id.includes('/node_modules/@livekit/krisp-noise-filter/') &&
            !id.includes('/node_modules/@jitsi/rnnoise-wasm/')) {
            return 'vendor'
          }
        },
      },
    },
  },
})

export default config
