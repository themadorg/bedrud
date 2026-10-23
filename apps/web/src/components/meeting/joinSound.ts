import type { JoinSoundMode } from '#/lib/audio-preferences.store'

/** A peer who rejoins within this window gets the chime, not their clip again. */
export const WELCOME_SOUND_COOLDOWN_MS = 10 * 60 * 1000

/** A clip that would start later than this after the join is dropped for the chime. */
export const WELCOME_SOUND_LATE_MS = 1500

export type JoinSound = 'welcome' | 'chime' | 'none'

export interface JoinSoundInput {
  /** The listener's own choice in audio settings. */
  mode: JoinSoundMode
  deafened: boolean
  /** The admin switch, from public settings. */
  welcomeSoundsEnabled: boolean
  /** The joiner's clip, from their participant profile; empty when they have none. */
  welcomeSoundUrl?: string
  /** When this listener last played this joiner's clip. */
  lastPlayedAt?: number
  /** Another joiner's clip is still playing. */
  welcomePlaying: boolean
  /** Time from the join event until the profile arrived. */
  waitedMs: number
  now: number
}

/** Decides what a listener hears when someone joins. */
export function chooseJoinSound(input: JoinSoundInput): JoinSound {
  if (input.mode === 'off' || input.deafened) return 'none'
  if (input.mode === 'chime' || !input.welcomeSoundsEnabled || !input.welcomeSoundUrl) return 'chime'
  if (input.waitedMs > WELCOME_SOUND_LATE_MS || input.welcomePlaying) return 'chime'
  if (input.lastPlayedAt !== undefined && input.now - input.lastPlayedAt < WELCOME_SOUND_COOLDOWN_MS) return 'chime'
  return 'welcome'
}
