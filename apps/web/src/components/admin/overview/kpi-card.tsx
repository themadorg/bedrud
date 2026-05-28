import { TrendingDown, TrendingUp } from 'lucide-react'
import type { ReactElement } from 'react'
import { Badge } from '#/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import type { KpiEntry } from '#/lib/use-admin-overview'

interface KpiCardProps {
  title: string
  entry: KpiEntry
  icon: ReactElement
}

export function KpiCard({ title, entry, icon }: KpiCardProps) {
  const hasDelta = entry.delta !== undefined && entry.delta !== 0
  const isPositive = entry.delta !== undefined && entry.delta > 0

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle>
        <div className="text-muted-foreground">{icon}</div>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-semibold tabular-nums">{entry.value}</div>
        {hasDelta && (
          <div className="mt-1 flex items-center gap-2 text-xs">
            <Badge variant="secondary" className="gap-1 px-1.5 py-0">
              {isPositive ? <TrendingUp className="h-3 w-3" /> : <TrendingDown className="h-3 w-3" />}
              <span className="tabular-nums">
                {isPositive ? '+' : ''}
                {entry.delta}
                {entry.deltaPercent ? ` (${entry.deltaPercent}%)` : ''}
              </span>
            </Badge>
            <span className="text-muted-foreground">{entry.deltaLabel ?? ''}</span>
          </div>
        )}
        {entry.activeNow !== undefined && entry.activeNow !== entry.value && (
          <p className="mt-0.5 text-xs text-muted-foreground">{entry.activeNow} active</p>
        )}
      </CardContent>
    </Card>
  )
}
