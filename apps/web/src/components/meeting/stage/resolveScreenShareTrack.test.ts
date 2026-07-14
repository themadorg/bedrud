import { Track } from 'livekit-client'
import { describe, expect, it } from 'vitest'
import { resolveScreenShareTrack } from './resolveScreenShareTrack'

function mockLocal(identity: string, hasScreen = false) {
  const publication = hasScreen ? { track: { sid: 'tr_local' }, source: Track.Source.ScreenShare } : undefined
  return {
    identity,
    getTrackPublication: (source: Track.Source) => (source === Track.Source.ScreenShare ? publication : undefined),
  } as never
}

describe('resolveScreenShareTrack', () => {
  it('prefers local publication when stage owner is self', () => {
    const track = resolveScreenShareTrack('user-1', mockLocal('user-1', true), [])
    expect(track?.participant.identity).toBe('user-1')
    expect(track?.publication?.track?.sid).toBe('tr_local')
  })

  it('falls back to subscribed remote track', () => {
    const remote = {
      participant: { identity: 'user-2' },
      publication: { track: { sid: 'tr_remote' } },
      source: Track.Source.ScreenShare,
    }
    const track = resolveScreenShareTrack('user-2', mockLocal('user-1'), [remote as never])
    expect(track?.participant.identity).toBe('user-2')
  })
})
