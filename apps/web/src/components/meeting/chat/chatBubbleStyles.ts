import { cn } from '@/lib/utils'

export type BubblePosition = 'only' | 'first' | 'middle' | 'last'

export function bubblePosition(index: number, total: number): BubblePosition {
  if (total === 1) return 'only'
  if (index === 0) return 'first'
  if (index === total - 1) return 'last'
  return 'middle'
}

export function bubbleClassName(
  isLocal: boolean,
  pos: BubblePosition,
  stacked: boolean,
  options?: { media?: boolean },
) {
  const connect = stacked && pos !== 'only' && pos !== 'first'
  return cn(
    'meet-chat-bubble min-w-0',
    isLocal ? 'meet-chat-bubble--out' : 'meet-chat-bubble--in',
    `meet-chat-bubble--${pos}`,
    connect && 'meet-chat-bubble--stacked',
    options?.media && 'meet-chat-bubble--media',
  )
}

export function actionBubbleChrome() {
  return {
    background: 'var(--meet-chat-action-bg)',
    border: '1px solid var(--meet-chat-action-border)',
    color: 'var(--meet-chat-action-fg)',
    boxShadow: 'var(--meet-chat-action-shadow)',
  } as const
}

export function controlBubbleChrome() {
  return {
    background: 'var(--meet-chat-control-bg)',
    border: '1px solid var(--meet-chat-control-border)',
    boxShadow: 'var(--meet-chat-control-shadow)',
  } as const
}
