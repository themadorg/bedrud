import { describe, expect, it } from 'vitest'
import { isMeetingChatDataTopic } from './chatTopic'

describe('isMeetingChatDataTopic', () => {
  it('matches Bedrud and LiveKit chat topics', () => {
    expect(isMeetingChatDataTopic('chat', 'chat')).toBe(true)
    expect(isMeetingChatDataTopic('lk-chat-topic', 'chat')).toBe(true)
    expect(isMeetingChatDataTopic('lk.chat', 'chat')).toBe(true)
  })

  it('matches chat wire types when topic is missing', () => {
    expect(isMeetingChatDataTopic(undefined, 'chat')).toBe(true)
    expect(isMeetingChatDataTopic(undefined, 'chat_chunk')).toBe(true)
    expect(isMeetingChatDataTopic(undefined, 'reaction')).toBe(false)
  })

  it('rejects unrelated topics and wire types', () => {
    expect(isMeetingChatDataTopic('presence', 'deafen_state')).toBe(false)
    expect(isMeetingChatDataTopic('stage', 'stage_set')).toBe(false)
    expect(isMeetingChatDataTopic('presence', 'chat')).toBe(true)
  })
})
