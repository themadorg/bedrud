import type { JSX } from 'react'

type DocumentMeta = JSX.IntrinsicElements['meta'] & { title?: string }
type DocumentLink = JSX.IntrinsicElements['link']

export const APP_NAME = 'Bedrud'

/**
 * The light `--background` from theme.css, for the manifest's own `theme_color`. Nothing here
 * renders a theme-color meta: the system bar colour comes from the CSS token, written by the
 * pre-paint script in the root route and kept current by `applyTheme`. This constant exists only
 * so the two places that cannot read CSS stay pinned to each other — `document-head.test.ts` pins
 * it to theme.css and `pwa-manifest.test.ts` pins manifest.json to it.
 */
export const LIGHT_THEME_COLOR = '#fafafa'

export const documentMeta: DocumentMeta[] = [
  { charSet: 'utf-8' },
  { name: 'viewport', content: 'width=device-width, initial-scale=1, viewport-fit=cover' },
  { title: APP_NAME },
  { name: 'mobile-web-app-capable', content: 'yes' },
  { name: 'apple-mobile-web-app-title', content: APP_NAME },
  // `black-translucent` would force white status text over the light theme.
  { name: 'apple-mobile-web-app-status-bar-style', content: 'default' },
]

/** Head links; the stylesheet URL comes from Vite's `?url` import in the root route. */
export function documentLinks(stylesheetHref: string): DocumentLink[] {
  return [
    { rel: 'stylesheet', href: stylesheetHref },
    { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
    { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
    { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' },
    { rel: 'manifest', href: '/manifest.json' },
  ]
}
