import { createContext, useContext } from 'react'
import type * as Y from 'yjs'

export interface WhiteboardSession {
  hostIdentity: string
  hostName: string
  updatedAt: number
}

export function whiteboardSessionKey(session: WhiteboardSession): string {
  return `${session.hostIdentity}:${session.updatedAt}`
}

export interface WhiteboardWatchContextValue {
  session: WhiteboardSession | null
  isHost: boolean
  ydoc: Y.Doc | null
  pendingOpen: boolean
  whiteboardVisible: boolean
  requestStartWhiteboard: () => string | null
  confirmStartWhiteboard: () => string | null
  cancelStartWhiteboard: () => void
  acceptWhiteboard: () => void
  declineWhiteboard: () => void
  stopWhiteboard: () => void
  flushWhiteboardSync: () => void
}

/** Isolated module so provider and hooks always share one React context instance. */
export const WhiteboardWatchContext = createContext<WhiteboardWatchContextValue | null>(null)

export function useWhiteboardWatch(): WhiteboardWatchContextValue {
  const ctx = useContext(WhiteboardWatchContext)
  if (!ctx) throw new Error('useWhiteboardWatch must be used inside WhiteboardWatchProvider')
  return ctx
}