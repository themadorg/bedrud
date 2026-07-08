import { describe, expect, it } from 'vitest'
import { alignRtlTextElements } from './whiteboardTextDirection'

describe('alignRtlTextElements', () => {
  it('sets right alignment when text starts with Persian', () => {
    const elements = [
      {
        id: 't1',
        type: 'text',
        originalText: 'سلام',
        textAlign: 'left',
        isDeleted: false,
      },
    ] as never

    const aligned = alignRtlTextElements(elements)
    expect(aligned?.[0]).toMatchObject({ textAlign: 'right' })
  })

  it('uses text field when originalText is empty during editing', () => {
    const elements = [
      {
        id: 't1',
        type: 'text',
        originalText: '',
        text: 'سلام',
        textAlign: 'left',
        isDeleted: false,
      },
    ] as never

    const aligned = alignRtlTextElements(elements)
    expect(aligned?.[0]).toMatchObject({ textAlign: 'right' })
  })

  it('leaves Latin text unchanged', () => {
    const elements = [
      {
        id: 't1',
        type: 'text',
        originalText: 'hello',
        textAlign: 'left',
        isDeleted: false,
      },
    ] as never

    expect(alignRtlTextElements(elements)).toBeNull()
  })
})