import { createFileRoute, Link, Outlet, useRouterState } from '@tanstack/react-router'
import { Camera, Lock, Mic, User } from 'lucide-react'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

export const Route = createFileRoute('/dashboard/settings')({
  component: SettingsLayout,
})

const TABS = [
  { to: '/dashboard/settings' as const, label: 'Profile', icon: User, isIndex: true },
  { to: '/dashboard/settings/security' as const, label: 'Security', icon: Lock },
  { to: '/dashboard/settings/audio' as const, label: 'Audio', icon: Mic },
  { to: '/dashboard/settings/video' as const, label: 'Video', icon: Camera },
]

function SettingsLayout() {
  const { location } = useRouterState()
  const path = location.pathname

  const activeTab =
    TABS.find((t) =>
      t.isIndex ? path === '/dashboard/settings' || path === '/dashboard/settings/' : path.startsWith(t.to),
    )?.to ?? TABS[0].to

  return (
    <div className="mx-auto max-w-4xl space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">Settings</h1>
      </div>

      {/* Tab strip */}
      <Tabs value={activeTab}>
        <TabsList>
          {TABS.map(({ to, label, icon: Icon }) => (
            <TabsTrigger key={to} value={to} asChild>
              <Link to={to}>
                <Icon className="h-3.5 w-3.5 mr-1.5" />
                {label}
              </Link>
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      <Outlet />
    </div>
  )
}
