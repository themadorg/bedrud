import { describe, expect, it } from 'vitest'
import { KEYBOARD_SHRINK_THRESHOLD_PX, shouldExpandForKeyboard } from './sheetKeyboard'

describe('shouldExpandForKeyboard', () => {
  it('should treat a large shrink as a keyboard', () => {
    expect(shouldExpandForKeyboard(812, 480)).toBe(true)
  })

  it('should not treat a browser toolbar as a keyboard', () => {
    expect(shouldExpandForKeyboard(812, 760)).toBe(false)
  })

  it('should not expand when the viewport grows back', () => {
    expect(shouldExpandForKeyboard(480, 812)).toBe(false)
  })

  it('should not expand when the viewport does not move', () => {
    expect(shouldExpandForKeyboard(812, 812)).toBe(false)
  })

  it('should sit clear of a browser toolbar and under the shortest keyboard', () => {
    expect(KEYBOARD_SHRINK_THRESHOLD_PX).toBeGreaterThan(60)
    expect(KEYBOARD_SHRINK_THRESHOLD_PX).toBeLessThan(200)
  })
})
