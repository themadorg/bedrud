import { useIsSpeaking, useParticipantInfo, VideoTrack } from '@livekit/components-react'
import type { Participant, RemoteParticipant } from 'livekit-client'
import { Track } from 'livekit-client'
import { Maximize2, MicOff, Pin, Scan } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { DeafenHeadphonesIcon } from '#/components/meeting/DeafenHeadphonesIcon'
import { ParticipantAvatar } from '#/components/meeting/ParticipantAvatar'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { useAvatarPhotoPalette } from '#/lib/avatar-photo-palette'
import { resolveAvatarUrl } from '#/lib/avatar-url'
import { selectVolume, useParticipantOverridesStore } from '#/lib/participant-overrides.store'
import { getPalette } from '#/lib/participant-palette'
import { isPushToTalkParticipant, shouldShowMicMutedIndicator } from '#/lib/push-to-talk-participant'
import { cn } from '#/lib/utils'
import { useVideoPreferencesStore } from '#/lib/video-preferences.store'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { hasCameraVideo, useCameraTrackPublication } from '@/components/meeting/useCameraTrackPublication'

const elementSourceUsed = new WeakMap<HTMLMediaElement, true>()

interface Props {
  participant: Participant
  totalCount: number
  index: number
  isPinned?: boolean
  onTogglePin?: () => void
}

function MicStatusIcon({
  participant,
  isSpeaking,
  size,
  localPushToTalkEnabled,
}: {
  participant: Participant
  isSpeaking: boolean
  size: number
  localPushToTalkEnabled: boolean
}) {
  const isPtt = isPushToTalkParticipant(participant, localPushToTalkEnabled)

  if (shouldShowMicMutedIndicator(participant, localPushToTalkEnabled)) {
    return <MicOff size={size} className="shrink-0 text-red-400" />
  }

  if (isPtt && !isSpeaking) return null

  const barCount = size <= 11 ? 3 : 5
  const maxHeight = size <= 11 ? 12 : 18
  const idleHeight = size <= 11 ? 4 : 5

  return (
    <div className={cn('flex shrink-0 items-end', size <= 11 ? 'h-3 gap-[1.5px]' : 'h-[18px] gap-[3px]')}>
      {Array.from({ length: barCount }).map((_, i) => (
        <span
          key={i}
          className={cn('inline-block rounded-sm', size <= 11 ? 'w-[2px] rounded-px' : 'w-[3px]')}
          style={{
            transformOrigin: 'bottom center',
            ...(isSpeaking
              ? {
                  height: maxHeight,
                  background: 'var(--primary)',
                  animation: `meet-speak-bar 0.7s ease-in-out ${i * (size <= 11 ? 0.15 : 0.12)}s infinite`,
                }
              : {
                  height: idleHeight,
                  background: size <= 11 ? 'rgba(255,255,255,0.25)' : 'rgba(255,255,255,0.15)',
                  animation: 'none',
                  transition: 'height 0.3s ease, background 0.3s ease',
                }),
          }}
        />
      ))}
    </div>
  )
}

