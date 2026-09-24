import { useEffect, useState } from 'react'
import { getPublicSettings } from '#/lib/use-public-settings'
import { cn } from '@/lib/utils'

/**
 * The build version this instance reports through `/api/auth/settings`, which every signed-in
 * user may read — the admin overview is the only other place that carries it, and that endpoint
 * is superadmin-only.
 *
 * Null until the request settles, and null again if it fails: the version is a label, so a
 * failed fetch renders nothing instead of an error.
 */
export function useAppVersion(): string | null {
  const [version, setVersion] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    void getPublicSettings()
      .then((settings) => {
        if (!cancelled) setVersion(settings.version ?? null)
      })
      .catch(() => {
        /* label only — nothing to report */
      })
    return () => {
      cancelled = true
    }
  }, [])

  return version
}

/** Renders the instance version, or nothing while it is unknown. */
export function AppVersion({ className }: { className?: string }) {
  const version = useAppVersion()
  if (!version) return null

  return (
    <p className={cn('text-[10px] text-muted-foreground', className)}>
      <span className="font-mono">bedrud</span> {version}
    </p>
  )
}
