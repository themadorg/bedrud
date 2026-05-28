import { Badge } from '#/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import type { RoomComposition } from '#/lib/use-admin-overview'

interface RoomCompositionProps {
  data: RoomComposition
}

export function AdminRoomComposition({ data }: RoomCompositionProps) {
  const total = data.live + data.public + data.private + data.persistent

  const segments: { label: string; value: number; color: string }[] = [
    { label: 'Live', value: data.live, color: 'bg-success' },
    { label: 'Public', value: data.public, color: 'bg-primary' },
    { label: 'Private', value: data.private, color: 'bg-muted-foreground' },
    { label: 'Persistent', value: data.persistent, color: 'bg-accent' },
  ]

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">Room breakdown</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Segmented bar */}
        <div className="flex h-2 overflow-hidden rounded-sm bg-muted">
          {segments.map((s) =>
            s.value > 0 ? (
              <div key={s.label} className={s.color} style={{ width: `${total > 0 ? (s.value / total) * 100 : 0}%` }} />
            ) : null,
          )}
        </div>

        {/* Legend */}
        <div className="space-y-2">
          {segments.map((s) => (
            <div key={s.label} className="flex items-center justify-between text-xs">
              <div className="flex items-center gap-2">
                <div className={`h-2 w-2 rounded-sm ${s.color}`} />
                <span className="text-muted-foreground">{s.label}</span>
              </div>
              <span className="tabular-nums font-medium">{s.value}</span>
            </div>
          ))}
          {data.stale > 0 && (
            <div className="flex items-center justify-between border-t pt-2 text-xs">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-sm bg-destructive" />
                <span className="text-muted-foreground">Stale (48h+)</span>
              </div>
              <Badge variant="outline" className="tabular-nums text-xs">
                {data.stale}
              </Badge>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
