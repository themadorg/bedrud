# Welcome sound on room join — design

Tracks themadorg/bedrud#51. This is a design proposal, nothing here is built yet.
Each decision lists one recommendation and the alternatives that were rejected.

## What exists today

| Area | Current code | What it gives us |
|------|--------------|------------------|
| Avatar upload | `POST /api/auth/me/avatar` → `AuthHandler.UploadAvatar` → `storage.SaveUserAvatar` | Size check, MIME sniff, write to `./data/uploads/avatars/<userID>.<ext>`, return `/uploads/avatars/<userID>.<ext>?v=<unix>`, stored in `User.AvatarURL` |
| Avatar serving | `app.Get("/uploads/avatars/*")` in `server/internal/server/server.go`, guarded by `storage.ResolveAvatarFile` | Public static file route with path-traversal checks |
| Profile lookup in a meeting | `GET /api/room/:roomId/participant/:identity/profile` → `RoomHandler.GetParticipantProfile` | Returns `{id, name, avatarUrl}`. Caller and target must both be in the LiveKit room. Guests get an empty `avatarUrl`. |
| Join signalling | `MeetingProvider` in `apps/web/src/components/meeting/MeetingContext.tsx` already calls `fetchMeetingParticipantProfile` on every `RoomEvent.ParticipantConnected` | Every peer already does one authenticated HTTP fetch per join |
| Join chime | `MeetingSoundEffects.tsx` calls `playJoin()` from `apps/web/src/lib/meeting-sounds.ts` (Web Audio oscillator, gain 0.09) on `ParticipantConnected`. It is skipped for people already present when you join. | Shared `AudioContext`, autoplay already unlocked by the join click |
| Listener prefs | `useAudioPreferencesStore` (zustand persist, `audio-preferences` key), synced through `patchUserPreferences` (`/api/auth/preferences`, `audio` blob) | A place for a listener toggle that follows the user across devices |
| Admin settings | `models.SystemSettings` (for example `RNNoiseEnabled`, `KrispEnabled`), `SettingsRepository.GetEffectiveSettings` + `mergeFromConfig`, `GET /api/auth/settings` (public), `AudioTab` in `apps/web/src/components/admin/settings/audio-tab.tsx` | Kill-switch pattern and config-default-plus-admin-override pattern |
| Rate limits | `middleware.APIRateLimiter` and friends (IP-keyed, per minute) | Burst protection only; nothing per user per day |
| Audio decoding in Go | None. `server/go.mod` has no audio decoder; the pion modules are indirect, from LiveKit | Any decoder is a new dependency |

There is no listener-side way to silence the current join chime.

## Decisions

### 1. Clip limits and format

**Recommendation: MP3 only, decoded on the server with `github.com/hajimehoshi/go-mp3` (pure Go, no cgo).
Defaults: 7 s max duration, 32 KiB max size, mono or stereo at any sample rate the decoder accepts.
Validate by decoding the whole file once; that pass gives the duration from the sample count and the PCM for the loudness checks below. Reject; never transcode.**

- MP3 is the only compressed format that has a mature pure-Go decoder *and* plays natively in every browser, on Android, on iOS and in the Rust desktop app. Keeping the binary single and cross-compilable (`tonistiigi/xx`) rules out cgo.
- The issue's 20 KB default only fits 7 s at about 23 kbps, which sounds poor in MP3. 32 KiB gives about 37 kbps for 7 s, which is fine for a jingle. Both stay configurable, so the default is the only thing in question.
- Duration comes from the decoded sample count, not from header tags, which can lie.
- Decoding is bounded: the size cap is applied before decoding (as the avatar handler already does with `io.LimitReader`), and the decoder stops after `maxDuration + 1 s` of samples.

Rejected: Ogg/Opus (no complete pure-Go decoder; libopus needs cgo) · Ogg/Vorbis (Safari support is unreliable) · WAV (does not fit any sane byte cap) · `ffmpeg` subprocess transcoding (an external runtime dependency breaks the single-binary install) · client-side validation only (bypassable, and the issue rules it out).

### 2. Where the clip lives and how clients fetch it

**Recommendation: mirror the avatar exactly.
Store the clip at `./data/uploads/welcome-sounds/<userID>.mp3`, serve it from a new `GET /uploads/welcome-sounds/*` route (same `Resolve…File` traversal guard), keep the versioned URL `/uploads/welcome-sounds/<userID>.mp3?v=<unix>` in a new `User.WelcomeSoundURL` column, and add `welcomeSoundUrl` to the `GetParticipantProfile` response.**

