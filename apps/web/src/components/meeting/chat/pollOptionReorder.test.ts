import { describe, expect, it } from 'vitest'
import { reorderItems } from './pollOptionReorder'

describe('reorderItems', () => {
  it('moves an item down', () => {
    expect(reorderItems(['a', 'b', 'c'], 0, 2)).toEqual(['b', 'c', 'a'])
  })

  it('moves an item up', () => {
    expect(reorderItems(['a', 'b', 'c'], 2, 0)).toEqual(['c', 'a', 'b'])
  })

  it('returns the same array for invalid indices', () => {
    expect(reorderItems(['a', 'b'], 0, 0)).toEqual(['a', 'b'])
    expect(reorderItems(['a', 'b'], -1, 1)).toEqual(['a', 'b'])
    expect(reorderItems(['a', 'b'], 0, 5)).toEqual(['a', 'b'])
  })
})
