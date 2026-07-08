export const DEFAULT_PUSH_TO_TALK_KEY = 'Space'

const MODIFIER_CODES = new Set([
  'ShiftLeft',
  'ShiftRight',
  'ControlLeft',
  'ControlRight',
  'AltLeft',
  'AltRight',
  'MetaLeft',
  'MetaRight',
])

const NAMED_KEY_LABELS: Record<string, string> = {
  Space: 'Space',
  Enter: 'Enter',
  Tab: 'Tab',
  Backspace: 'Backspace',
  Escape: 'Escape',
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
}

export function isModifierKey(code: string): boolean {
  return MODIFIER_CODES.has(code)
}

export function isEditableKeyboardTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return (
    target.tagName === 'INPUT' ||
    target.tagName === 'TEXTAREA' ||
    target.tagName === 'SELECT' ||
    target.isContentEditable
  )
}

/** Human-readable label for a KeyboardEvent.code value. */
export function formatKeyboardCode(code: string): string {
  if (NAMED_KEY_LABELS[code]) return NAMED_KEY_LABELS[code]
  if (code.startsWith('Key')) return code.slice(3)
  if (code.startsWith('Digit')) return code.slice(5)
  if (code.startsWith('Numpad')) return `Num ${code.slice(6)}`
  return code
}

export function normalizePushToTalkKey(code: string | undefined | null): string {
  if (!code || typeof code !== 'string') return DEFAULT_PUSH_TO_TALK_KEY
  return code
}
