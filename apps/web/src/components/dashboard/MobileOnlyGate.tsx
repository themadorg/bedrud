import { useNavigate } from '@tanstack/react-router'
import { type ReactNode, useEffect, useState } from 'react'
import { MobileBottomNav } from '@/components/dashboard/MobileBottomNav'
import { isMobileViewport } from '@/lib/use-is-mobile'

export function MobileOnlyGate({ desktopTo = '/dashboard', children }: { desktopTo?: string; children: ReactNode }) {
  const navigate = useNavigate()
  const [ready, setReady] = useState(false)

  // Reads the viewport inside the effect rather than through the hook: the hook's server
  // snapshot is "desktop", and an effect keyed on it would bounce phones away on hydration.
  useEffect(() => {
    if (!isMobileViewport()) {
      navigate({ to: desktopTo, replace: true })
      return
    }
    setReady(true)
  }, [desktopTo, navigate])

  if (!ready) return null

  return (
    <div className="min-h-screen bg-background lg:hidden">
      <main id="main-content" className="p-4 pb-[calc(5.5rem+env(safe-area-inset-bottom,0px))]">
        {children}
      </main>
      <MobileBottomNav />
    </div>
  )
}
