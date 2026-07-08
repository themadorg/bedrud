import type { AppState } from '@excalidraw/excalidraw/types'
import { describe, expect, it } from 'vitest'
import { pickSyncableSettings, settingsSignature } from './whiteboardSyncSettings'

describe('whiteboardSyncSettings', () => {
  it('picks canvas background and grid from app state', () => {
    expect(
      pickSyncableSettings({
        viewBackgroundColor: '#1e1e2e',
        gridModeEnabled: true,
      } as AppState),
    ).toEqual({ viewBackgroundColor: '#1e1e2e', gridModeEnabled: true })
  })

  it('detects background signature changes', () => {
    const before = settingsSignature({ viewBackgroundColor: '#ffffff', gridModeEnabled: false })
    const after = settingsSignature({ viewBackgroundColor: '#000000', gridModeEnabled: false })
    expect(before).not.toBe(after)
  })

  it('detects grid signature changes', () => {
    const before = settingsSignature({ viewBackgroundColor: '#ffffff', gridModeEnabled: false })
    const after = settingsSignature({ viewBackgroundColor: '#ffffff', gridModeEnabled: true })
    expect(before).not.toBe(after)
  })
})
