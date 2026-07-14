import { describe, expect, it } from 'vitest'
import { bubbleClassName, bubblePosition } from './chatBubbleStyles'

describe('chatBubbleStyles', () => {
  it('applies incoming single-bubble classes', () => {
    expect(bubbleClassName(false, 'only', false)).toContain('meet-chat-bubble--in')
    expect(bubbleClassName(false, 'only', false)).toContain('meet-chat-bubble--only')
  })

  it('connects middle bubbles in a cluster', () => {
    expect(bubbleClassName(true, 'middle', true)).toContain('meet-chat-bubble--stacked')
    expect(bubbleClassName(true, 'first', true)).not.toContain('meet-chat-bubble--stacked')
    expect(bubblePosition(1, 3)).toBe('middle')
  })
})