- Peers already call the profile endpoint on every join, so playback needs no new signalling and no new request.
- The server decides what goes out: when the feature is off, or the target is a guest, `welcomeSoundUrl` is empty. The kill switch is then enforced server-side even against stale clients.
- The `?v=` suffix lets clients cache the decoded buffer per URL and drop it on replace.
- Account deletion (`queue/handler_user_delete.go`) and the "remove sound" endpoint delete the file, as `DeleteUserAvatarFiles` does for avatars.

Rejected: LiveKit participant metadata (the client writes its own metadata through `patchLocalParticipantMetadata`, so anyone could point the whole room at an arbitrary URL) · the joiner broadcasting the clip or URL over the data channel (same spoofing problem, plus payload size) · the chat upload store and S3 (built for per-room attachments and room-scoped cleanup; one small file per user does not need it) · inline `data:` URIs in the profile response (bloats every profile fetch).

### 3. Playback trigger, de-duplication and rate limiting

**Recommendation: each listener plays the joiner's sound locally, through the existing shared `AudioContext` in `meeting-sounds.ts`, *instead of* the generic chime for that join.
Triggering lives in `MeetingSoundEffects`. When `fetchMeetingParticipantProfile` resolves with a `welcomeSoundUrl`, the listener plays it; otherwise it plays the chime.**

Guards, all client-side and in memory:

- **Initial mount:** keep the existing rule. Nobody hears sounds for people who were already present when they joined.
- **Reconnects:** a per-identity cooldown map (`identity → lastPlayedAt`) held in a ref that outlives LiveKit reconnects. Default cooldown 10 minutes. A peer who flaps or reloads gets the plain chime, not their clip again.
- **Bursts:** at most one welcome sound playing at a time. A join that arrives while one is playing gets the chime. This covers meeting starts, where many people join at once.
- **Deafened listeners** hear nothing. Today `playJoin` ignores deafen; the new code checks `isSelfDeafened || isServerDeafened` for both sounds.
- **The joiner** does not hear their own clip in the room. The profile panel has a preview button instead.
- **Late profile:** if the profile fetch takes longer than about 1.5 s, the listener plays the chime and drops the clip, so sounds never land seconds after the tile appears.

Server-side change limit: **at most 2 uploads per UTC calendar day per user**, tracked with two new `User` columns (`WelcomeSoundChangeDay` as a `YYYY-MM-DD` string and `WelcomeSoundChangeCount`). Clearing the sound is always allowed and does not count. The upload route also sits behind the existing `APIRateLimiter` for burst protection.

Rejected: playing through a published LiveKit audio track (fights echo cancellation, ends up in recordings, and the issue rules it out) · server-sent "play now" events (needs a webhook-to-data-channel path for no gain, since peers already observe joins) · a separate rate-limit table (two columns cover "2 per day" for one row per user).

### 4. Mute controls

**Decision for v1: a listener preference, a room toggle for the host, and the admin kill switch.**

- **Listener preference:** a new `joinSoundMode: 'welcome' | 'chime' | 'off'` in `useAudioPreferencesStore`, default `'welcome'`, synced through the existing `audio` preferences blob. This also adds the missing way to silence the current chime. It sits in the meeting settings audio section and the dashboard settings.
- **Admin kill switch:** `SystemSettings.WelcomeSoundEnabled` (default `false`, opt-in as the issue asks), exposed in `GET /api/auth/settings` as `welcomeSound: {enabled, maxBytes, maxDurationSeconds}`. When it is off, uploads return 403, the profile hides the URL, and the profile panel hides the section. The toggle lives in a "Welcome sounds" section of the admin Audio tab, next to RNNoise and Krisp.
- **Limits from config:** `welcomeSound.maxBytes`, `welcomeSound.maxDurationSeconds`, `welcomeSound.maxPeakDbfs` and `welcomeSound.maxLoudnessLufs` in `config.yaml` (with env overrides), merged through `mergeFromConfig`'s existing "0 means use the config value" rule and overridable in the admin panel.
- **Enable flag:** it lives only in `SystemSettings`, like RNNoise and Krisp. A config default for a bool would need a `*bool` to tell "unset" from "false". If operators want one anyway, it is a small add-on.
- **Host toggle:** a new `RoomSettings.AllowWelcomeSounds` (default `true`), set by the room owner in the room settings. When it is off, `GetParticipantProfile` returns an empty `welcomeSoundUrl` for that room, so it is enforced server-side like the kill switch. It touches room create and update, `@bedrud/api-types`, and the web room settings UI. Native clients only need to tolerate the new field until their parity issues land.

Rejected: a per-listener volume slider (a three-way mode plus a fixed, quiet playback gain covers the need with less UI) · a per-speaker mute in the participant context menu (nice later, not needed for safety).

### 5. Abuse and safety

