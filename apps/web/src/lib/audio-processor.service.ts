import type { LocalAudioTrack } from 'livekit-client'
import type { NoiseSuppressionMode } from '#/lib/audio-preferences.store'

/**
 * Manages the lifecycle of a LiveKit audio processor (Krisp or RNNoise).
 *
 * Used as a module-level singleton so both ControlsBar (quick toggle) and
 * AudioProcessorManager (meeting room) share the same processor state.
 *
 * Heavy SDKs are only dynamic-imported when the instance admin has enabled
 * them AND the active mode matches:
 * - RNNoise (`#/lib/rnnoise-processor` + WASM) → `setRNNoiseAllowed(true)` + mode `rnnoise`
 * - Krisp (`@livekit/krisp-noise-filter`) → `setKrispAllowed(true)` + mode `krisp`
 * Otherwise those packages are never fetched.
 */
export class AudioProcessorService {
  private track: LocalAudioTrack | null = null
  private currentMode: NoiseSuppressionMode = 'none'
  /** Instance-level gates from public settings (admin System → Audio). */
  private rnnoiseInstanceAllowed = false
  private krispInstanceAllowed = false

  setRNNoiseAllowed(allowed: boolean): void {
    this.rnnoiseInstanceAllowed = allowed
  }

  isRNNoiseAllowed(): boolean {
    return this.rnnoiseInstanceAllowed
  }

  setKrispAllowed(allowed: boolean): void {
    this.krispInstanceAllowed = allowed
  }

  isKrispAllowed(): boolean {
    return this.krispInstanceAllowed
  }

  /** Apply both instance gates (from public settings). */
  setNoisePackageAllowed(opts: { rnnoise?: boolean; krisp?: boolean }): void {
    if (opts.rnnoise !== undefined) this.rnnoiseInstanceAllowed = opts.rnnoise
    if (opts.krisp !== undefined) this.krispInstanceAllowed = opts.krisp
  }

  /** Attach to a track and apply the given mode. Called on room connect. */
  async attach(track: LocalAudioTrack, mode: NoiseSuppressionMode): Promise<void> {
    this.track = track
    this.currentMode = 'none'
    await this.switchMode(mode)
  }

  /**
   * Switch to a new noise suppression mode.
   * Tears down any existing processor first to avoid double-processing.
   */
  async switchMode(
    mode: NoiseSuppressionMode,
    opts?: { echoCancellation?: boolean; autoGainControl?: boolean },
  ): Promise<void> {
    if (!this.track) return

    // Never load heavy packages unless instance admin enabled them.
    let effective: NoiseSuppressionMode = mode
    if (mode === 'rnnoise' && !this.rnnoiseInstanceAllowed) effective = 'browser'
    if (mode === 'krisp' && !this.krispInstanceAllowed) effective = 'browser'

    const modeChanged = effective !== this.currentMode

    if (modeChanged && this.currentMode !== 'none' && this.currentMode !== 'browser') {
      try {
        await this.track.stopProcessor()
      } catch (err) {
        if (import.meta.env.DEV) console.warn('[AudioProcessorService] stopProcessor failed:', err)
      }
    }

    this.currentMode = effective

    const mediaTrack = this.track.mediaStreamTrack
    if (mediaTrack) {
      const browserNS = effective === 'browser'
      mediaTrack
        .applyConstraints({
          noiseSuppression: browserNS,
          echoCancellation: opts?.echoCancellation ?? true,
          autoGainControl: opts?.autoGainControl ?? effective === 'browser',
        })
        .catch((err) => {
          if (import.meta.env.DEV) console.warn('[AudioProcessorService] applyConstraints failed:', err)
        })
    }

    if (modeChanged) {
      if (effective === 'rnnoise') {
        // Only when rnnoiseInstanceAllowed. Dynamic import → no download when disabled.
        const { RNNoiseProcessor } = await import('#/lib/rnnoise-processor')
        await this.track.setProcessor(new RNNoiseProcessor())
      } else if (effective === 'krisp') {
        // Only when krispInstanceAllowed. Dynamic import → no download when disabled.
        const { KrispNoiseFilter } = await import('@livekit/krisp-noise-filter')
        await this.track.setProcessor(KrispNoiseFilter())
      }
    }
  }

  async setEchoCancellation(enabled: boolean): Promise<void> {
    if (!this.track) return
    const mediaTrack = this.track.mediaStreamTrack
    if (mediaTrack) {
      mediaTrack.applyConstraints({ echoCancellation: enabled }).catch((err) => {
        if (import.meta.env.DEV) console.warn('[AudioProcessorService] setEchoCancellation failed:', err)
      })
    }
  }

  async detach(): Promise<void> {
    if (this.track && this.currentMode !== 'none' && this.currentMode !== 'browser') {
      try {
        await this.track.stopProcessor()
      } catch (err) {
        if (import.meta.env.DEV) console.warn('[AudioProcessorService] detach stopProcessor failed:', err)
      }
    }
    this.track = null
    this.currentMode = 'none'
  }

  static isKrispSupported(): boolean {
    if (typeof window === 'undefined') return false
    const isFirefox = navigator.userAgent.toLowerCase().includes('firefox')
    const hasAudioWorklet = typeof AudioWorklet !== 'undefined'
    return !isFirefox && hasAudioWorklet && window.isSecureContext
  }
}

export const audioProcessorService = new AudioProcessorService()
