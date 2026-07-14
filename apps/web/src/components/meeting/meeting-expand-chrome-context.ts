import { createContext, useContext } from 'react'

export type MeetingExpandChromeHandlers = {
  openChat: () => void
  openInfo: () => void
  closeChat: () => void
  closeInfo: () => void
}

export const MeetingExpandChromeContext = createContext<MeetingExpandChromeHandlers | null>(null)

export function useMeetingExpandChromeHandlers(): MeetingExpandChromeHandlers {
  const ctx = useContext(MeetingExpandChromeContext)
  if (!ctx) {
    throw new Error('useMeetingExpandChromeHandlers must be used within MeetingExpandChromeProvider')
  }
  return ctx
}
