---
name: bedrud-fe-state
description: Frontend state management — all 7 Zustand 5 stores.
license: Apache License
---

# Bedrud Frontend Stores

React 19 SPA. `apps/web/`. Zustand 5.

---

## `src/lib/auth.store.ts` — `useAuthStore`

```
State: { tokens: AuthTokens | null, initialized: boolean }
AuthTokens: { accessToken: string, refreshToken: string | null }
```

| Action | Purpose |
|--------|---------|
| `setTokens(tokens, remember?)` | Store: localStorage (remember=true), sessionStorage (false), 'ephemeral' → sessionStorage |
| `updateAccessToken(accessToken)` | Update access token only |
| `clear()` | Clear tokens + storage keys |
| `initialize()` | Restore session: cookie-based refresh → fallback persisted access validated via `GET /api/auth/me`. Deduplicates concurrent calls |

Storage keys: `auth_remember`, `auth_at`.

---

## `src/lib/user.store.ts` — `useUserStore`

```
State: { user: User | null }
User: { id, email, name, provider, isAdmin, accesses: string[] | null, avatarUrl? }
```

Actions: `setUser(user)`, `clear()`.

---

## `src/lib/theme.store.ts` — `useThemeStore`

```
State: { theme: Theme }  // persisted via zustand/middleware/persist, key "theme"
Theme: 'light' | 'dark' | 'system'
```

Actions: `setTheme(theme)` — update store + apply DOM `dark` class.

Helpers: `resolveTheme(theme)` — resolve 'system' → 'light'|'dark'. `applyTheme(theme)` — toggle `<html>` class.

---

## `src/lib/audio-preferences.store.ts` — `useAudioPreferencesStore`

```
State: AudioPreferences  // persisted, key "audio-preferences"
AudioPreferences: {
  noiseSuppressionMode: NoiseSuppressionMode  // 'none'|'browser'|'rnnoise'|'krisp'
  echoCancellation: boolean   // default true
  autoGainControl: boolean    // default true
  inputGain: number           // 0-300, default 100
  noiseGate: number           // 0-100, default 0
  mutedBeepEnabled: boolean   // default true
  mutedBeepInterval: number   // ms, default 3000
}
```

Actions: `setMode`, `setEchoCancellation`, `setAutoGainControl`, `setInputGain` (clamped), `setNoiseGate` (clamped), `setMutedBeepEnabled`, `setMutedBeepInterval`, `merge(partial)`.

---

## `src/lib/recent-rooms.store.ts` — `useRecentRoomsStore`

```
State: { rooms: RecentRoom[] }  // persisted, key "bedrud-recent-rooms"
RecentRoom: { name: string, joinedAt: number }
```

Actions: `add(name)` — prepend, dedup, cap 20. `remove(name)`, `clear()`.

---

## `src/lib/video-preferences.store.ts` — `useVideoPreferencesStore`

```
State: VideoPreferences  // persisted, key "video-preferences"
VideoPreferences: { mirrorWebcam: boolean }  // default true
```

Actions: `setMirrorWebcam(enabled)`, `merge(partial)`.

---

## `src/lib/participant-overrides.store.ts` — `useParticipantOverridesStore`

```
State: { volumes: Map<string, number>, muted: Set<string> }  // NOT persisted
```

Actions: `setVolume(identity, vol)` (clamped 0-2), `toggleMute(identity)`.
Selectors: `selectIsMuted(identity)` → bool, `selectVolume(identity)` → number (0 if muted, default 1).
