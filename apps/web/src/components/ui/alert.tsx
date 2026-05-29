import { AlertCircle, Check } from 'lucide-react'
import { cn } from '#/lib/utils'

export interface AlertProps {
  type: 'success' | 'error'
  message: string
  className?: string
}

/**
 * Minimal inline status alert used for form feedback (success/error).
 * Extracted from duplicated local definitions in dashboard settings pages.
 */
export function Alert({ type, message, className }: AlertProps) {
  return (
    <div
      className={cn(
        'flex items-center gap-2 border px-3 py-2 text-xs',
        type === 'success'
          ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
          : 'border-destructive/30 bg-destructive/10 text-destructive',
        className,
      )}
    >
      {type === 'success' ? (
        <Check className="h-3.5 w-3.5 shrink-0" />
      ) : (
        <AlertCircle className="h-3.5 w-3.5 shrink-0" />
      )}
      {message}
    </div>
  )
}