export function ParticipantTile({ participant, totalCount, index, isPinned = false, onTogglePin }: Props) {
  const { identity } = useParticipantInfo({ participant })
  const isSpeaking = useIsSpeaking(participant)
  const { isSelfDeafened, isParticipantDeafened, getParticipantDisplayName, getParticipantAvatarUrl } =
    useMeetingRoomContext()

  const volume = useParticipantOverridesStore(selectVolume(identity ?? ''))

  const isDeafened = isParticipantDeafened(participant)

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

  const cameraTrack = useCameraTrackPublication(participant)
  const showCameraVideo = hasCameraVideo(cameraTrack)
  /** cover = crop/fill tile; contain = fit full frame centered (letterbox) */
  const [videoFit, setVideoFit] = useState<'cover' | 'contain'>('cover')
  const mirrorWebcam = useVideoPreferencesStore((s) => s.mirrorWebcam)
  const pushToTalkEnabled = useAudioPreferencesStore((s) => s.pushToTalkEnabled)
  const displayName = getParticipantDisplayName(participant)
  const avatarUrl = getParticipantAvatarUrl(participant)
  const resolvedAvatarUrl = resolveAvatarUrl(avatarUrl)
  const initial = displayName.charAt(0).toUpperCase()

  const namePalette = useMemo(() => getPalette(displayName), [displayName])
  const photoPalette = useAvatarPhotoPalette(resolvedAvatarUrl)
  const palette = resolvedAvatarUrl && photoPalette ? photoPalette : namePalette

  // Scale avatar to available tile size
  const avatarPx = totalCount === 1 ? 120 : totalCount <= 4 ? 72 : 48
  const fontSizePx = totalCount === 1 ? 44 : totalCount <= 4 ? 26 : 18

  // Memoize expensive dynamic style objects to help React.memo / reconciliation
  const tileStyle = useMemo(
    () => ({
      borderRadius: totalCount === 1 ? 0 : 10,
      background: `radial-gradient(ellipse 90% 70% at 50% 35%, ${palette.tile}, var(--meet-tile-fallback) 72%)`,
      animationDelay: `${index * 0.04}s`,
    }),
    [totalCount, index, palette.tile],
  )

  const avatarStyle = useMemo(
    () => ({
      width: avatarPx,
      height: avatarPx,
      background: palette.avatar,
      fontSize: fontSizePx,
      clipPath: 'circle(50% at 50% 50%)',
      boxShadow: isSpeaking
        ? `0 0 0 3px rgba(255,255,255,0.18), 0 0 ${avatarPx * 0.6}px ${palette.glow}`
        : `0 0 ${avatarPx * 0.4}px ${palette.glow}`,
      transition: 'box-shadow 0.3s ease',
    }),
    [avatarPx, fontSizePx, isSpeaking, palette.avatar, palette.glow],
  )

  const tileActionBtnClass =
    'z-10 flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-lg border backdrop-blur-sm transition-[opacity,background,color,border-color] duration-150'

  const pinButtonClass = cn(
    'absolute top-2 right-2',
    tileActionBtnClass,
    isPinned
      ? 'border-[color-mix(in_oklab,var(--accent-600)_35%,transparent)] bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
      : 'border-[var(--meet-tile-action-border)] bg-[var(--meet-tile-action-bg)] text-[var(--meet-tile-action-fg)]',
    isPinned ? '' : 'opacity-0 group-hover:opacity-100',
  )

  const isFitContain = videoFit === 'contain'
  const fitButtonClass = cn(
    'absolute top-2',
    onTogglePin ? 'right-[42px]' : 'right-2',
    tileActionBtnClass,
    isFitContain
      ? 'border-[color-mix(in_oklab,var(--accent-600)_35%,transparent)] bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
      : 'border-[var(--meet-tile-action-border)] bg-[var(--meet-tile-action-bg)] text-[var(--meet-tile-action-fg)]',
    // Stay visible while fit mode is on so the user can switch back without hunting
    isFitContain ? '' : 'opacity-0 group-hover:opacity-100',
  )

  return (
    <div className={cn('meet-tile group h-full w-full', isSpeaking && 'meet-speaking')} style={tileStyle}>
      {showCameraVideo && cameraTrack ? (
        <div
          className={cn('absolute inset-0', isFitContain && 'bg-black')}
          style={{ transform: participant.isLocal && mirrorWebcam ? 'scaleX(-1)' : undefined }}
        >
          {/*
            LiveKit ships `.lk-participant-media-video[data-lk-orientation=landscape] { object-fit: cover }`
            which beats Tailwind utilities. Inline style is required to actually switch fit modes.
          */}
          <VideoTrack
            trackRef={{ participant, source: Track.Source.Camera, publication: cameraTrack }}
            className="absolute inset-0 h-full w-full"
            style={{
              objectFit: isFitContain ? 'contain' : 'cover',
              objectPosition: 'center',
            }}
          />
        </div>
      ) : (
        /* No video: gradient avatar */
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3.5">
          <ParticipantAvatar
            avatarUrl={avatarUrl}
            initials={initial}
            paletteBackground={palette.avatar}
            style={avatarStyle}
            textClassName=""
          />

          {/* Name label + mute indicator (only when large enough to be readable) */}
          {totalCount <= 2 && (
            <div className="flex items-center gap-1.5">
              <span className="text-sm font-medium text-white/60">
                {displayName}
                {participant.isLocal && <span className="ms-1.5 text-xs text-[var(--meet-btn-muted-fg)]">you</span>}
              </span>
              <MicStatusIcon
                participant={participant}
                isSpeaking={isSpeaking}
                size={13}
                localPushToTalkEnabled={pushToTalkEnabled}
              />
              {isDeafened && <DeafenHeadphonesIcon size={13} off className="text-red-400" />}
            </div>
          )}
        </div>
      )}

      {/* Name + mute badge at bottom-left — for video tiles or dense grids */}
      {(showCameraVideo || totalCount > 2) && (
        <div className="absolute bottom-2 left-2 flex max-w-[calc(100%-50px)] items-center gap-[5px] rounded-[7px] border border-[var(--meet-tile-action-border)] bg-[var(--meet-tile-action-bg)] px-2 py-[3px] backdrop-blur-sm">
          <span className="overflow-hidden text-ellipsis whitespace-nowrap text-xs font-medium text-[var(--meet-tile-action-fg)]">
            {displayName}
            {participant.isLocal && <span className="ms-1 text-[11px] text-[var(--meet-btn-muted-fg)]">you</span>}
          </span>
          <MicStatusIcon
            participant={participant}
            isSpeaking={isSpeaking}
            size={11}
            localPushToTalkEnabled={pushToTalkEnabled}
          />
          {isDeafened && <DeafenHeadphonesIcon size={11} off className="text-red-400" />}
        </div>
      )}

      {/* Fit video — hover only when cover; always visible when contain so it can be undone */}
      {showCameraVideo && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            setVideoFit((mode) => (mode === 'cover' ? 'contain' : 'cover'))
          }}
          className={fitButtonClass}
          aria-label={isFitContain ? 'Fill video in tile' : 'Fit video (center, no crop)'}
          aria-pressed={isFitContain}
          title={isFitContain ? 'Fill video' : 'Fit video'}
        >
          {isFitContain ? <Maximize2 size={13} /> : <Scan size={13} />}
        </button>
      )}

      {/* Pin button — always visible when pinned, appears on hover otherwise */}
      {onTogglePin && (
        <button
          type="button"
          onClick={onTogglePin}
          className={pinButtonClass}
          aria-label={isPinned ? 'Unpin participant' : 'Pin participant'}
        >
          <Pin size={13} className={isPinned ? 'fill-current' : ''} />
        </button>
      )}
    </div>
  )
}
