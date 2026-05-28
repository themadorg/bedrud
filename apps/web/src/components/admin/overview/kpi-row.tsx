import { Activity, Shield, UserCheck, Users, Video } from 'lucide-react'
import { KpiCard } from '#/components/admin/overview/kpi-card'
import { Skeleton } from '#/components/ui/skeleton'
import type { OverviewKPIs } from '#/lib/use-admin-overview'

export function AdminKpiRow({ kpis, isLoading }: { kpis?: OverviewKPIs; isLoading: boolean }) {
  if (isLoading || !kpis) {
    return (
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        {Array.from({ length: 5 }, (_, i) => (
          <div key={i} className="rounded-lg border p-5">
            <Skeleton className="mb-2 h-4 w-20" />
            <Skeleton className="h-8 w-16" />
            <Skeleton className="mt-2 h-3 w-24" />
          </div>
        ))}
      </section>
    )
  }

  return (
    <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
      <KpiCard title="Total users" entry={kpis.totalUsers} icon={<Users className="h-4 w-4" />} />
      <KpiCard title="Online now" entry={kpis.onlineNow} icon={<Activity className="h-4 w-4" />} />
      <KpiCard title="Total rooms" entry={kpis.totalRooms} icon={<Video className="h-4 w-4" />} />
      <KpiCard title="Active sessions" entry={kpis.activeSessions} icon={<UserCheck className="h-4 w-4" />} />
      <KpiCard title="Pending actions" entry={kpis.pendingActions} icon={<Shield className="h-4 w-4" />} />
    </section>
  )
}
