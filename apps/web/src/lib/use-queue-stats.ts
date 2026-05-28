import { useQuery } from '@tanstack/react-query'
import { api } from '#/lib/api'

export interface FailedJobSummary {
  id: string
  type: string
  error: string
  attempts: number
  updatedAt: string
  age: string
}

export interface QueueStats {
  pending: number
  active: number
  done24h: number
  failed24h: number
  total: number
  maxDepth: number
  oldestPending: string | null
  recentFailures: FailedJobSummary[]
  processedPerMin: number
  failedPerMin: number
  failRate: number

  pendingEmail: number
  failedEmail24h: number
  lastSendError?: string
  lastSendErrorAt?: string
}

export function useQueueStats() {
  return useQuery({
    queryKey: ['admin', 'queue'],
    queryFn: () => api.get<QueueStats>('/api/admin/queue'),
    refetchInterval: 10_000,
  })
}
