export type BubblePosition = 'only' | 'first' | 'middle' | 'last'

export function bubblePosition(index: number, total: number): BubblePosition {
  if (total === 1) return 'only'
  if (index === 0) return 'first'
  if (index === total - 1) return 'last'
  return 'middle'
}

export function bubbleRadius(isLocal: boolean, pos: BubblePosition): string {
  if (isLocal) {
    if (pos === 'only' || pos === 'first') return '16px 16px 4px 16px'
    if (pos === 'middle') return '16px 4px 4px 16px'
    return '16px 4px 16px 16px'
  }
  if (pos === 'only' || pos === 'first') return '16px 16px 16px 4px'
  if (pos === 'middle') return '4px 16px 16px 4px'
  return '4px 16px 16px 16px'
}

export function bubbleChrome(isLocal: boolean, pos: BubblePosition, stacked: boolean) {
  const connect = stacked && pos !== 'only' && pos !== 'first'
  const base = {
    borderRadius: bubbleRadius(isLocal, pos),
    background: isLocal ? 'var(--meet-chat-bubble-out-bg)' : 'var(--meet-chat-bubble-in-bg)',
    border: isLocal ? '1px solid var(--meet-chat-bubble-out-border)' : '1px solid var(--meet-chat-bubble-in-border)',
    marginTop: connect ? -1 : 0,
    color: isLocal ? 'var(--meet-chat-bubble-out-fg)' : 'var(--meet-chat-bubble-in-fg)',
    boxShadow: isLocal ? 'var(--meet-chat-bubble-out-shadow)' : 'var(--meet-chat-bubble-in-shadow)',
  }

  if (!connect) return base

  return {
    ...base,
    borderTop: 'none',
  }
}

export function actionBubbleChrome() {
  return {
    borderRadius: '16px 16px 4px 16px',
    background: 'var(--meet-chat-action-bg)',
    border: '1px solid var(--meet-chat-action-border)',
    color: 'var(--meet-chat-action-fg)',
    boxShadow: 'var(--meet-chat-action-shadow)',
  } as const
}

export function controlBubbleChrome() {
  return {
    borderRadius: '16px 16px 16px 4px',
    background: 'var(--meet-chat-control-bg)',
    border: '1px solid var(--meet-chat-control-border)',
    boxShadow: 'var(--meet-chat-control-shadow)',
  } as const
}
