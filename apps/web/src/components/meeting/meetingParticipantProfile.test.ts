import { afterEach, describe, expect, it, vi } from 'vitest'

const get = vi.fn()
vi.mock('#/lib/api', () => ({ api: { get: (...args: unknown[]) => get(...args) } }))

const { fetchMeetingParticipantProfile } = await import('./meetingParticipantProfile')

afterEach(() => {
  get.mockReset()
})

describe('fetchMeetingParticipantProfile', () => {
  it('should share one request between callers asking for the same participant at once', async () => {
    get.mockResolvedValue({ id: 'peer', name: 'Peer', welcomeSoundUrl: '/uploads/welcome-sounds/peer.mp3' })

    const [first, second] = await Promise.all([
      fetchMeetingParticipantProfile('room-1', 'peer'),
      fetchMeetingParticipantProfile('room-1', 'peer'),
    ])

    expect(get).toHaveBeenCalledTimes(1)
    expect(first).toEqual(second)
    expect(first?.welcomeSoundUrl).toBe('/uploads/welcome-sounds/peer.mp3')
  })

  it('should ask again once the earlier request has finished', async () => {
    get.mockResolvedValue({ id: 'peer', name: 'Peer' })

    await fetchMeetingParticipantProfile('room-1', 'peer')
    await fetchMeetingParticipantProfile('room-1', 'peer')

    expect(get).toHaveBeenCalledTimes(2)
  })

  it('should keep separate requests for different participants', async () => {
    get.mockResolvedValue({ id: 'x', name: 'X' })

    await Promise.all([
      fetchMeetingParticipantProfile('room-1', 'peer-a'),
      fetchMeetingParticipantProfile('room-1', 'peer-b'),
    ])

    expect(get).toHaveBeenCalledTimes(2)
  })
})
