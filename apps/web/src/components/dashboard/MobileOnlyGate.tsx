import { useNavigate } from '@tanstack/react-router'
import { useEffect, useState, type ReactNode } from 'react'
import { MobileBottomNav } from '@/components/dashboard/MobileBottomNav'

export function MobileOnlyGate({
  desktopTo = '/dashboard',
  children,
}: {
  desktopTo?: string
  children: ReactNode
}) {
  const navigate = useNavigate()
  const [ready, setReady] = useState(false)

  useEffect(() => {
    if (window.matchMedia('(min-width: 1024px)').matches) {
      navigate({ to: desktopTo, replace: true })
      return
    }
    setReady(true)
  }, [desktopTo, navigate])

  if (!ready) return null

  return (
    <div className="min-h-screen bg-background lg:hidden">
      <main
        id="main-content"
        className="p-4 pb-[calc(5.5rem+env(safe-area-inset-bottom,0px))]"
      >
        {children}
      </main>
      <MobileBottomNav />
    </div>
  )
}
