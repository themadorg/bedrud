const EMOJI_SEGMENTER = new Intl.Segmenter(undefined, { granularity: 'grapheme' })

function isEmojiGrapheme(cluster: string): boolean {
  if (/^\p{RI}{2}$/u.test(cluster)) return true
  if (/^[0-9#*]\uFE0F?\u20E3$/u.test(cluster)) return true
  return /\p{Extended_Pictographic}/u.test(cluster)
}

/** True when trimmed text is exactly one emoji grapheme cluster. */
export function isSingleEmojiText(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return false

  const segments = [...EMOJI_SEGMENTER.segment(trimmed)]
  if (segments.length !== 1) return false

  const cluster = segments[0].segment
  return cluster === trimmed && isEmojiGrapheme(cluster)
}

export function isSingleEmojiMessage(message: {
  message: string
  attachments: unknown[]
  poll?: unknown
}): boolean {
  if (message.attachments.length > 0 || message.poll) return false
  return isSingleEmojiText(message.message)
}