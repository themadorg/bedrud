import { createContext, type ReactNode, useContext, useMemo } from 'react'

export interface MeetingUILayoutState {
  chatDocked: boolean
  participantsDocked: boolean
}

const MeetingUILayoutContext = createContext<MeetingUILayoutState>({
  chatDocked: false,
  participantsDocked: false,
})

export function MeetingUILayoutProvider({
  chatDocked,
  participantsDocked,
  children,
}: MeetingUILayoutState & { children: ReactNode }) {
  const value = useMemo(
    () => ({ chatDocked, participantsDocked }),
    [chatDocked, participantsDocked],
  )
  return <MeetingUILayoutContext.Provider value={value}>{children}</MeetingUILayoutContext.Provider>
}

export function useMeetingUILayout() {
  return useContext(MeetingUILayoutContext)
}

export function meetRightInsetClass({ chatDocked, participantsDocked }: MeetingUILayoutState) {
  if (chatDocked && participantsDocked) return 'right-[min(calc(320px+288px),100vw)]'
  if (chatDocked) return 'right-[min(320px,100vw)]'
  if (participantsDocked) return 'right-[min(288px,100vw)]'
  return 'right-0'
}

export function meetControlsDockClass({ chatDocked, participantsDocked }: MeetingUILayoutState) {
  if (chatDocked && participantsDocked) return 'left-[calc(50%-min(160px,50vw)-min(144px,50vw))]'
  if (chatDocked) return 'left-[calc(50%-min(160px,50vw))]'
  if (participantsDocked) return 'left-[calc(50%-min(144px,50vw))]'
  return 'left-1/2'
}

export function participantsDockOffset(chatDocked: boolean) {
  return chatDocked ? 'min(320px, 100vw)' : undefined
}
