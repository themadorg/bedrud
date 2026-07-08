/** True when an incoming data packet belongs to the Bedrud chat protocol. */
export function isMeetingChatDataTopic(topic: string | undefined, rawType: unknown): boolean {
  if (topic === 'chat' || topic === 'lk-chat-topic' || topic === 'lk.chat') return true
  if (typeof rawType !== 'string') return false
  return rawType === 'chat' || rawType.startsWith('chat_')
}
