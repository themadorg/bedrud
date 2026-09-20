import { createFileRoute, Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { Camera, Lock, Mic, User } from 'lucide-react'
import { useEffect } from 'react'
import { isMobileViewport } from '#/lib/use-is-mobile'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/dashboard/settings')({
  component: SettingsLayout,
})

const TABS = [
  { to: '/dashboard/settings' as const, label: 'Profile', icon: User, isIndex: true },
  { to: '/dashboard/settings/security' as const, label: 'Security', icon: Lock },
  { to: '/dashboard/settings/audio' as const, label: 'Audio', icon: Mic },
  { to: '/dashboard/settings/video' as const, label: 'Video', icon: Camera },
]

/**
 * Sends a phone visitor to the phone settings page, mirroring the redirect `MobileOnlyGate`
 * already performs in the other direction. The viewport is read inside the effect rather than
 * through the hook, because the hook reports "desktop" in its server snapshot: an effect keyed on
 * it would see false for a phone on the first hydrated render and leave the visitor on the desktop
 * page. `MobileOnlyGate` reads it the same way for the mirror-image reason, its condition being
 * negated there.
 */
function useRedirectPhonesToPhoneSettings() {
  const navigate = useNavigate()

  useEffect(() => {
    if (isMobileViewport()) navigate({ to: '/settings', replace: true })
  }, [navigate])
}

function SettingsLayout() {
  useRedirectPhonesToPhoneSettings()
  const { location } = useRouterState()
  const path = location.pathname

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Settings</h1>
      </div>

      {/* Plain links (not Radix Tabs) so all sections stay visible — Profile / Security / Audio / Video */}
      <nav
        className="flex flex-wrap gap-1 rounded-lg border bg-muted p-1 text-muted-foreground"
        aria-label="Settings sections"
      >
        {TABS.map(({ to, label, icon: Icon, isIndex }) => {
          const active = isIndex
            ? path === '/dashboard/settings' || path === '/dashboard/settings/'
            : path.startsWith(to)
          return (
            <Link
              key={to}
              to={to}
              className={cn(
                'inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
                active ? 'bg-background text-foreground shadow-sm' : 'hover:bg-background/60 hover:text-foreground',
              )}
            >
              <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden />
              {label}
            </Link>
          )
        })}
      </nav>

      <Outlet />
    </div>
  )
}
