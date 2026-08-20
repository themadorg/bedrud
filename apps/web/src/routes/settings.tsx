import { createFileRoute, Link, Outlet, useRouterState } from '@tanstack/react-router'
import { Camera, ChevronLeft, ChevronRight, FlaskConical, Lock, Mic, Palette } from 'lucide-react'
import { loadRegisteredUser, requireRegisteredUser } from '#/lib/require-registered-user'
import { MobileOnlyGate } from '@/components/dashboard/MobileOnlyGate'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/settings')({
  beforeLoad: requireRegisteredUser,
  loader: loadRegisteredUser,
  staleTime: Infinity,
  head: () => ({ meta: [{ title: 'Settings — Bedrud' }] }),
  component: SettingsLayout,
})

const SECTIONS = [
  { to: '/settings/appearance' as const, label: 'Appearance', description: 'Theme and interface', icon: Palette },
  { to: '/settings/audio' as const, label: 'Audio', description: 'Mic, noise, push-to-talk', icon: Mic },
  { to: '/settings/video' as const, label: 'Video', description: 'Camera and quality', icon: Camera },
  { to: '/settings/security' as const, label: 'Security', description: 'Password and sessions', icon: Lock },
  {
    to: '/settings/experimental' as const,
    label: 'Experimental',
    description: 'Whiteboard, YouTube, WebXDC, …',
    icon: FlaskConical,
  },
] as const

function SettingsLayout() {
  const { location } = useRouterState()
  const isIndex = location.pathname === '/settings' || location.pathname === '/settings/'

  return (
    <MobileOnlyGate desktopTo="/dashboard/settings">
      {isIndex ? (
        <div className="space-y-4">
          <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
          <nav aria-label="Settings categories">
            <ul className="m-0 list-none overflow-hidden border border-border bg-muted/40 p-0">
              {SECTIONS.map(({ to, label, description, icon: Icon }, index) => (
                <li key={to} className={cn(index > 0 && 'border-t border-border')}>
                  <Link
                    to={to}
                    className="flex w-full items-center gap-3 bg-transparent px-3.5 py-3 text-start transition-colors active:bg-muted hover:bg-muted"
                  >
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center bg-primary/10 text-primary">
                      <Icon size={16} />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block text-[15px] font-medium text-foreground">{label}</span>
                      <span className="block truncate text-[12px] text-muted-foreground">{description}</span>
                    </span>
                    <ChevronRight size={18} className="shrink-0 text-muted-foreground" />
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        </div>
      ) : (
        <div className="space-y-3">
          <Link to="/settings" className="inline-flex h-11 items-center gap-0.5 text-[15px] text-primary">
            <ChevronLeft size={22} className="shrink-0" />
            Settings
          </Link>
          <Outlet />
        </div>
      )}
    </MobileOnlyGate>
  )
}
