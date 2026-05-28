import { AlertCircle, CheckCircle2, Info, ShieldAlert } from 'lucide-react'
import { Badge } from '#/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import type { AttentionItem } from '#/lib/use-admin-overview'

interface NeedsAttentionProps {
  items: AttentionItem[]
}

const severityConfig = {
  error: { icon: ShieldAlert, class: 'text-destructive', label: 'Error' },
  warning: { icon: AlertCircle, class: 'text-warning', label: 'Warning' },
  info: { icon: Info, class: 'text-muted-foreground', label: 'Info' },
}

export function AdminNeedsAttention({ items }: NeedsAttentionProps) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">Needs attention</CardTitle>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <div className="flex items-center gap-2 py-6 text-xs text-muted-foreground">
            <CheckCircle2 className="h-4 w-4 text-success" />
            <span>All clear</span>
          </div>
        ) : (
          <div className="space-y-2">
            {items.map((item, i) => {
              const cfg = severityConfig[item.severity] ?? severityConfig.info
              const Icon = cfg.icon
              return (
                <div key={i} className="flex items-start gap-2 rounded-sm border p-2 text-xs">
                  <Icon className={`mt-0.5 h-3.5 w-3.5 shrink-0 ${cfg.class}`} />
                  <div className="min-w-0 flex-1">
                    <p>{item.message}</p>
                    {item.daysLeft !== undefined && (
                      <Badge variant="outline" className="mt-1 text-[10px]">
                        {item.daysLeft} days
                      </Badge>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
