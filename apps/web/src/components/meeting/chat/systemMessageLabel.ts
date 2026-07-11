import type { SystemMessage } from '../MeetingContext'

/** Human-readable line for a system/event entry (centered pill, not a chat bubble). */
export function systemMessageLabel(msg: SystemMessage): string {
  if (typeof msg.message === 'string' && msg.message.trim()) {
    return msg.message.trim()
  }
  if (msg.event === 'kick' && msg.target && msg.actor) {
    return `${msg.target} was kicked by ${msg.actor}`
  }
  if (msg.event === 'ban' && msg.target && msg.actor) {
    return `${msg.target} was banned by ${msg.actor}`
  }
  if (msg.event === 'spotlight' && msg.target && msg.actor) {
    return `${msg.actor} spotlighted ${msg.target}`
  }
  if (msg.event === 'stage') {
    return msg.actor ? `${msg.actor} shared something on stage` : 'Something was shared on stage'
  }
  if (msg.actor && msg.target) {
    return `${msg.target} — ${msg.event} — ${msg.actor}`
  }
  return msg.event
}
