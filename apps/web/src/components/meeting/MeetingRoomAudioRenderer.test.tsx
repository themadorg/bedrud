import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MeetingRoomAudioRenderer } from './MeetingRoomAudioRenderer'

const START_AUDIO_LABEL = 'Turn on sound'

const audioPlayback = vi.hoisted(() => ({ canPlayAudio: true, startAudio: vi.fn() }))

vi.mock('@livekit/components-react', () => ({
  RoomAudioRenderer: () => null,
  useRoomContext: () => ({ remoteParticipants: new Map(), on: vi.fn(), off: vi.fn() }),
  useAudioPlayback: () => audioPlayback,
}))

vi.mock('@/components/meeting/MeetingContext', () => ({
  useMeetingRoomContext: () => ({ isSelfDeafened: false, isServerDeafened: false }),
}))

describe('MeetingRoomAudioRenderer', () => {
  beforeEach(() => {
    audioPlayback.canPlayAudio = true
    audioPlayback.startAudio = vi.fn()
  })

  afterEach(() => {
    cleanup()
  })

  it('should show the start audio prompt when the browser blocks playback', () => {
    audioPlayback.canPlayAudio = false

    render(<MeetingRoomAudioRenderer />)

    expect(screen.queryByRole('button', { name: START_AUDIO_LABEL })).not.toBeNull()
  })

  it('should hide the start audio prompt when playback is allowed', () => {
    render(<MeetingRoomAudioRenderer />)

    expect(screen.queryByRole('button', { name: START_AUDIO_LABEL })).toBeNull()
  })

  it('should start room audio when the prompt is tapped', async () => {
    audioPlayback.canPlayAudio = false

    render(<MeetingRoomAudioRenderer />)
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: START_AUDIO_LABEL }))
    })

    expect(audioPlayback.startAudio).toHaveBeenCalledOnce()
  })
})
