---
name: bedrud-fe-state
description: Frontend state management — all 10 Zustand 5 stores.
license: Apache License
---

# Bedrud Frontend Stores

React SPA. `apps/web/`. Zustand 5. All stores live in `apps/web/src/lib/*.store.ts`.

Related (not stores): `user-preferences.ts` deep-merges server-side prefs blobs (`audio` / `video` / `experimental` / `interface`) via `PATCH`-style put to `/api/auth/preferences`.

---

## Index

| File | Hook | Persist key | Notes |
|------|------|-------------|-------|
| `auth.store.ts` | `useAuthStore` | manual: `auth_remember`, `auth_at` | Not zustand/persist |
| `user.store.ts` | `useUserStore` | — | In-memory only |
| `theme.store.ts` | `useThemeStore` | `theme` | DOM `dark` class |
| `audio-preferences.store.ts` | `useAudioPreferencesStore` | `audio-preferences` | PTT + NS + gain |
| `video-preferences.store.ts` | `useVideoPreferencesStore` | `video-preferences` | Webcam mirror |
| `experimental-preferences.store.ts` | `useExperimentalPreferencesStore` | `experimental-preferences` | Whiteboard / YouTube |
| `interface-preferences.store.ts` | `useInterfacePreferencesStore` | `interface-preferences` | Welcome screen |
| `recent-rooms.store.ts` | `useRecentRoomsStore` | `bedrud-recent-rooms` | Cap 20, dedupe on rehydrate |
| `participant-overrides.store.ts` | `useParticipantOverridesStore` | — | Map/Set, session only |
| `profile-sync.store.ts` | `useProfileSyncStore` | — | Version bump signal |

---

## `src/lib/auth.store.ts` — `useAuthStore`

```
State: { tokens: AuthTokens | null, initialized: boolean }
AuthTokens: { accessToken: string, refreshToken: string | null }
```

| Action | Purpose |
|--------|---------|
| `setTokens(tokens, remember?)` | Store tokens. `remember=true` → localStorage; `false` or `'ephemeral'` → sessionStorage |
| `updateAccessToken(accessToken)` | Update access token in state + same storage as remember flag |
| `clear()` | Clear tokens, reset `initialized`, wipe both storages' keys, reset init promise |
| `initialize()` | Restore session: cookie `POST /api/auth/refresh` first → fallback persisted AT validated via `GET /api/auth/me`. Deduplicates concurrent calls via module `_init.promise` |

Storage keys: `auth_remember`, `auth_at` (localStorage and/or sessionStorage). Not `zustand/persist`.

---

## `src/lib/user.store.ts` — `useUserStore`

```
State: { user: User | null }
User: {
  id, email, name, provider: string
  isSuperAdmin: boolean
  isAdmin: boolean
  accesses: string[] | null
  avatarUrl?: string
}
```

Actions: `setUser(user)`, `clear()`. Not persisted.

---

## `src/lib/theme.store.ts` — `useThemeStore`

```
State: { theme: Theme }  // default 'system'
Theme: 'light' | 'dark' | 'system'
```

Persist key: `theme`.

Actions: `setTheme(theme)` — update store; apply DOM `dark` class only if resolved theme differs (avoids fighting view-transitions).

Helpers (exported, not on store):
- `resolveTheme(theme)` — `'system'` → OS preference; SSR → `'light'`
- `applyTheme(theme)` — toggle `<html class="dark">`; SSR no-op

---

## `src/lib/audio-preferences.store.ts` — `useAudioPreferencesStore`

```
State: AudioPreferences  // persisted
AudioPreferences: {
  noiseSuppressionMode: NoiseSuppressionMode  // 'none'|'browser'|'rnnoise'|'krisp'; default 'browser'
  echoCancellation: boolean   // default true
  autoGainControl: boolean    // default true
  inputGain: number           // 0–300, default 100
  noiseGate: number           // 0–100, default 0
  mutedBeepEnabled: boolean   // default true
  mutedBeepInterval: number   // ms, default 3000
  pushToTalkEnabled: boolean  // default false
  pushToTalkKey: string       // KeyboardEvent.code, default 'Space' (DEFAULT_PUSH_TO_TALK_KEY)
}
```

