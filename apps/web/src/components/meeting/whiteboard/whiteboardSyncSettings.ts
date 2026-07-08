import type { AppState } from '@excalidraw/excalidraw/types'

/** Canvas-level settings shared across all whiteboard participants. */
export type WhiteboardSyncSettings = Pick<AppState, 'viewBackgroundColor' | 'gridModeEnabled'>

const SYNCABLE_SETTINGS_KEYS = [
  'viewBackgroundColor',
  'gridModeEnabled',
] as const satisfies readonly (keyof WhiteboardSyncSettings)[]

export function pickSyncableSettings(appState: AppState): WhiteboardSyncSettings {
  return {
    viewBackgroundColor: appState.viewBackgroundColor,
    gridModeEnabled: appState.gridModeEnabled,
  }
}

export function settingsSignature(settings: Partial<WhiteboardSyncSettings>): string {
  return SYNCABLE_SETTINGS_KEYS.map((key) => `${key}:${settings[key] ?? ''}`).join('|')
}

export const WHITEBOARD_SYNC_SETTINGS_KEYS = SYNCABLE_SETTINGS_KEYS
