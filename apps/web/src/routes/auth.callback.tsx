import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { type ServerUser, toUser, useUserStore } from '#/lib/user.store'

export const Route = createFileRoute('/auth/callback')({
  component: OAuthCallback,
})

function OAuthCallback() {
  const navigate = useNavigate()
  const clearTokens = useAuthStore((s) => s.clear)
  const setUser = useUserStore((s) => s.setUser)
  const cancelledRef = useRef(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    cancelledRef.current = false
    // The server's OAuth callback already set the access_token as an
    // HTTP-only cookie (see server commit be894ef).  The auth middleware
    // reads it from that cookie when no Authorization header is present,
    // so /api/auth/me will authenticate purely via the cookie — no token
    // in the URL, no JS-accessible token needed.
    api
      .get<ServerUser>('/api/auth/me')
      .then((me) => {
        if (cancelledRef.current) return
        setUser(toUser(me))
        navigate({ to: '/dashboard' })
      })
      .catch(() => {
        if (cancelledRef.current) return
        // Cookie missing, expired, or rejected — show error then redirect
        clearTokens()
        setError('OAuth sign-in failed. Redirecting to login…')
        setTimeout(() => {
          if (!cancelledRef.current) {
            navigate({ to: '/auth' })
          }
        }, 1500)
      })
    return () => {
      cancelledRef.current = true
    }
  }, [navigate, clearTokens, setUser])

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center gap-3">
        <p className="text-destructive text-sm">{error}</p>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center gap-3">
      <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      <p className="text-muted-foreground text-sm">Signing you in…</p>
    </div>
  )
}
