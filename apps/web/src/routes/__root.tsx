import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRootRoute, HeadContent, Scripts } from '@tanstack/react-router'
import { useEffect } from 'react'
import { IntlProvider } from 'react-intl'
import { Toaster } from 'sonner'
import { useAuthStore } from '#/lib/auth.store'
import { applyTheme, useThemeStore } from '#/lib/theme.store'
import { installVisualViewportCssVars } from '#/lib/visual-viewport'
import enMessages from '#/locales/en.json'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import appCss from '../styles.css?url'

// Inline script that runs before first paint to avoid theme flash.
// Reads the persisted Zustand value from localStorage directly.
const themeScript = `
(function(){
  try {
    var stored = JSON.parse(localStorage.getItem('theme') || '{}');
    var theme = stored.state?.theme || 'system';
    var dark = theme === 'dark' ||
      (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    if (dark) document.documentElement.classList.add('dark');
  } catch(e) {}
})();
`

// iOS Safari: layout 100vh is taller than the visible area when the toolbar is
// shown. Publish visualViewport height/offset before paint so fixed modals fit.
const viewportScript = `
(function(){
  function u(){
    try {
      var vv = window.visualViewport;
      var h = vv ? vv.height : window.innerHeight;
      var w = vv ? vv.width : window.innerWidth;
      var t = vv ? vv.offsetTop : 0;
      var l = vv ? vv.offsetLeft : 0;
      var r = document.documentElement;
      r.style.setProperty('--app-height', h + 'px');
      r.style.setProperty('--app-width', w + 'px');
      r.style.setProperty('--app-offset-top', t + 'px');
      r.style.setProperty('--app-offset-left', l + 'px');
    } catch(e) {}
  }
  u();
  if (window.visualViewport) {
    visualViewport.addEventListener('resize', u);
    visualViewport.addEventListener('scroll', u);
  }
  window.addEventListener('resize', u);
  window.addEventListener('orientationchange', u);
})();
`

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30_000 },
  },
})

export const Route = createRootRoute({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1, viewport-fit=cover' },
      { title: 'Bedrud' },
    ],
    links: [
      { rel: 'stylesheet', href: appCss },
      { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
      { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
      { rel: 'manifest', href: '/manifest.json' },
    ],
    scripts: [{ children: themeScript }, { children: viewportScript }],
  }),
  shellComponent: RootDocument,
})

function RootDocument({ children }: { children: React.ReactNode }) {
  const theme = useThemeStore((s) => s.theme)

  // Re-sync whenever the stored theme changes (e.g. on another tab).
  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  // Also re-sync when the OS preference changes while theme is 'system'.
  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => applyTheme(useThemeStore.getState().theme)
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [])

  // Keep --app-height / --app-offset-top in sync (iOS Safari toolbar).
  useEffect(() => installVisualViewportCssVars(), [])

  // Restore session via HTTP-only cookie refresh.
  // Runs in the background — does NOT block initial render.
  // Protected routes await the result in their beforeLoad guards.
  // Errors are logged so network failures don't silently swallow auth init.
  useEffect(() => {
    useAuthStore
      .getState()
      .initialize()
      .catch((err) => {
        console.error('Auth initialization failed:', err)
      })
  }, [])

  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <HeadContent />
      </head>
      <body className="font-sans antialiased">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[100] focus:bg-background focus:px-4 focus:py-2 focus:text-sm focus:font-medium focus:shadow-lg focus:outline-none focus:ring-2 focus:ring-primary"
        >
          Skip to main content
        </a>
        <ErrorBoundary variant="server">
          <IntlProvider locale="en" messages={enMessages}>
            <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
          </IntlProvider>
          <Toaster richColors closeButton />
        </ErrorBoundary>
        <Scripts />
      </body>
    </html>
  )
}
