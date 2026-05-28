import { useQuery } from '@tanstack/react-query'
import { api } from '#/lib/api'

export interface OverviewHealth {
  status: 'healthy' | 'degraded' | 'down'
  tls: TLSStatus | null
  realtime: string
  alertsCount: number
  uptimeSeconds: number
  dbStatus: string
}

export interface TLSStatus {
  enabled: boolean
  daysRemaining: number
  expiryDate: string
  status: 'valid' | 'expiring' | 'expired' | 'unknown' | 'error'
}

export interface KpiEntry {
  value: number
  delta?: number
  deltaLabel?: string
  deltaPercent?: number
  peakToday?: number
  activeNow?: number
}

export interface OverviewKPIs {
  totalUsers: KpiEntry
  onlineNow: KpiEntry
  totalRooms: KpiEntry
  activeSessions: KpiEntry
  pendingActions: KpiEntry
}

export interface DayActivity {
  date: string
  roomsCreated: number
  roomsActive: number
  participants: number
}

export interface RoomComposition {
  live: number
  public: number
  private: number
  persistent: number
  stale: number
}

export interface AttentionItem {
  type: string
  severity: 'error' | 'warning' | 'info'
  message: string
  daysLeft?: number
  roomId?: string
}

export interface RecentUser {
  id: string
  name: string
  email: string
  provider: string
  createdAt: string
}

export interface RoomEvent {
  type: string
  roomId?: string
  roomName?: string
  userId?: string
  userName?: string
  timestamp: string
}

export interface AdminRoomEvent {
  type: string
  roomId?: string
  roomName?: string
  userId?: string
  userName?: string
  timestamp: string
}

export interface InstanceInfo {
  name: string
  version: string
  uptimeSeconds: number
  startedAt: string
}

export interface AdminOverview {
  health: OverviewHealth
  kpis: OverviewKPIs
  activityTrend: DayActivity[]
  roomComposition: RoomComposition
  needsAttention: AttentionItem[]
  recentSignups: RecentUser[]
  recentRoomEvents: RoomEvent[]
  instanceInfo: InstanceInfo
}

export function useAdminOverview() {
  return useQuery({
    queryKey: ['admin', 'overview'],
    queryFn: () => api.get<AdminOverview>('/api/admin/overview'),
    refetchInterval: 60_000,
  })
}