Persist key: `audio-preferences`.

| Action | Notes |
|--------|-------|
| `setMode(mode)` | NS mode |
| `setEchoCancellation(v)` | |
| `setAutoGainControl(v)` | |
| `setInputGain(v)` | clamped 0–300 |
| `setNoiseGate(v)` | clamped 0–100 |
| `setMutedBeepEnabled(v)` | |
| `setMutedBeepInterval(v)` | |
| `setPushToTalkEnabled(v)` | |
| `setPushToTalkKey(key)` | via `normalizePushToTalkKey` |
| `merge(partial)` | field-wise; clamps gain/gate; normalizes PTT key |

---

## `src/lib/video-preferences.store.ts` — `useVideoPreferencesStore`

```
State: VideoPreferences  // persisted
VideoPreferences: { mirrorWebcam: boolean }  // default true
```

Persist key: `video-preferences`.

Actions: `setMirrorWebcam(enabled)`, `merge(partial)`.

---

## `src/lib/experimental-preferences.store.ts` — `useExperimentalPreferencesStore`

```
State: ExperimentalPreferences  // persisted
ExperimentalPreferences: {
  whiteboardEnabled: boolean                   // default false
  youtubeEnabled: boolean                      // default false
  whiteboardDisclaimerAcknowledged: boolean    // default false; one-time disclaimer
}
```

Persist key: `experimental-preferences`.

| Action | Purpose |
|--------|---------|
| `setWhiteboardEnabled(enabled)` | Toggle whiteboard feature |
| `setYoutubeEnabled(enabled)` | Toggle YouTube feature |
| `acknowledgeWhiteboardDisclaimer()` | Set disclaimer acknowledged true |
| `merge(partial)` | Field-wise partial update |

---

## `src/lib/interface-preferences.store.ts` — `useInterfacePreferencesStore`

```
State: InterfacePreferences  // persisted
InterfacePreferences: { showWelcomeScreen: boolean }  // default true
```

Persist key: `interface-preferences`.

Actions: `setShowWelcomeScreen(enabled)`, `merge(partial)`.

---

## `src/lib/recent-rooms.store.ts` — `useRecentRoomsStore`

```
State: { rooms: RecentRoom[] }  // default []
RecentRoom: { name: string, joinedAt: number }
```

Persist key: `bedrud-recent-rooms`.

| Action | Purpose |
|--------|---------|
| `add(name)` | Prepend with `joinedAt: Date.now()`, dedupe by name, cap 20 |
| `remove(name)` | Filter out by name |
| `clear()` | Empty list |

Rehydrate: `onRehydrateStorage` dedupes rooms by name (first wins).

---

## `src/lib/participant-overrides.store.ts` — `useParticipantOverridesStore`

```
State: {
  volumes: Map<string, number>  // identity → 0–2
  muted: Set<string>            // muted identities
}
```

Not persisted (session/in-memory only).

| Action | Purpose |
|--------|---------|
| `setVolume(identity, vol)` | Clamped 0–2 |
| `toggleMute(identity)` | Add/remove from muted set |

Selectors (factories — use in components for granular re-renders):
- `selectIsMuted(identity)` → `boolean`
- `selectVolume(identity)` → `0` if muted, else map value or default `1`

```ts
const isMuted = useParticipantOverridesStore(selectIsMuted(identity))
const volume = useParticipantOverridesStore(selectVolume(identity))
```

---

## `src/lib/profile-sync.store.ts` — `useProfileSyncStore`

```
State: { version: number }  // default 0
```

Not persisted. Lightweight pub/sub for profile refresh.

Actions: `bump()` — `version++`. Consumers re-fetch profile when `version` changes.