**Recommendation: reject on upload, then play quietly.**

On the decoded PCM, in the same pass as the duration check:

| Check | Metric | Default | Error the user sees |
|-------|--------|---------|---------------------|
| Peak | Sample peak across all channels | reject if ≥ −1 dBFS | "This sound peaks too high. Lower the volume and export again." |
| Loudness | ITU-R BS.1770 integrated loudness (K-weighting + gating, about 100 lines of Go) | reject if > −20 LUFS | "This sound is too loud. Lower the volume and export again." |
| Silence | Integrated loudness | reject if < −50 LUFS | "This sound is silent." |

- Sample peak is used rather than true peak because true peak needs 4× oversampling. The −1 dBFS ceiling plus the quiet playback gain covers the inter-sample gap. True peak can follow if needed.
- **Playback gain:** clients play welcome sounds through a fixed gain node (about 0.5, tuned so a −20 LUFS clip lands near the loudness of the current chime) and never boost.
- **Moderation:** a superadmin can clear any user's sound (`DELETE /api/admin/users/:id/welcome-sound`) from the admin users table. This is the abuse lever for offensive clips; loudness checks cannot judge content.
- **Guests** cannot upload (the route sits behind `RejectGuest`) and never have a URL.
- **Fixtures:** tests generate PCM synthetically (a loud full-scale square wave, a quiet sine, silence) and encode them once into committed MP3 fixtures under `server/internal/storage/testdata/`.

Rejected: auto-normalising on upload (needs an MP3 encoder in Go; no good pure-Go one exists, and the issue prefers rejection) · storing a measured gain for clients to apply (a good later step, but not needed while uploads above −20 LUFS are rejected) · crest-factor checks (the issue lists them as optional; peak plus loudness cover the ear-blast case).

### 6. Mobile and desktop clients

**Recommendation: web only in v1. File one tracked parity issue per client (Android, iOS, desktop).**

- Mobile and desktop users who uploaded a sound on the web are still heard by web peers, because the sound hangs off the user profile, not the client.
- Native clients only need to read `welcomeSoundUrl` from the profile endpoint and play it under the same guards. MP3 plays natively on all three. Upload UI on native clients can wait.

Rejected: shipping all four clients at once (four review surfaces for an opt-in feature that ships disabled).

## Suggested units

Each unit merges on its own and leaves the feature dark until unit 4.

1. **Server: settings and config.** `WelcomeSoundConfig` in `server/config/config.go` with env overrides, `SystemSettings` fields, `mergeFromConfig`, admin GET/PUT field handling and validation, and the public settings block. Tests: merge, admin override, defaults.
2. **Server: audio validation.** `storage/welcome_sound.go` with an MP3 decode pass, duration, sample peak, BS.1770 loudness and save/delete/resolve helpers mirroring `avatar.go`, plus the generated fixtures. Adds `go-mp3`. Tests: oversize, over-duration, loud, peaky, silent and valid.
3. **Server: API.** `User` columns, `POST` and `DELETE /api/auth/me/welcome-sound` (feature gate, `RejectGuest`, 2-per-day limit), the `/uploads/welcome-sounds/*` route, `welcomeSoundUrl` in `GetParticipantProfile` and `/api/auth/me`, cleanup on user delete, the admin clear endpoint, and Swagger regeneration.
4. **Web: admin and profile.** A "Welcome sounds" section in `AudioTab` and `TAB_FIELDS`. An upload, preview and remove section in `ProfileSettingsPanel`, shown only when enabled, with the effective limits and the server's error text. A "Clear welcome sound" action in the admin users table.
5. **Web: playback.** `playWelcome(url)` in `meeting-sounds.ts` (fetch, `decodeAudioData`, cache by URL, fixed gain). The trigger, cooldown, single-at-a-time and late-profile guards in `MeetingSoundEffects`. The deafen check for both sounds. The `joinSoundMode` pref, its UI and its sync. Unit tests for the guard logic as a pure function.
6. **Room toggle.** `RoomSettings.AllowWelcomeSounds` in the model, room create and update, `@bedrud/api-types` and the profile handler check, plus the switch in the web room settings. Tests: the profile handler hides the URL when the room has it off.
7. **Follow-ups (issues only):** Android, iOS and desktop playback parity, and measured playback gain or true-peak checks.

Docs ship with units 3 to 5: the admin and user guides under `docs/guide` and the `apps/site` docs page for the setting.

## Decisions taken

- **Default size:** 32 KiB.
- **Format:** MP3 only.
- **Room toggle:** in v1 (unit 6).
- **Playback gain:** 0.5, the starting value on the listening demo, accepted as is. It can be tuned in unit 5.
