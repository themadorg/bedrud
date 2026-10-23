/**
 * Notification sounds for meetings using the Web Audio API.
 * The chimes are generated on the fly; the only files played are peers' welcome sounds.
 */

import { WELCOME_SOUND_PLAYBACK_GAIN } from '#/lib/welcome-sound'

let ctx: AudioContext | null = null

function getCtx(): AudioContext {
  if (!ctx || ctx.state === 'closed') ctx = new AudioContext()
  // Resume on first interaction (browser autoplay policy)
  if (ctx.state === 'suspended') ctx.resume()
  return ctx
}

function tone(freq: number, duration: number, type: OscillatorType = 'sine', gain = 0.12, rampDown = 0.06) {
  const ac = getCtx()
  const osc = ac.createOscillator()
  const vol = ac.createGain()
  osc.type = type
  osc.frequency.value = freq
  vol.gain.value = gain
  // Fade out to avoid click
  vol.gain.setValueAtTime(gain, ac.currentTime + duration - rampDown)
  vol.gain.linearRampToValueAtTime(0, ac.currentTime + duration)
  osc.connect(vol).connect(ac.destination)
  osc.start()
  osc.stop(ac.currentTime + duration)
}

/** Two-note rising chime — someone joined */
export function playJoin() {
  tone(660, 0.12, 'sine', 0.09)
  setTimeout(() => tone(880, 0.15, 'sine', 0.09), 100)
}

/** Single descending note — someone left */
export function playLeave() {
  const ac = getCtx()
  const osc = ac.createOscillator()
  const vol = ac.createGain()
  osc.type = 'sine'
  osc.frequency.setValueAtTime(660, ac.currentTime)
  osc.frequency.linearRampToValueAtTime(440, ac.currentTime + 0.18)
  vol.gain.value = 0.09
  vol.gain.setValueAtTime(0.09, ac.currentTime + 0.12)
  vol.gain.linearRampToValueAtTime(0, ac.currentTime + 0.2)
  osc.connect(vol).connect(ac.destination)
  osc.start()
  osc.stop(ac.currentTime + 0.22)
}

/** Soft pop — new chat message */
export function playChat() {
  tone(1200, 0.07, 'sine', 0.07)
  setTimeout(() => tone(1500, 0.06, 'sine', 0.05), 55)
}

/** Short buzz — you tried to speak while muted */
export function playMutedBeep() {
  tone(340, 0.09, 'square', 0.06)
  setTimeout(() => tone(340, 0.09, 'square', 0.06), 130)
}

// Decoded clips by URL; the `?v=` suffix changes on every upload, so a replaced clip is fetched again.
const welcomeBuffers = new Map<string, Promise<AudioBuffer>>()

function loadWelcomeBuffer(ac: AudioContext, url: string): Promise<AudioBuffer> {
  const cached = welcomeBuffers.get(url)
  if (cached) return cached
  const loading = fetch(url)
    .then((response) => {
      if (!response.ok) throw new Error(`welcome sound fetch failed: ${response.status}`)
      return response.arrayBuffer()
    })
    .then((data) => ac.decodeAudioData(data))
  welcomeBuffers.set(url, loading)
  loading.catch(() => welcomeBuffers.delete(url))
  return loading
}

/**
 * Plays a peer's welcome sound at the fixed welcome gain. Resolves when the clip ends and
 * rejects if it cannot be fetched or decoded, so the caller can fall back to the chime.
 */
export async function playWelcome(url: string): Promise<void> {
  const ac = getCtx()
  const buffer = await loadWelcomeBuffer(ac, url)
  await new Promise<void>((resolve) => {
    const source = ac.createBufferSource()
    const vol = ac.createGain()
    source.buffer = buffer
    vol.gain.value = WELCOME_SOUND_PLAYBACK_GAIN
    source.connect(vol).connect(ac.destination)
    source.onended = () => resolve()
    source.start()
  })
}

/** Close the shared AudioContext. Call on meeting unmount to release resources (esp. mobile). */
export async function closeCtx() {
  if (ctx) {
    try {
      await ctx.close()
    } catch {
      // ignore close errors
    }
    ctx = null
  }
}
