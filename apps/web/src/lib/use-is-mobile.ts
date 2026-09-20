import { useSyncExternalStore } from 'react'

/**
 * Widths below this get the phone layout. It equals Tailwind's `lg` breakpoint, which the
 * dashboard shell already uses for its sidebar and bottom navigation, so CSS and JS flip on
 * the same pixel. Tablets in portrait count as phones, as they do in the Android app.
 */
export const MOBILE_BREAKPOINT_PX = 1024

/** The media query behind `useIsMobile`; the one line CSS and JS both quote. */
export const MOBILE_MEDIA_QUERY = `(max-width: ${MOBILE_BREAKPOINT_PX - 1}px)`

/** Reads the viewport once. Use it inside effects and event handlers; renders use `useIsMobile`. */
export function isMobileViewport(): boolean {
  if (typeof window === 'undefined') return false
  return window.matchMedia(MOBILE_MEDIA_QUERY).matches
}

function subscribeToViewport(onChange: () => void): () => void {
  const mediaQueryList = window.matchMedia(MOBILE_MEDIA_QUERY)
  mediaQueryList.addEventListener('change', onChange)
  return () => mediaQueryList.removeEventListener('change', onChange)
}

function getServerSnapshot(): boolean {
  return false
}

/**
 * True below `MOBILE_BREAKPOINT_PX`. The server snapshot is `false`, so hydration renders the
 * desktop tree and React re-renders with the real width right after; the `lg:` classes keep
 * desktop chrome hidden on narrow screens in the meantime.
 */
export function useIsMobile(): boolean {
  return useSyncExternalStore(subscribeToViewport, isMobileViewport, getServerSnapshot)
}
