import { Link } from '@tanstack/react-router'
import { AlertTriangle, ArrowLeft, Home, Radio, RefreshCw, Server, WifiOff } from 'lucide-react'
import { useIntl } from 'react-intl'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/utils'

/**
 * Parse raw API error strings like "404: {\"error\":\"Room not found\"}"
 * into human-readable messages.
 */
export function parseApiError(raw: string): { code?: number; message: string } {
  const match = raw.match(/^(\d{3}):\s*(.*)$/s)
  if (!match) return { message: raw }

  const code = Number(match[1])
  const body = match[2].trim()
  try {
    const parsed = JSON.parse(body) as { error?: string; message?: string }
    return { code, message: parsed.error ?? parsed.message ?? body }
  } catch {
    return { code, message: body }
  }
}

/** Map of known error codes to friendly descriptions. */
const FRIENDLY_ERRORS: Record<number, { title: string; description: string }> = {
  400: { title: 'Bad request', description: 'The request was invalid. Please check your input and try again.' },
  401: { title: 'Not authenticated', description: 'You need to sign in to access this.' },
  403: { title: 'Access denied', description: "You don't have permission to access this." },
  404: { title: 'Not found', description: "The resource you're looking for doesn't exist." },
  410: { title: 'No longer available', description: 'This room has been closed.' },
  429: { title: 'Too many requests', description: 'Please wait a moment and try again.' },
  500: { title: 'Server error', description: 'Something went wrong on our end. Please try again later.' },
}

/** Room-specific overrides for error codes. */
const ROOM_FRIENDLY_ERRORS: Record<number, { title: string; description: string }> = {
  404: { title: 'Room not found', description: "This room doesn't exist or has been deleted." },
  410: { title: 'Room closed', description: 'This room is no longer active.' },
  403: { title: 'Room access denied', description: "You don't have permission to join this room." },
}

type ErrorVariant = 'not-found' | 'room-error' | 'kicked' | 'session' | 'server'

interface ErrorPageProps {
  variant?: ErrorVariant
  title?: string
  description?: string
  error?: string
  showHome?: boolean
  showBack?: boolean
  onRetry?: () => void
  className?: string
}

const VARIANT_CONFIG: Record<
  ErrorVariant,
  { icon: typeof AlertTriangle; defaultTitleId: string; defaultDescriptionId: string }
> = {
  'not-found': {
    icon: Radio,
    defaultTitleId: 'error.notFound.title',
    defaultDescriptionId: 'error.notFound.description',
  },
  'room-error': {
    icon: WifiOff,
    defaultTitleId: 'error.roomError.title',
    defaultDescriptionId: 'error.roomError.description',
  },
  kicked: {
    icon: Server,
    defaultTitleId: 'error.kicked.title',
    defaultDescriptionId: 'error.kicked.description',
  },
  session: {
    icon: AlertTriangle,
    defaultTitleId: 'error.session.title',
    defaultDescriptionId: 'error.session.description',
  },
  server: {
    icon: AlertTriangle,
    defaultTitleId: 'error.server.title',
    defaultDescriptionId: 'error.server.description',
  },
}

export function ErrorPage({
  variant = 'not-found',
  title,
  description,
  error,
  showHome = true,
  showBack = true,
  onRetry,
  className,
}: ErrorPageProps) {
  const intl = useIntl()
  const config = VARIANT_CONFIG[variant]
  const Icon = config.icon

  // Parse raw API error into a human-readable message
  const parsed = error ? parseApiError(error) : null
  const lookup = variant === 'room-error' ? ROOM_FRIENDLY_ERRORS : FRIENDLY_ERRORS
  const friendly = parsed?.code ? lookup[parsed.code] : undefined

  const displayTitle = title ?? friendly?.title ?? intl.formatMessage({ id: config.defaultTitleId })
  const displayDescription = description ?? friendly?.description ?? intl.formatMessage({ id: config.defaultDescriptionId })

  return (
    <div className={cn('flex min-h-screen flex-col bg-background', className)}>
      {/* Header */}
      <header className="flex items-center gap-2 border-b px-6 py-3">
        <Link to="/" className="flex items-center gap-2">
          <div className="flex h-6 w-6 items-center justify-center bg-primary">
            <Radio className="h-3 w-3 text-primary-foreground" />
          </div>
          <span className="font-mono text-xs font-semibold tracking-tight">bedrud</span>
        </Link>
      </header>

      {/* Body */}
      <main className="flex flex-1 flex-col items-center justify-center gap-6 px-6 py-20">
        {/* Icon in a bordered square */}
        <div className="flex h-16 w-16 items-center justify-center border text-muted-foreground">
          <Icon className="h-7 w-7" strokeWidth={1.5} />
        </div>

        {/* Error code badge */}
        {(variant === 'not-found' || parsed?.code) && (
          <Badge variant="outline" className="font-mono text-[10px] font-semibold uppercase tracking-widest">
            {parsed?.code ?? 404}
          </Badge>
        )}

        {/* Text */}
        <div className="max-w-sm text-center" role="alert" aria-live="assertive">
          <h1 className="text-lg font-semibold">{displayTitle}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{displayDescription}</p>
        </div>

        {/* Parsed error detail */}
        {parsed?.message && !friendly && (
          <Card className="max-w-md px-4 py-3">
            <p className="break-words text-xs text-foreground/80">{parsed.message}</p>
          </Card>
        )}

        {/* Actions */}
        <div className="flex items-center gap-3 pt-2">
          {onRetry && (
            <Button variant="default" size="sm" onClick={onRetry}>
              <RefreshCw className="h-3.5 w-3.5" />
              Retry
            </Button>
          )}
          {showBack && (
            <Button variant="outline" size="sm" onClick={() => window.history.back()}>
              <ArrowLeft className="h-3.5 w-3.5" />
              Go back
            </Button>
          )}
          {showHome && (
            <Link
              to="/"
              className="inline-flex h-9 items-center gap-2 bg-primary px-4 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90"
            >
              <Home className="h-3.5 w-3.5" />
              Home
            </Link>
          )}
        </div>
      </main>
    </div>
  )
}
