import { useRoomContext } from '@livekit/components-react'
import type { LocalParticipant } from 'livekit-client'
import { ConnectionState } from 'livekit-client'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { isRoomSignalingReady } from '#/lib/livekit-publish'
import { formatKeyboardCode, isEditableKeyboardTarget } from '#/lib/push-to-talk-key'
import { patchParticipantMetadata } from '#/lib/push-to-talk-participant'

export function useMeetingMicKeyboard(
  localParticipant: LocalParticipant | undefined,
  isSelfDeafened: boolean,
  micEnabled: boolean,
) {
  const room = useRoomContext()
  const pushToTalkEnabled = useAudioPreferencesStore((s) => s.pushToTalkEnabled)
  const pushToTalkKey = useAudioPreferencesStore((s) => s.pushToTalkKey)
  const pttActiveRef = useRef(false)
  const prevPushToTalkRef = useRef(pushToTalkEnabled)
  const [pttVisible, setPttVisible] = useState(false)
  const [userMicMuted, setUserMicMuted] = useState(false)

  const micUiEnabled = pushToTalkEnabled ? !userMicMuted && !isSelfDeafened : micEnabled
  const pttAvailable = pushToTalkEnabled && !isSelfDeafened && !userMicMuted

  const stopPtt = useCallback(() => {
    if (!pttActiveRef.current) return
    pttActiveRef.current = false
    setPttVisible(false)
    if (localParticipant && pushToTalkEnabled && !userMicMuted) {
      void localParticipant.setMicrophoneEnabled(false)
    }
  }, [localParticipant, pushToTalkEnabled, userMicMuted])

  const startPtt = useCallback(() => {
    if (!pttAvailable || !localParticipant || pttActiveRef.current) return
    pttActiveRef.current = true
    void localParticipant.setMicrophoneEnabled(true)
    setPttVisible(true)
  }, [localParticipant, pttAvailable])

  const toggleMic = useCallback(() => {
    if (!localParticipant || isSelfDeafened) return

    if (pushToTalkEnabled) {
      if (userMicMuted) {
        setUserMicMuted(false)
        pttActiveRef.current = false
        setPttVisible(false)
        void localParticipant.setMicrophoneEnabled(false)
      } else {
        setUserMicMuted(true)
        pttActiveRef.current = false
        setPttVisible(false)
        void localParticipant.setMicrophoneEnabled(false)
      }
      return
    }

    void localParticipant.setMicrophoneEnabled(!micEnabled)
  }, [isSelfDeafened, localParticipant, micEnabled, pushToTalkEnabled, userMicMuted])

  useEffect(() => {
    if (!localParticipant || room.state !== ConnectionState.Connected || !isRoomSignalingReady(room)) return
    patchParticipantMetadata(localParticipant, {
      pushToTalk: pushToTalkEnabled,
      pttMicHardMuted: pushToTalkEnabled ? userMicMuted : false,
    })
  }, [room, localParticipant, pushToTalkEnabled, userMicMuted])

  useEffect(() => {
    if (!localParticipant) return

    const wasPtt = prevPushToTalkRef.current
    prevPushToTalkRef.current = pushToTalkEnabled

    if (pushToTalkEnabled && !wasPtt) {
      if (micEnabled) {
        setUserMicMuted(false)
        void localParticipant.setMicrophoneEnabled(false)
      } else {
        setUserMicMuted(true)
      }
      pttActiveRef.current = false
      setPttVisible(false)
      return
    }

    if (!pushToTalkEnabled && wasPtt) {
      pttActiveRef.current = false
      setPttVisible(false)
      const wasHardMuted = userMicMuted
      setUserMicMuted(false)
      if (!wasHardMuted && !micEnabled) {
        void localParticipant.setMicrophoneEnabled(true)
      }
    }
  }, [localParticipant, micEnabled, pushToTalkEnabled, userMicMuted])

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (isEditableKeyboardTarget(e.target)) return
      if (!localParticipant || isSelfDeafened) return

      if (pushToTalkEnabled && e.code === pushToTalkKey && !e.repeat) {
        e.preventDefault()
        startPtt()
        return
      }

      if (e.code !== 'Space' || e.repeat) return
      e.preventDefault()
      toggleMic()
    }

    function handleKeyUp(e: KeyboardEvent) {
      if (!pushToTalkEnabled || e.code !== pushToTalkKey) return
      stopPtt()
    }

    document.addEventListener('keydown', handleKeyDown)
    document.addEventListener('keyup', handleKeyUp)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.removeEventListener('keyup', handleKeyUp)
      stopPtt()
    }
  }, [isSelfDeafened, localParticipant, pushToTalkEnabled, pushToTalkKey, startPtt, stopPtt, toggleMic])

  const micShortcutLabel = pushToTalkEnabled
    ? `${formatKeyboardCode(pushToTalkKey)} — Push to talk · Space — Mute / unmute`
    : 'Space — Mute / unmute'

  const micTip = isSelfDeafened ? 'Undeafen & Unmute' : micUiEnabled ? 'Mute (Space)' : 'Unmute (Space)'

  const pttTip = !pushToTalkEnabled
    ? ''
    : isSelfDeafened
      ? 'Undeafen to use push to talk'
      : userMicMuted
        ? 'Unmute to use push to talk'
        : `Hold or ${formatKeyboardCode(pushToTalkKey)} to talk`

  return {
    pttVisible,
    pttAvailable,
    micUiEnabled,
    micShortcutLabel,
    micTip,
    pttTip,
    pushToTalkEnabled,
    toggleMic,
    startPtt,
    stopPtt,
  }
}
