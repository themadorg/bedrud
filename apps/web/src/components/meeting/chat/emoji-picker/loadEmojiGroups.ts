import type { EmojiGroup } from './types'

let cache: EmojiGroup[] | null = null
let pending: Promise<EmojiGroup[]> | null = null

export function loadEmojiGroups(): Promise<EmojiGroup[]> {
  if (cache) return Promise.resolve(cache)
  if (pending) return pending

  pending = import('./data/emoji-groups.json').then((mod) => {
    cache = mod.default as EmojiGroup[]
    return cache
  })

  return pending
}
