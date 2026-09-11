/** Which surface the in-call chat draws on. */
export type ChatSurface =
  /** A bottom sheet over the live call. Phones. */
  | 'sheet'
  /** The left dock inside expanded WebXDC, which owns its own chrome. */
  | 'elevated'
  /** A 320px sidebar the stage is inset for. Desktop, pinned. */
  | 'dock'
  /** A 320px panel floating over the stage. Desktop, unpinned. */
  | 'overlay'

interface ChatSurfaceInput {
  /** Below the app's single 1024px breakpoint. */
  isMobile: boolean
  /** The desktop pin. Phones have no pin control, so this is ignored there. */
  stuck: boolean
  /** Opened from expanded WebXDC, which stacks its own dock above everything. */
  elevated: boolean
}

/**
 * Resolves the presentation once, so the four booleans are read in one place rather than recombined
 * at each use. Order matters: the elevated dock outranks the width, because expanded WebXDC owns the
 * whole screen at any size, and the phone sheet outranks the pin, because the pin is a desktop
 * control that a phone can still be carrying from a wider window.
 */
export function chatSurfaceFor({ isMobile, stuck, elevated }: ChatSurfaceInput): ChatSurface {
  if (elevated) return 'elevated'
  if (isMobile) return 'sheet'
  return stuck ? 'dock' : 'overlay'
}
