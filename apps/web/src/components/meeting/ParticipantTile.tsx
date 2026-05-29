import { useIsSpeaking, useParticipantInfo, VideoTrack } from '@livekit/components-react'
import type { Participant, RemoteParticipant } from 'livekit-client'
import { ParticipantEvent, Track } from 'livekit-client'
import { MicOff, Pin, VolumeX } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { selectVolume, useParticipantOverridesStore } from '#/lib/participant-overrides.store'
import { getPalette } from '#/lib/participant-palette'
import { useLongPress } from '#/lib/useLongPress'
import { cn } from '#/lib/utils'
import { useVideoPreferencesStore } from '#/lib/video-preferences.store'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { ParticipantContextMenu, ParticipantMenuButton } from '@/components/meeting/ParticipantContextMenu'

// Prevents calling createMediaElementSource twice on the same <audio>/<video> element,
// which throws an InvalidStateError. Keyed weakly so it garbage-collects with the element.
const elementSourceUsed = new WeakMap<HTMLMediaElement, true>()

interface Props {
  participant: Participant
  totalCount: number
  index: number
  isPinned?: boolean
  onTogglePin?: () => void
}

export function ParticipantTile({ participant, totalCount, index, isPinned = false, onTogglePin }: Props) {
  const { name, identity } = useParticipantInfo({ participant })
  const isSpeaking = useIsSpeaking(participant)
  const { isSelfDeafened } = useMeetingRoomContext()

  const volume = useParticipantOverridesStore(selectVolume(identity ?? ''))

  // Parse participant metadata for deafened state (visible to all peers)
  const isDeafened = useMemo(() => {
    try {
      return JSON.parse(participant.metadata ?? '{}').deafened === true
    } catch {
      return false
    }
  }, [participant.metadata])

  // Web Audio boost: GainNode for volume > 100%
  const gainRef = useRef<{ ctx: AudioContext; gain: GainNode } | null>(null)
  const cancelledRef = useRef(false)

  // Cleanup AudioContext on unmount
  useEffect(() => {
    cancelledRef.current = false
    return () => {
      cancelledRef.current = true
      if (gainRef.current) {
        gainRef.current.ctx.close().catch(() => {})
        gainRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    if (participant.isLocal) return
    const remote = participant as RemoteParticipant
    const effective = isSelfDeafened ? 0 : volume

    if (effective <= 1) {
      // Normal range — native volume handles it, tear down gain graph
      remote.setVolume(effective)
      if (gainRef.current) {
        gainRef.current.ctx.close().catch(() => {})
        elementSourceUsed.delete(
          (remote.getTrackPublication(Track.Source.Microphone)?.track?.attachedElements?.[0] as
            | HTMLMediaElement
            | undefined) ?? ({} as HTMLMediaElement),
        )
        gainRef.current = null
      }
    } else {
      // Boost: set native volume to 1, let GainNode amplify beyond
      remote.setVolume(1)

      if (gainRef.current) {
        gainRef.current.gain.gain.value = effective
      } else {
        // Lazily create AudioContext + GainNode on first boost.
        // Track lookup is async-adjacent (element may attach later), so
        // guard against post-unmount writes with cancelledRef.
        const pub = remote.getTrackPublication(Track.Source.Microphone)
        const el = pub?.track?.attachedElements?.[0] as HTMLMediaElement | undefined
        if (el && !cancelledRef.current) {
          // Prevent double-creation on the same media element
          if (elementSourceUsed.has(el)) return
          elementSourceUsed.set(el, true)

          const ctx = new AudioContext()
          const source = ctx.createMediaElementSource(el)
          const gain = ctx.createGain()
          gain.gain.value = effective
          source.connect(gain).connect(ctx.destination)

          // Final unmount check before mutating the ref
          if (cancelledRef.current) {
            ctx.close().catch(() => {})
            return
          }
          gainRef.current = { ctx, gain }
        }
      }
    }
  }, [participant, volume, isSelfDeafened])

  const noopLongPress = useCallback(() => {
    // No-op: Radix ContextMenu handles contextmenu event natively on mobile long-press
  }, [])
  const longPressHandlers = useLongPress(noopLongPress, 500)

  const [cameraTrack, setCameraTrack] = useState(() => participant.getTrackPublication(Track.Source.Camera))
  useEffect(() => {
    const refresh = () => setCameraTrack(participant.getTrackPublication(Track.Source.Camera))
    participant.on(ParticipantEvent.TrackPublished, refresh)
    participant.on(ParticipantEvent.TrackUnpublished, refresh)
    participant.on(ParticipantEvent.TrackMuted, refresh)
    participant.on(ParticipantEvent.TrackUnmuted, refresh)
    participant.on(ParticipantEvent.TrackSubscribed, refresh)
    participant.on(ParticipantEvent.TrackUnsubscribed, refresh)
    return () => {
      participant.off(ParticipantEvent.TrackPublished, refresh)
      participant.off(ParticipantEvent.TrackUnpublished, refresh)
      participant.off(ParticipantEvent.TrackMuted, refresh)
      participant.off(ParticipantEvent.TrackUnmuted, refresh)
      participant.off(ParticipantEvent.TrackSubscribed, refresh)
      participant.off(ParticipantEvent.TrackUnsubscribed, refresh)
    }
  }, [participant])

  const hasCameraVideo = Boolean(cameraTrack?.isSubscribed && !cameraTrack.isMuted)
  const mirrorWebcam = useVideoPreferencesStore((s) => s.mirrorWebcam)
  const displayName = name ?? identity ?? '?'
  const initial = displayName.charAt(0).toUpperCase()

  const palette = useMemo(() => getPalette(displayName), [displayName])

  // Scale avatar to available tile size
  const avatarPx = totalCount === 1 ? 120 : totalCount <= 4 ? 72 : 48
  const fontSizePx = totalCount === 1 ? 44 : totalCount <= 4 ? 26 : 18

  // Memoize expensive dynamic style objects to help React.memo / reconciliation
  const tileStyle = useMemo(
    () => ({
      borderRadius: totalCount === 1 ? 0 : 10,
      background: `radial-gradient(ellipse 90% 70% at 50% 35%, ${palette.tile}, #08080f 72%)`,
      animationDelay: `${index * 0.04}s`,
    }),
    [totalCount, index, palette.tile],
  )

  const avatarStyle = useMemo(
    () => ({
      width: avatarPx,
      height: avatarPx,
      borderRadius: '50%',
      background: palette.avatar,
      fontSize: fontSizePx,
      boxShadow: isSpeaking
        ? `0 0 0 3px rgba(255,255,255,0.18), 0 0 ${avatarPx * 0.6}px ${palette.glow}`
        : `0 0 ${avatarPx * 0.4}px ${palette.glow}`,
      transition: 'box-shadow 0.3s ease',
    }),
    [avatarPx, fontSizePx, isSpeaking, palette.avatar, palette.glow],
  )

  const pinButtonStyle = useMemo(
    () => ({
      background: isPinned ? 'color-mix(in oklab, var(--primary) 70%, transparent)' : 'rgba(0,0,0,0.55)',
      backdropFilter: 'blur(8px)',
      border: `1px solid ${isPinned ? 'color-mix(in oklab, var(--accent-400) 50%, transparent)' : 'rgba(255,255,255,0.1)'}`,
      color: isPinned ? '#e0e7ff' : 'rgba(255,255,255,0.8)',
    }),
    [isPinned],
  )

  return (
    <ParticipantContextMenu participant={participant} isPinned={isPinned} onTogglePin={onTogglePin}>
      <div className={cn('meet-tile group', isSpeaking && 'meet-speaking')} {...longPressHandlers} style={tileStyle}>
        {hasCameraVideo && cameraTrack ? (
          /* Video stream — wrapper suppresses browser's native <video> context menu
             so right-click bubbles to the Radix ContextMenuTrigger instead */
          // biome-ignore lint/a11y/noStaticElementInteractions: wrapper prevents native <video> context menu so Radix ContextMenu works
          <div
            className="absolute inset-0"
            style={{ transform: participant.isLocal && mirrorWebcam ? 'scaleX(-1)' : undefined }}
            onContextMenu={(e) => e.preventDefault()}
          >
            <VideoTrack
              trackRef={{ participant, source: Track.Source.Camera, publication: cameraTrack }}
              className="absolute inset-0 w-full h-full object-cover"
            />
          </div>
        ) : (
          /* No video: gradient avatar */
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-3.5">
            <div className="flex items-center justify-center shrink-0 text-white font-bold" style={avatarStyle}>
              {initial}
            </div>

            {/* Name label + mute indicator (only when large enough to be readable) */}
            {totalCount <= 2 && (
              <div className="flex items-center gap-1.5">
                <span className="text-white/60 text-sm font-medium">
                  {displayName}
                  {participant.isLocal && <span className="text-white/75 text-xs ms-1.5">you</span>}
                </span>
                {isDeafened && <VolumeX size={13} className="shrink-0 text-red-400" />}
                {!participant.isMicrophoneEnabled && <MicOff size={13} className="shrink-0 text-red-400" />}
              </div>
            )}

            {/* Waveform bars — always visible, animated when speaking */}
            <div className="flex items-end gap-[3px] h-[18px]">
              {[0, 1, 2, 3, 4].map((i) => (
                <span
                  key={i}
                  className="inline-block w-[3px] rounded-sm"
                  style={{
                    transformOrigin: 'bottom center',
                    ...(isSpeaking
                      ? {
                          height: 18,
                          background: 'var(--primary)',
                          animation: `meet-speak-bar 0.7s ease-in-out ${i * 0.12}s infinite`,
                        }
                      : {
                          height: 5,
                          background: 'rgba(255,255,255,0.15)',
                          animation: 'none',
                          transition: 'height 0.3s ease, background 0.3s ease',
                        }),
                  }}
                />
              ))}
            </div>
          </div>
        )}

        {/* Name + mute badge at bottom-left — for video tiles or dense grids */}
        {(hasCameraVideo || totalCount > 2) && (
          <div className="absolute bottom-2 left-2 flex items-center gap-[5px] bg-black/60 backdrop-blur-sm rounded-[7px] px-2 py-[3px] max-w-[calc(100%-50px)]">
            <span className="text-white text-xs font-medium overflow-hidden text-ellipsis whitespace-nowrap">
              {displayName}
              {participant.isLocal && <span className="text-white/75 text-[11px] ms-1">you</span>}
            </span>
            {isDeafened && <VolumeX size={11} className="shrink-0 text-red-400" />}
            {!participant.isMicrophoneEnabled ? (
              <MicOff size={11} className="shrink-0 text-red-400" />
            ) : (
              <div className="flex items-end gap-[1.5px] h-3 shrink-0">
                {[0, 1, 2].map((i) => (
                  <span
                    key={i}
                    className="inline-block w-[2px] rounded-px"
                    style={{
                      transformOrigin: 'bottom center',
                      ...(isSpeaking
                        ? {
                            height: 12,
                            background: 'var(--primary)',
                            animation: `meet-speak-bar 0.7s ease-in-out ${i * 0.15}s infinite`,
                          }
                        : {
                            height: 4,
                            background: 'rgba(255,255,255,0.25)',
                            animation: 'none',
                            transition: 'height 0.3s ease, background 0.3s ease',
                          }),
                    }}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {/* Pin button — always visible when pinned, appears on hover otherwise */}
        {onTogglePin && (
          <button
            type="button"
            onClick={onTogglePin}
            className={cn(
              'absolute top-2 right-2 w-[30px] h-[30px] rounded-lg flex items-center justify-center cursor-pointer transition-[opacity,background] duration-150',
              isPinned ? '' : 'opacity-0 group-hover:opacity-100',
            )}
            style={pinButtonStyle}
            aria-label={isPinned ? 'Unpin participant' : 'Pin participant'}
          >
            <Pin size={13} className={isPinned ? 'fill-current' : ''} />
          </button>
        )}

        {/* 3-dot button — top-left corner (pin button is top-right) */}
        <div className="absolute top-2 left-2 opacity-0 group-hover:opacity-100 transition-opacity duration-150">
          <ParticipantMenuButton participant={participant} isPinned={isPinned} onTogglePin={onTogglePin} />
        </div>
      </div>
    </ParticipantContextMenu>
  )
}
