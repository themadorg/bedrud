import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronRight, Plus, RefreshCw } from 'lucide-react'
import { AdminActivityChart } from '#/components/admin/overview/activity-chart'
import { AdminHealthStrip } from '#/components/admin/overview/health-strip'
import { AdminKpiRow } from '#/components/admin/overview/kpi-row'
import { AdminNeedsAttention } from '#/components/admin/overview/needs-attention'
import { AdminRecentEvents } from '#/components/admin/overview/recent-events'
import { AdminRecentSignups } from '#/components/admin/overview/recent-signups'
import { AdminRoomComposition } from '#/components/admin/overview/room-composition'
import { Button } from '#/components/ui/button'
import { Separator } from '#/components/ui/separator'
import { useAdminOverview } from '#/lib/use-admin-overview'

export function AdminOverviewPage() {
  const navigate = useNavigate()
  const { data, isLoading, isError, error, refetch, isRefetching } = useAdminOverview()

  if (isError) {
    return (
      <div className="mx-auto max-w-6xl px-4 pt-8">
        <div className="border border-destructive/30 bg-destructive/10 px-4 py-4 text-sm flex items-center justify-between">
          <span className="text-destructive">
            {error instanceof Error ? error.message : 'Failed to load admin overview.'}
          </span>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="mr-1.5 h-3 w-3" />
            Retry
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-6xl space-y-4 px-4 pb-8">
      {/* Row 1: Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-sm font-semibold">Overview</h1>
          <p className="text-xs text-muted-foreground">Real-time system stats for this Bedrud instance.</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isRefetching}>
            <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${isRefetching ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Separator orientation="vertical" className="h-5" />
          <Button size="sm" onClick={() => navigate({ to: '/dashboard/admin/rooms', search: { create: true } })}>
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            Create room
          </Button>
        </div>
      </div>

      {/* Row 2: Health strip */}
      {data?.health && <AdminHealthStrip health={data.health} />}

      {/* Row 3: KPI row */}
      <AdminKpiRow kpis={data?.kpis} isLoading={isLoading} />

      {/* Row 4: Activity + Composition */}
      <section className="grid gap-4 xl:grid-cols-12">
        <div className="xl:col-span-8">{data?.activityTrend && <AdminActivityChart data={data.activityTrend} />}</div>
        <div className="xl:col-span-4">
          {data?.roomComposition && <AdminRoomComposition data={data.roomComposition} />}
        </div>
      </section>

      {/* Row 5: Operations panels */}
      <section className="grid gap-4 xl:grid-cols-12">
        <div className="xl:col-span-4">
          {data?.needsAttention && <AdminNeedsAttention items={data.needsAttention} />}
        </div>
        <div className="xl:col-span-4">
          <AdminRecentSignups users={data?.recentSignups ?? []} isLoading={isLoading} />
          {data?.recentSignups && (
            <Link
              to="/dashboard/admin/users/recent-signups"
              className="mt-1 flex items-center justify-end gap-1 px-2 py-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors"
            >
              View all <ChevronRight className="h-3 w-3" />
            </Link>
          )}
        </div>
        <div className="xl:col-span-4">
          {data?.recentRoomEvents && (
            <>
              <AdminRecentEvents events={data.recentRoomEvents} />
              <Link
                to="/dashboard/admin/rooms/events"
                className="mt-1 flex items-center justify-end gap-1 px-2 py-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors"
              >
                View all <ChevronRight className="h-3 w-3" />
              </Link>
            </>
          )}
        </div>
      </section>
    </div>
  )
}
