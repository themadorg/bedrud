import type { ChatPoll } from '../MeetingContext'

export interface PollOptionResult {
  optionId: string
  text: string
  count: number
  pct: number
  voterNames: string[]
}

export function computePollResults(poll: ChatPoll, resolveName: (identity: string) => string): PollOptionResult[] {
  const voteEntries = Object.entries(poll.votes)
  const totalVotes = voteEntries.length

  return poll.options.map((option) => {
    const voters = voteEntries.filter(([, optionId]) => optionId === option.id).map(([identity]) => identity)
    const count = voters.length
    const pct = totalVotes > 0 ? Math.round((count / totalVotes) * 100) : 0
    return {
      optionId: option.id,
      text: option.text,
      count,
      pct,
      voterNames: voters.map(resolveName),
    }
  })
}
