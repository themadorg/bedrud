import type { ExcalidrawTextElement, OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import { textStartsRtl } from '#/lib/text-direction'

/** Text content used for direction detection (wysiwyg may only have `text` populated). */
export function textElementContent(textEl: ExcalidrawTextElement): string {
  return (textEl.originalText || textEl.text || '').trimStart()
}

/** Align new/edited RTL text (e.g. Persian) to the right on the whiteboard. */
export function alignRtlTextElements(
  elements: readonly OrderedExcalidrawElement[],
): OrderedExcalidrawElement[] | null {
  let changed = false

  const next = elements.map((el) => {
    if (el.type !== 'text' || el.isDeleted) return el

    const textEl = el as ExcalidrawTextElement
    if (!textStartsRtl(textElementContent(textEl))) return el
    if (textEl.textAlign === 'right') return el

    changed = true
    return { ...textEl, textAlign: 'right' as const } as OrderedExcalidrawElement
  })

  return changed ? next : null
}