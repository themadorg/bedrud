import { useLocalParticipant, useRoomContext } from '@livekit/components-react'
import { RoomEvent } from 'livekit-client'
import { useEffect, useRef } from 'react'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { closeCtx, playChat, playJoin, playLeave, playMutedBeep } from '#/lib/meeting-sounds'
import { captureLevel, SAMPLE_INTERVAL_MS, SpeechLevelTracker } from '#/lib/speech-level-tracker'
import { useMeetingChatContext, useMeetingRoomContext } from './MeetingContext'

/**
 * Invisible component that plays notification sounds for meeting events:
 * - Participant joins / leaves
 * - New chat message (from others)
 * - Talking while muted (monitors mic input independently)
 */
export function MeetingSoundEffects() {
  const room = useRoomContext()
  const { chatMessages } = useMeetingChatContext()

  // Close shared AudioContext on unmount (prevents mobile audio issues from long-lived context)
  useEffect(() => {
    return () => {
      closeCtx().catch(() => {})
    }
  }, [])

  // ── Join / Leave sounds ──────────────────────────────────────
  // Use RoomEvent directly for accurate join/leave detection rather
  // than diffing the participants array (which can change for other reasons).
  const mountedRef = useRef(false)

  useEffect(() => {
    // Skip sounds during initial mount — we don't want a blast of join sounds
    // for everyone already in the room.
    if (!mountedRef.current) {
      mountedRef.current = true
      return
    }
  }, [])

  useEffect(() => {
    const onConnect = () => {
      if (mountedRef.current) playJoin()
    }
    const onDisconnect = () => {
      if (mountedRef.current) playLeave()
    }

    room.on(RoomEvent.ParticipantConnected, onConnect)
    room.on(RoomEvent.ParticipantDisconnected, onDisconnect)
    return () => {
      room.off(RoomEvent.ParticipantConnected, onConnect)
      room.off(RoomEvent.ParticipantDisconnected, onDisconnect)
    }
  }, [room])

  // ── Chat message sound ───────────────────────────────────────
  const chatLenRef = useRef(chatMessages.length)

  useEffect(() => {
    const prev = chatLenRef.current
    chatLenRef.current = chatMessages.length
    if (chatMessages.length <= prev) return

    // Only play sound for messages from others
    const latest = chatMessages[chatMessages.length - 1]
    if (!latest?.isLocal) {
      playChat()
    }
  }, [chatMessages])

  // ── Muted-mic detection ──────────────────────────────────────
  // When the user is muted, open a separate mic stream and watch for
  // speech. If detected, play a short beep so they know they're muted.
  useMutedMicMonitor()

  return null
}

// ── Hook: monitor mic input while muted ────────────────────────

function useMutedMicMonitor() {
  const { isSelfDeafened } = useMeetingRoomContext()
  // useLocalParticipant is a reactive hook — re-renders when mic state changes.
  // room.localParticipant.isMicrophoneEnabled is a plain property that does NOT
  // trigger React re-renders, which caused the monitor to stay active after unmuting.
  const { isMicrophoneEnabled } = useLocalParticipant()
  const pushToTalkEnabled = useAudioPreferencesStore((s) => s.pushToTalkEnabled)
  const mutedBeepEnabled = useAudioPreferencesStore((s) => s.mutedBeepEnabled)
  const mutedBeepInterval = useAudioPreferencesStore((s) => s.mutedBeepInterval)
  // Use refs so the tick loop always reads the latest values without restarting
  const beepEnabledRef = useRef(mutedBeepEnabled)
  const beepIntervalRef = useRef(mutedBeepInterval)
  useEffect(() => {
    beepEnabledRef.current = mutedBeepEnabled
  }, [mutedBeepEnabled])
  useEffect(() => {
    beepIntervalRef.current = mutedBeepInterval
  }, [mutedBeepInterval])

  useEffect(() => {
    // Only monitor when mic is disabled and user is NOT deafened
    // (deafened users intentionally silenced everything — don't nag them).
    if (isMicrophoneEnabled || isSelfDeafened || pushToTalkEnabled) return

    let cancelled = false
    let stream: MediaStream | null = null
    let sampleTimer: ReturnType<typeof setInterval> | undefined
    let lastBeepAt = 0
    let ac: AudioContext | null = null

    async function start() {
      try {
        stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      } catch {
        return // no mic permission — nothing we can do
      }
      if (cancelled) {
        stream.getTracks().forEach((t) => {
          t.stop()
        })
        return
      }

      ac = new AudioContext()
      const source = ac.createMediaStreamSource(stream)
      const analyser = ac.createAnalyser()
      analyser.fftSize = 512
      source.connect(analyser)
      const data = new Float32Array(analyser.fftSize)
      // Speech is judged against this microphone's own noise floor; a fixed bar missed quiet mics and fired on hot ones.
      const tracker = new SpeechLevelTracker()

      function sample() {
        analyser.getFloatTimeDomainData(data)
        let sum = 0
        for (let i = 0; i < data.length; i++) sum += data[i] * data[i]
        const isSpeech = tracker.isSpeech(captureLevel(Math.sqrt(sum / data.length)))

        if (beepEnabledRef.current && isSpeech && Date.now() - lastBeepAt > beepIntervalRef.current) {
          lastBeepAt = Date.now()
          playMutedBeep()
        }
      }
      sampleTimer = setInterval(sample, SAMPLE_INTERVAL_MS)
    }

    start()
    return () => {
      cancelled = true
      clearInterval(sampleTimer)
      stream?.getTracks().forEach((t) => {
        t.stop()
      })
      ac?.close()
    }
  }, [isMicrophoneEnabled, isSelfDeafened, pushToTalkEnabled])
}
