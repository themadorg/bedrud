import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { isMobileViewport, MOBILE_BREAKPOINT_PX, MOBILE_MEDIA_QUERY, useIsMobile } from './use-is-mobile'

type ChangeListener = (event: MediaQueryListEvent) => void

const originalMatchMedia = window.matchMedia

/** Installs a fake `matchMedia` that evaluates `(max-width: Npx)` against a settable viewport width. */
function installMatchMedia(initialWidth: number) {
  let width = initialWidth
  const listeners = new Set<ChangeListener>()
  const maxWidthOf = (query: string) => Number(/max-width:\s*(\d+)px/.exec(query)?.[1])
  window.matchMedia = vi.fn((query: string) => ({
    get matches() {
      return width <= maxWidthOf(query)
    },
    media: query,
    onchange: null,
    addEventListener: (_type: 'change', listener: ChangeListener) => {
      listeners.add(listener)
    },
    removeEventListener: (_type: 'change', listener: ChangeListener) => {
      listeners.delete(listener)
    },
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })) as unknown as typeof window.matchMedia
  return {
    resize(nextWidth: number) {
      width = nextWidth
      for (const listener of listeners) listener({ media: MOBILE_MEDIA_QUERY } as MediaQueryListEvent)
    },
    listenerCount() {
      return listeners.size
    },
  }
}

afterEach(() => {
  window.matchMedia = originalMatchMedia
})

describe('MOBILE_MEDIA_QUERY', () => {
  it('should stop one pixel below the breakpoint', () => {
    expect(MOBILE_BREAKPOINT_PX).toBe(1024)
    expect(MOBILE_MEDIA_QUERY).toBe('(max-width: 1023px)')
  })
})

describe('isMobileViewport', () => {
  it('should report mobile at 1023px', () => {
    installMatchMedia(1023)
    expect(isMobileViewport()).toBe(true)
  })

  it('should report desktop at 1024px', () => {
    installMatchMedia(1024)
    expect(isMobileViewport()).toBe(false)
  })
})

describe('useIsMobile', () => {
  it('should follow change events across the breakpoint', () => {
    const viewport = installMatchMedia(1024)
    const { result } = renderHook(() => useIsMobile())
    expect(result.current).toBe(false)
    act(() => viewport.resize(1023))
    expect(result.current).toBe(true)
    act(() => viewport.resize(1024))
    expect(result.current).toBe(false)
  })

  it('should stop listening after unmount', () => {
    const viewport = installMatchMedia(1024)
    const { unmount } = renderHook(() => useIsMobile())
    expect(viewport.listenerCount()).toBe(1)
    unmount()
    expect(viewport.listenerCount()).toBe(0)
  })
})
