import { Activity, AlertTriangle, CheckCircle2, Clock, Mail, RefreshCw } from 'lucide-react'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import { Progress } from '#/components/ui/progress'
import { Skeleton } from '#/components/ui/skeleton'
import { useQueueStats } from '#/lib/use-queue-stats'

function fmt(s: number): string {
  return (s ?? 0).toFixed(1)
}

function pct(v: number) {
  return (v * 100).toFixed(1)
}

function fillColor(used: number, max: number): string {
  const r = max > 0 ? used / max : 0
  if (r < 0.5) return 'bg-green-500'
  if (r < 0.8) return 'bg-yellow-500'
  return 'bg-red-500'
}

function oldestLabel(d: string | null): string {
  if (!d) return '—'
  const ms = Date.now() - new Date(d).getTime()
  const sec = Math.floor(ms / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const h = Math.floor(min / 60)
  return `${h}h ago`
}

function failRateLabel(r: number): string {
  if (r === 0) return '0%'
  return `${pct(r)}%`
}

function KpiCardBig({
  title,
  value,
  sub,
  icon,
  color,
}: {
  title: string
  value: string | number
  sub?: string
  icon: React.ReactNode
  color: string
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-xs font-medium text-muted-foreground">{title}</CardTitle>
        <div className={`${color}`}>{icon}</div>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-semibold tabular-nums">{value}</div>
        {sub && <p className="mt-1 text-xs text-muted-foreground">{sub}</p>}
      </CardContent>
    </Card>
  )
}

function SkeletonCard() {
  return (
    <div className="rounded-lg border p-5">
      <Skeleton className="mb-2 h-4 w-20" />
      <Skeleton className="h-8 w-16" />
      <Skeleton className="mt-2 h-3 w-24" />
    </div>
  )
}

export function QueueStatsPage() {
  const { data, isLoading, refetch, isRefetching } = useQueueStats()

  if (isLoading) {
    return (
      <div className="mx-auto max-w-6xl space-y-4 px-4 pb-8">
        <div className="flex items-center justify-between">
          <div>
            <Skeleton className="h-5 w-24" />
            <Skeleton className="mt-1 h-3 w-52" />
          </div>
        </div>
        <div className="grid gap-4 md:grid-cols-4">
          {Array.from({ length: 4 }, (_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
        <div className="grid gap-4 md:grid-cols-2">
          <SkeletonCard />
          <SkeletonCard />
        </div>
        <Skeleton className="h-6 w-full" />
        <Skeleton className="h-40 w-full" />
      </div>
    )
  }

  if (!data) {
    return (
      <div className="mx-auto max-w-6xl px-4 pb-8 pt-8 text-center text-sm text-muted-foreground">
        Failed to load queue stats.
      </div>
    )
  }

  const failures = data.recentFailures ?? []
  const depthUsed = (data.pending ?? 0) + (data.active ?? 0)
  const maxDepth = data.maxDepth ?? 10000
  const depthPct = maxDepth > 0 ? Math.min((depthUsed / maxDepth) * 100, 100) : 0

  return (
    <div className="mx-auto max-w-6xl space-y-4 px-4 pb-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-sm font-semibold">Queue</h1>
          <p className="text-xs text-muted-foreground">
            {data.total} total jobs · {data.pending} pending · {data.active} active
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isRefetching}>
          <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${isRefetching ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* KPI row */}
      <section className="grid gap-4 md:grid-cols-4">
        <KpiCardBig
          title="Pending"
          value={data.pending}
          sub={data.oldestPending ? `Oldest: ${oldestLabel(data.oldestPending)}` : undefined}
          icon={<Clock className="h-4 w-4" />}
          color="text-amber-500"
        />
        <KpiCardBig title="Active" value={data.active} icon={<Activity className="h-4 w-4" />} color="text-blue-500" />
        <KpiCardBig
          title="Done (24h)"
          value={data.done24h}
          icon={<CheckCircle2 className="h-4 w-4" />}
          color="text-green-500"
        />
        <KpiCardBig
          title="Failed (24h)"
          value={data.failed24h}
          sub={`Fail rate: ${(data.failRate ?? 0) === 0 && (data.done24h ?? 0) + (data.failed24h ?? 0) === 0 ? '—' : failRateLabel(data.failRate ?? 0)}`}
          icon={<AlertTriangle className="h-4 w-4" />}
          color="text-red-500"
        />
      </section>

      {/* Email queue health */}
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">Email Queue</CardTitle>
            <Mail className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-semibold tabular-nums">
              {data.pendingEmail ?? 0}
              <span className="ml-1 text-xs font-normal text-muted-foreground">pending</span>
            </div>
            <p className="mt-1 flex items-center gap-2 text-xs">
              <span className={data.failedEmail24h > 0 ? 'text-red-500 font-medium' : 'text-muted-foreground'}>
                {data.failedEmail24h ?? 0} failed (24h)
              </span>
            </p>
            {data.lastSendError && (
              <div className="mt-2 rounded border border-red-500/20 bg-red-500/5 px-2 py-1.5 text-[10px]">
                <p className="font-medium text-red-500">Last send error</p>
                <p className="mt-0.5 break-all text-muted-foreground" title={data.lastSendError}>
                  {data.lastSendError.length > 120 ? data.lastSendError.slice(0, 120) + '…' : data.lastSendError}
                </p>
                {data.lastSendErrorAt && (
                  <p className="mt-0.5 italic text-muted-foreground">{oldestLabel(data.lastSendErrorAt)}</p>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      {/* Rate cards */}
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">Processed / min</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-semibold tabular-nums">{fmt(data.processedPerMin)}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">Failed / min</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-semibold tabular-nums">{fmt(data.failedPerMin)}</div>
            <p className="mt-1 flex items-center gap-1 text-xs text-red-500">
              <Badge variant="destructive" className="px-1 py-px text-[10px]">
                {(data.done24h ?? 0) + (data.failed24h ?? 0) === 0 ? '—' : failRateLabel(data.failRate ?? 0)}
              </Badge>
              <span>fail rate (24h)</span>
            </p>
          </CardContent>
        </Card>
      </section>

      {/* Queue depth bar */}
      <section>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">Queue Depth</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>
                {depthUsed} / {data.maxDepth} used
              </span>
              <span>{depthPct.toFixed(0)}%</span>
            </div>
            <Progress value={depthPct} className="h-2" indicatorClassName={fillColor(depthUsed, data.maxDepth)} />
          </CardContent>
        </Card>
      </section>

      {/* Recent failures */}
      <section>
        <Card>
          <CardHeader>
            <CardTitle className="text-xs font-medium text-muted-foreground">Recent Failures</CardTitle>
          </CardHeader>
          <CardContent>
            {failures.length === 0 ? (
              <p className="py-4 text-center text-xs text-muted-foreground">No recent failures.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="pb-2 pr-4 font-medium">Type</th>
                      <th className="pb-2 pr-4 font-medium">Error</th>
                      <th className="pb-2 pr-4 font-medium">Attempts</th>
                      <th className="pb-2 font-medium">Age</th>
                    </tr>
                  </thead>
                  <tbody>
                    {failures.map((f) => (
                      <tr key={f.id} className="border-b last:border-0">
                        <td className="py-2 pr-4">
                          <Badge variant="outline" className="font-mono text-[10px]">
                            {f.type}
                          </Badge>
                        </td>
                        <td className="max-w-[300px] truncate py-2 pr-4 text-muted-foreground" title={f.error}>
                          {f.error || '—'}
                        </td>
                        <td className="py-2 pr-4 tabular-nums">{f.attempts}</td>
                        <td className="py-2 text-muted-foreground">{f.age}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  )
}
