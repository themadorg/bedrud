import { createContext, type ReactNode, useContext, useMemo } from 'react'
import { cn } from '@/lib/utils'

export interface MeetingUILayoutState {
  chatDocked: boolean
  participantsDocked: boolean
}

/** Mobile bottom filmstrip height (tiles + padding) above the controls bar. */
export const MEET_MOBILE_FILMSTRIP_H = 72
/** Approx. mobile controls pill clearance (matches ControlsBar bottom layout). */
export const MEET_MOBILE_CONTROLS_H = 68

const MeetingUILayoutContext = createContext<MeetingUILayoutState>({
  chatDocked: false,
  participantsDocked: false,
})

export function MeetingUILayoutProvider({
  chatDocked,
  participantsDocked,
  children,
}: MeetingUILayoutState & { children: ReactNode }) {
  const value = useMemo(() => ({ chatDocked, participantsDocked }), [chatDocked, participantsDocked])
  return <MeetingUILayoutContext.Provider value={value}>{children}</MeetingUILayoutContext.Provider>
}

export function useMeetingUILayout() {
  return useContext(MeetingUILayoutContext)
}

/**
 * Right inset for docked side panels.
 * Mobile never side-docks the video strip (it becomes a bottom filmstrip).
 */
export function meetRightInsetClass({ chatDocked, participantsDocked }: MeetingUILayoutState) {
  if (chatDocked && participantsDocked) {
    return 'right-0 sm:right-[min(calc(320px+288px),var(--app-width,100svw))]'
  }
  if (chatDocked) {
    return 'right-0 sm:right-[min(320px,var(--app-width,100svw))]'
  }
  if (participantsDocked) {
    return 'right-0 sm:right-[min(288px,var(--app-width,100svw))]'
  }
  return 'right-0'
}

/**
 * Bottom clearance for stage/screenshare overlays.
 * Desktop: room for controls. Mobile + filmstrip: controls + small video strip.
 */
export function meetStageBottomClass({ participantsDocked }: Pick<MeetingUILayoutState, 'participantsDocked'>) {
  if (participantsDocked) {
    // Keep full class strings static for Tailwind. Values match MEET_MOBILE_* constants (68 + 72).
    return cn(
      'bottom-[calc(88px+env(safe-area-inset-bottom))]',
      'max-sm:bottom-[calc(68px+72px+env(safe-area-inset-bottom))]',
    )
  }
  return 'bottom-[calc(88px+env(safe-area-inset-bottom))]'
}

/** Combined shell classes for stage / youtube / whiteboard overlays. */
export function meetStageShellClass(layout: MeetingUILayoutState, extra?: string) {
  return cn(
    'absolute top-[calc(56px+env(safe-area-inset-top))] left-0 z-[5] flex flex-col transition-[right,bottom] duration-200',
    meetRightInsetClass(layout),
    meetStageBottomClass(layout),
    extra,
  )
}

export function meetControlsDockClass({ chatDocked, participantsDocked }: MeetingUILayoutState) {
  // Mobile: always centered (filmstrip is bottom, not side).
  if (chatDocked && participantsDocked) {
    return 'left-1/2 sm:left-[calc(50%-min(160px,50vw)-min(144px,50vw))]'
  }
  if (chatDocked) {
    return 'left-1/2 sm:left-[calc(50%-min(160px,50vw))]'
  }
  if (participantsDocked) {
    return 'left-1/2 sm:left-[calc(50%-min(144px,50vw))]'
  }
  return 'left-1/2'
}

export function participantsDockOffset(chatDocked: boolean) {
  return chatDocked ? 'min(320px, var(--app-width, 100svw))' : undefined
}
