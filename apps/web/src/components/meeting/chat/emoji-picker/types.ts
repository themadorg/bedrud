export interface EmojiItem {
  emoji: string
  name: string
  slug: string
}

export interface EmojiGroup {
  category: string
  slug: string
  emojis: EmojiItem[]
}

export interface EmojiPickerProps {
  onEmojiSelect: (emoji: string) => void
  className?: string
  emojisPerRow?: number
  emojiSize?: number
  containerHeight?: number
  autoFocus?: boolean
}
