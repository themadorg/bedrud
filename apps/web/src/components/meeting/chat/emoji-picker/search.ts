import type { EmojiGroup, EmojiItem } from './types'

export function searchEmojis(groups: EmojiGroup[], query: string): EmojiItem[] {
  const q = query.trim().toLowerCase()
  if (!q) return []

  const results: EmojiItem[] = []
  const seen = new Set<string>()

  for (const group of groups) {
    for (const item of group.emojis) {
      if (seen.has(item.emoji)) continue
      const haystack = `${item.name} ${item.slug}`.toLowerCase()
      if (haystack.includes(q)) {
        seen.add(item.emoji)
        results.push(item)
      }
    }
  }

  return results
}
