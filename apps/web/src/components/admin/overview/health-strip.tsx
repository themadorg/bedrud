import { AlertCircle, BadgeCheck, Cable, Shield, Timer, Wifi } from 'lucide-react'
import { Badge } from '#/components/ui/badge'
import { Card } from '#/components/ui/card'
import { Separator } from '#/components/ui/separator'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '#/components/ui/tooltip'
import type { OverviewHealth } from '#/lib/use-admin-overview'

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m`
  return '<1m'
}

interface StatusBadgeProps {
  status: string
  label: string
  icon: React.ElementType
  tooltip?: string
}

function StatusBadge({ status, label, icon: Icon, tooltip }: StatusBadgeProps) {
  const isOk = status === 'healthy' || status === 'connected' || status === 'valid'
  const isWarn = status === 'degraded' || status === 'expiring' || status === 'warning'
  const variant = isOk ? 'secondary' : isWarn ? 'outline' : 'destructive'

  const badge = (
    <Badge variant={variant} className="gap-1.5 px-3 py-1 text-xs font-normal">
      <Icon className={`h-3 w-3 ${isOk ? 'text-success' : isWarn ? 'text-warning' : 'text-destructive'}`} />
      <span>{label}</span>
    </Badge>
  )

  if (tooltip) {
    return (
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>{badge}</TooltipTrigger>
          <TooltipContent side="bottom" className="text-xs">
            {tooltip}
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    )
  }
  return badge
}

export function AdminHealthStrip({ health }: { health: OverviewHealth }) {
  return (
    <Card className="p-3">
      <div className="flex items-center gap-3 overflow-x-auto">
        <StatusBadge
          status={health.status}
          label={health.status === 'healthy' ? 'Healthy' : health.status === 'degraded' ? 'Degraded' : 'Down'}
          icon={BadgeCheck}
          tooltip={health.status === 'healthy' ? 'All systems operational' : undefined}
        />

        <Separator orientation="vertical" className="h-5" />

        {health.tls ? (
          <StatusBadge
            status={health.tls.status}
            label={health.tls.enabled ? `TLS ${health.tls.daysRemaining}d` : 'TLS off'}
            icon={Shield}
            tooltip={health.tls.enabled ? `Certificate expires ${health.tls.expiryDate}` : 'TLS not enabled'}
          />
        ) : (
          <StatusBadge status="unknown" label="TLS" icon={Shield} tooltip="TLS status unknown" />
        )}

        <Separator orientation="vertical" className="h-5" />

        <StatusBadge
          status={health.realtime}
          label={health.realtime === 'connected' ? 'Realtime' : 'Disconnected'}
          icon={Wifi}
          tooltip={health.realtime === 'connected' ? 'LiveKit connected' : 'LiveKit disconnected'}
        />

        <Separator orientation="vertical" className="h-5" />

        <StatusBadge
          status={health.dbStatus}
          label={health.dbStatus === 'connected' ? 'Database' : 'DB error'}
          icon={Cable}
          tooltip={health.dbStatus === 'connected' ? 'Database connected' : undefined}
        />

        <div className="ml-auto flex items-center gap-2 text-xs text-muted-foreground">
          <Timer className="h-3 w-3" />
          <span className="tabular-nums">{formatUptime(health.uptimeSeconds)} uptime</span>
          {health.alertsCount > 0 && (
            <>
              <Separator orientation="vertical" className="h-4" />
              <span className="flex items-center gap-1 text-destructive">
                <AlertCircle className="h-3 w-3" />
                {health.alertsCount} alert{health.alertsCount !== 1 ? 's' : ''}
              </span>
            </>
          )}
        </div>
      </div>
    </Card>
  )
}
