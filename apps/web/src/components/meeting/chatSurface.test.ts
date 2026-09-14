import { describe, expect, it } from 'vitest'
import { chatSurfaceFor } from './chatSurface'

describe('chatSurfaceFor', () => {
  it('should put chat on a sheet below the phone breakpoint', () => {
    expect(chatSurfaceFor({ isMobile: true, stuck: false, elevated: false })).toBe('sheet')
  })

  it('should ignore a stuck pin on a phone, because the pin is desktop-only', () => {
    expect(chatSurfaceFor({ isMobile: true, stuck: true, elevated: false })).toBe('sheet')
  })

  it('should use the elevated dock whatever the width, because it belongs to expanded WebXDC', () => {
    expect(chatSurfaceFor({ isMobile: true, stuck: false, elevated: true })).toBe('elevated')
    expect(chatSurfaceFor({ isMobile: false, stuck: true, elevated: true })).toBe('elevated')
  })

  it('should dock a pinned desktop chat', () => {
    expect(chatSurfaceFor({ isMobile: false, stuck: true, elevated: false })).toBe('dock')
  })

  it('should overlay an unpinned desktop chat', () => {
    expect(chatSurfaceFor({ isMobile: false, stuck: false, elevated: false })).toBe('overlay')
  })
})
