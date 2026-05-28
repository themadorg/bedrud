import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import type { ChatMessage } from '../MeetingContext'
import { MAX_INITIAL_LOAD, useChatPersistence } from './useChatPersistence'

function makeMsg(id: string): ChatMessage {
  return {
    id,
    timestamp: Date.now(),
    senderName: 'Alice',
    senderIdentity: 'alice',
    message: 'hello',
    attachments: [],
    isLocal: false,
  }
}

beforeEach(() => sessionStorage.clear())
afterEach(() => sessionStorage.clear())

describe('useChatPersistence', () => {
  it('returns empty array when sessionStorage has no entry', () => {
    const { result } = renderHook(() => useChatPersistence('room-1', 10000, 2160))
    const [initial] = result.current
    expect(initial).toEqual([])
  })

  it('returns stored messages on mount', () => {
    const msgs = [makeMsg('a'), makeMsg('b')]
    sessionStorage.setItem('chat:room-1', JSON.stringify(msgs))
    const { result } = renderHook(() => useChatPersistence('room-1', 10000, 2160))
    const [initial] = result.current
    expect(initial).toHaveLength(2)
    expect(initial[0].id).toBe('a')
  })

  it('persist() writes messages to sessionStorage', () => {
    const { result } = renderHook(() => useChatPersistence('room-2', 10000, 2160))
    const [, persist] = result.current
    const msgs = [makeMsg('x')]
    act(() => persist(msgs))
    const stored = JSON.parse(sessionStorage.getItem('chat:room-2') ?? '[]') as ChatMessage[]
    expect(stored).toHaveLength(1)
    expect(stored[0].id).toBe('x')
  })

  it('scopes storage to roomId', () => {
    const { result: r1 } = renderHook(() => useChatPersistence('room-A', 10000, 2160))
    const { result: r2 } = renderHook(() => useChatPersistence('room-B', 10000, 2160))
    act(() => r1.current[1]([makeMsg('from-A')]))
    act(() => r2.current[1]([makeMsg('from-B')]))
    const a = JSON.parse(sessionStorage.getItem('chat:room-A') ?? '[]') as ChatMessage[]
    const b = JSON.parse(sessionStorage.getItem('chat:room-B') ?? '[]') as ChatMessage[]
    expect(a[0].id).toBe('from-A')
    expect(b[0].id).toBe('from-B')
  })

  it('returns empty array when stored JSON is malformed', () => {
    sessionStorage.setItem('chat:room-bad', 'not-json{{{')
    const { result } = renderHook(() => useChatPersistence('room-bad', 10000, 2160))
    const [initial] = result.current
    expect(initial).toEqual([])
  })

  it('filters expired messages on restore when TTL is set', () => {
    const fresh = makeMsg('fresh')
    const stale = makeMsg('stale')
    stale.timestamp = Date.now() - 100 * 60 * 60 * 1000 // 100 hours ago
    sessionStorage.setItem('chat:room-ttl', JSON.stringify([fresh, stale]))
    const { result } = renderHook(() => useChatPersistence('room-ttl', 10000, 48)) // TTL = 48 hours
    const [initial] = result.current
    expect(initial).toHaveLength(1)
    expect(initial[0].id).toBe('fresh')
  })

  it('trims to max count on persist', () => {
    const { result } = renderHook(() => useChatPersistence('room-trim', 2, 0)) // max 2, TTL off
    const [, persist] = result.current
    const msgs = [makeMsg('a'), makeMsg('b'), makeMsg('c')]
    act(() => persist(msgs))
    const stored = JSON.parse(sessionStorage.getItem('chat:room-trim') ?? '[]') as ChatMessage[]
    expect(stored).toHaveLength(2)
    expect(stored[0].id).toBe('b')
    expect(stored[1].id).toBe('c')
  })

  it('caps initial load when storage exceeds MAX_INITIAL_LOAD', () => {
    const excess = MAX_INITIAL_LOAD + 50
    const msgs = Array.from({ length: excess }, (_, i) => makeMsg(`msg-${i}`))
    sessionStorage.setItem('chat:room-excess', JSON.stringify(msgs))
    const { result } = renderHook(() => useChatPersistence('room-excess', 10000, 0))
    const [initial] = result.current
    expect(initial).toHaveLength(MAX_INITIAL_LOAD)
    expect(initial[0].id).toBe(`msg-${excess - MAX_INITIAL_LOAD}`)
    expect(initial[MAX_INITIAL_LOAD - 1].id).toBe(`msg-${excess - 1}`)
  })

  it('filters expired messages on persist when TTL is set', () => {
    const { result } = renderHook(() => useChatPersistence('room-persist-ttl', 100, 48))
    const [, persist] = result.current
    const fresh = makeMsg('fresh')
    const stale = makeMsg('stale')
    stale.timestamp = Date.now() - 100 * 60 * 60 * 1000 // 100 hours ago
    act(() => persist([fresh, stale]))
    const stored = JSON.parse(sessionStorage.getItem('chat:room-persist-ttl') ?? '[]') as ChatMessage[]
    expect(stored).toHaveLength(1)
    expect(stored[0].id).toBe('fresh')
  })
})
