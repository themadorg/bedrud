import type { CSSProperties, ReactNode, Ref } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '@/lib/utils'

/** Shared size for elevated chat / settings / room-info docks (flush past expand rail). */
export const MEETING_ELEVATED_DOCK_WIDTH = 'min(360px, calc(var(--app-width, 100svw) - 3rem))'

/** Above expanded WebXDC (inline zIndex 200). */
export const MEETING_ELEVATED_Z = 250

const dockStyle: CSSProperties = {
  zIndex: MEETING_ELEVATED_Z,
  // Flush against expand left rail (w-12 = 3rem).
  left: 'calc(var(--app-offset-left, 0px) + 3rem)',
  top: 'var(--app-offset-top, 0px)',
  height: 'var(--app-height, 100svh)',
  width: MEETING_ELEVATED_DOCK_WIDTH,
  maxHeight: 'var(--app-height, 100svh)',
}

type Marker = 'chat' | 'settings' | 'info'

const markerAttr: Record<Marker, string> = {
  chat: 'data-elevated-chat',
  settings: 'data-elevated-settings',
  info: 'data-elevated-room-info',
}

type Props = {
  /** Accessibility label for the dialog. */
  label: string
  /** Marker for Escape / left-rail coordination. */
  marker: Marker
  children: ReactNode
  className?: string
  /** Optional ref (e.g. focus trap) on the shell. */
  shellRef?: Ref<HTMLElement>
}

/**
 * Shared body-portaled left dock for expanded WebXDC chrome panels.
 * Same position, width, z-index, and chrome for chat / settings / room info.
 */
export function MeetingElevatedLeftDock({ label, marker, children, className, shellRef }: Props) {
  if (typeof document === 'undefined') return null

  const markerProps = { [markerAttr[marker]]: 'true' as const }

  return createPortal(
    <aside
      ref={shellRef}
      role="dialog"
      aria-modal="true"
      aria-label={label}
      {...markerProps}
      style={dockStyle}
      className={cn(
        'meet-dialog fixed z-[250] flex flex-col overflow-hidden bg-[var(--meet-sidebar)] shadow-2xl backdrop-blur-2xl',
        'border-r border-[var(--meet-border-subtle)]',
        'pt-[env(safe-area-inset-top,0px)] pb-[env(safe-area-inset-bottom,0px)]',
        'animate-in fade-in-0 slide-in-from-left duration-200',
        className,
      )}
    >
      {children}
    </aside>,
    document.body,
  )
}
