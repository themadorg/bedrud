import { Package } from 'lucide-react'
import { useEffect, useState } from 'react'
import { API_URL } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { cn } from '#/lib/utils'

/** Package ids from the server are UUID hex only — never pass arbitrary strings into paths. */
const SAFE_PACKAGE_ID = /^[0-9a-fA-F-]{8,64}$/

const SAFE_IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp'])

type Props = {
  /** Server package UUID (instance or room package). */
  packageId?: string
  /** Remote/semi-remote HTTPS icon — only used when packageId icon is not available. */
  remoteIconUrl?: string
  hasIcon?: boolean
  name: string
  className?: string
}

/**
 * Renders a package icon. Instance/room icons load via authenticated fetch → blob URL
 * (Bearer header; plain <img src="/api/..."> cannot send Authorization).
 * Remote catalog icons may use HTTPS iconUrl with referrerPolicy=no-referrer.
 */
export function WebxdcPackageIcon({ packageId, remoteIconUrl, hasIcon, name, className }: Props) {
  const [src, setSrc] = useState<string | null>(null)

  useEffect(() => {
    setSrc(null)
    const id = (packageId || '').trim()
    if (!hasIcon || !id || !SAFE_PACKAGE_ID.test(id)) {
      return
    }
    let cancelled = false
    let objectUrl: string | null = null

    const load = async () => {
      try {
        const token = useAuthStore.getState().tokens?.accessToken
        const headers: Record<string, string> = {}
        if (token) headers.Authorization = `Bearer ${token}`
        const path = `/api/webxdc/packages/${encodeURIComponent(id)}/icon`
        const res = await fetch(`${API_URL}${path}`, { credentials: 'include', headers })
        if (!res.ok || cancelled) return
        const ct = (res.headers.get('Content-Type') || '').split(';')[0]?.trim().toLowerCase() ?? ''
        if (!SAFE_IMAGE_TYPES.has(ct)) return
        const blob = await res.blob()
        if (cancelled) return
        objectUrl = URL.createObjectURL(blob)
        setSrc(objectUrl)
      } catch {
        // Optional decoration — leave placeholder.
      }
    }

    void load()
    return () => {
      cancelled = true
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [packageId, hasIcon])

  // Prefer authenticated local icon; fall back to remote HTTPS URL (external gallery only).
  const displaySrc = src || (remoteIconUrl && /^https:\/\//i.test(remoteIconUrl) ? remoteIconUrl : null)

  if (!displaySrc) {
    return (
      <div
        className={cn(
          'meet-gallery-app-icon flex shrink-0 items-center justify-center bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]',
          className,
        )}
        aria-hidden
      >
        <Package className="h-6 w-6" />
      </div>
    )
  }

  return (
    <img
      src={displaySrc}
      alt=""
      title={name}
      className={cn('meet-gallery-app-icon shrink-0 object-cover', className)}
      referrerPolicy="no-referrer"
      draggable={false}
    />
  )
}
