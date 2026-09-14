import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type Theme = 'light' | 'dark' | 'system'

interface ThemeStore {
  theme: Theme
  setTheme: (theme: Theme) => void
}

export const useThemeStore = create<ThemeStore>()(
  persist(
    (set) => ({
      theme: 'system',
      setTheme: (theme) => {
        set({ theme })
        if (typeof document === 'undefined') return
        // Only apply if the class isn't already correct (avoids fighting
        // a view-transition that already toggled the class directly).
        const resolved = resolveTheme(theme)
        const isDark = document.documentElement.classList.contains('dark')
        if ((resolved === 'dark') !== isDark) {
          applyTheme(theme)
          return
        }
        // The view transition toggles only the class, so the theme colour still needs refreshing.
        syncThemeColor()
      },
    }),
    { name: 'theme' },
  ),
)

/** Resolves 'system' to the actual OS preference. Returns 'light' on SSR. */
export function resolveTheme(theme: Theme): 'light' | 'dark' {
  if (theme === 'system') {
    if (typeof window === 'undefined') return 'light'
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return theme
}

/**
 * Copies the resolved `--background` into the theme-color meta, so the system bar of an
 * installed app follows the app's own theme instead of the OS preference. The token is read
 * from CSS so the value lives in theme.css only.
 *
 * This function owns the meta outright, creating it when it is absent: React must never render
 * it. React 19 hydrates head metas as hoistables keyed by their content, so a meta the pre-paint
 * script has already recoloured no longer matches what React expects and React appends a second,
 * stale one — leaving the page with two theme-color metas in dark mode.
 */
function syncThemeColor() {
  const background = getComputedStyle(document.documentElement).getPropertyValue('--background').trim()
  if (!background) return
  let meta = document.querySelector('meta[name="theme-color"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.setAttribute('name', 'theme-color')
    document.head.appendChild(meta)
  }
  meta.setAttribute('content', background)
}

/** Applies the correct class to <html> and the matching theme colour. Safe to call outside React. No-op on SSR. */
export function applyTheme(theme: Theme) {
  if (typeof document === 'undefined') return
  const resolved = resolveTheme(theme)
  document.documentElement.classList.toggle('dark', resolved === 'dark')
  syncThemeColor()
}
