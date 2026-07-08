import { describe, expect, it } from 'vitest'
import { bubblePosition, bubbleRadius } from './chatBubbleStyles'

describe('chatBubbleStyles', () => {
  it('uses asymmetric corners for single remote bubble', () => {
    expect(bubbleRadius(false, 'only')).toBe('16px 16px 16px 4px')
  })

  it('connects middle bubbles in a cluster', () => {
    expect(bubbleRadius(true, 'middle')).toBe('16px 4px 4px 16px')
    expect(bubblePosition(1, 3)).toBe('middle')
  })
})