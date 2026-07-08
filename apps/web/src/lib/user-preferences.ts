import { api } from '#/lib/api'

export type UserPreferencesBlob = {
  audio?: object
  video?: object
  experimental?: object
  interface?: object
}

export function parseUserPreferences(json: string | undefined | null): UserPreferencesBlob {
  if (!json) return {}
  try {
    const parsed = JSON.parse(json) as unknown
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
      ? (parsed as UserPreferencesBlob)
      : {}
  } catch {
    return {}
  }
}

/** Deep-merge a patch into the user's stored preferences blob. */
export async function patchUserPreferences(patch: UserPreferencesBlob): Promise<void> {
  const res = await api.get<{ preferencesJson: string }>('/api/auth/preferences')
  const existing = parseUserPreferences(res.preferencesJson)
  const merged: UserPreferencesBlob = { ...existing }

  if (patch.audio) {
    merged.audio = { ...(existing.audio ?? {}), ...patch.audio }
  }
  if (patch.video) {
    merged.video = { ...(existing.video ?? {}), ...patch.video }
  }
  if (patch.experimental) {
    merged.experimental = { ...(existing.experimental ?? {}), ...patch.experimental }
  }
  if (patch.interface) {
    merged.interface = { ...(existing.interface ?? {}), ...patch.interface }
  }

  await api.put('/api/auth/preferences', { preferencesJson: JSON.stringify(merged) })
}
