import { useIsSpeaking, VideoTrack } from '@livekit/components-react'
import type { Participant } from 'livekit-client'
import { Track } from 'livekit-client'
import { MicOff, Minimize2 } from 'lucide-react'
import { useMemo } from 'react'
import { ParticipantAvatar } from '#/components/meeting/ParticipantAvatar'
import { useAudioPreferencesStore } from '#/lib/audio-preferences.store'
import { getPalette } from '#/lib/participant-palette'
import { shouldShowMicMutedIndicator } from '#/lib/push-to-talk-participant'
import { useMeetingRoomContext } from '@/components/meeting/MeetingContext'
import { hasCameraVideo, useCameraTrackPublication } from '@/components/meeting/useCameraTrackPublication'

interface Props {
  participant: Participant
  onClose: () => void
}

export function SpotlightView({ participant, onClose }: Props) {
  const isSpeaking = useIsSpeaking(participant)
  const { getParticipantDisplayName, getParticipantAvatarUrl } = useMeetingRoomContext()
  const cameraTrack = useCameraTrackPublication(participant)
  const showCameraVideo = hasCameraVideo(cameraTrack)
  const displayName = getParticipantDisplayName(participant)
  const avatarUrl = getParticipantAvatarUrl(participant)
  const palette = useMemo(() => getPalette(displayName), [displayName])
  const pushToTalkEnabled = useAudioPreferencesStore((s) => s.pushToTalkEnabled)

  return (
    <div className="relative w-full h-full flex items-center justify-center bg-[#030308] px-6 py-5">
      {/* Main content area — 16:9 with max width */}
      <div
        className="relative w-full h-full overflow-hidden rounded-2xl"
        style={{
          maxWidth: 'min(100%, calc(100vh * 16/9))',
          background: showCameraVideo
            ? '#000'
            : `radial-gradient(ellipse 80% 60% at 50% 35%, ${palette.glow.replace('0.5', '0.12')}, #0c0c1a 70%)`,
          boxShadow: isSpeaking
            ? `0 0 0 3px color-mix(in oklab, var(--primary) 80%, transparent), 0 0 60px color-mix(in oklab, var(--primary) 30%, transparent)`
            : '0 0 0 1px rgba(255,255,255,0.06)',
          transition: 'box-shadow 0.3s ease',
        }}
      >
        {showCameraVideo && cameraTrack ? (
          <VideoTrack
            trackRef={{ participant, source: Track.Source.Camera, publication: cameraTrack }}
            className="absolute inset-0 w-full h-full object-cover"
          />
        ) : (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-5">
            <ParticipantAvatar
              avatarUrl={avatarUrl}
              initials={displayName.charAt(0).toUpperCase()}
              paletteBackground={palette.avatar}
              className="flex items-center justify-center"
              style={{
                width: 110,
                height: 110,
                fontSize: 42,
                boxShadow: `0 0 70px ${palette.glow}`,
              }}
            />
            <span className="text-white/65 text-lg font-medium">{displayName}</span>
            {isSpeaking && (
              <div className="flex items-center gap-1">
                {[0, 1, 2, 3, 4].map((i) => (
                  <span
                    key={i}
                    className="inline-block w-1 rounded-sm bg-primary"
                    style={{
                      height: 22,
                      transformOrigin: 'bottom center',
                      animation: `meet-speak-bar 0.7s ease-in-out ${i * 0.12}s infinite`,
                    }}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {/* Name + mute badge */}
        <div className="absolute bottom-4 left-4 flex items-center gap-[7px] bg-black/60 backdrop-blur-md rounded-lg px-3 py-[5px]">
          <span className="text-white text-[13px] font-medium">{displayName}</span>
          {shouldShowMicMutedIndicator(participant, pushToTalkEnabled) && (
            <MicOff size={13} className="shrink-0 text-red-400" />
          )}
        </div>

        {/* Exit spotlight */}
        <button
          type="button"
          onClick={onClose}
          className="absolute top-3 right-3 w-[34px] h-[34px] rounded-lg bg-black/60 backdrop-blur-sm border border-white/10 flex items-center justify-center text-white/80 cursor-pointer"
          aria-label="Exit spotlight"
        >
          <Minimize2 size={15} />
        </button>
      </div>
    </div>
  )
}
