import { DoorOpen, PlusCircle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import { ScrollArea } from '#/components/ui/scroll-area'
import type { RoomEvent } from '#/lib/use-admin-overview'

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

function eventMeta(event: RoomEvent): { icon: React.ElementType; label: string } {
  if (event.type === 'room_created') return { icon: PlusCircle, label: 'Room created' }
  if (event.type === 'room_joined') return { icon: DoorOpen, label: 'User joined' }
  return { icon: PlusCircle, label: event.type }
}

interface RecentEventsProps {
  events: RoomEvent[]
}

export function AdminRecentEvents({ events }: RecentEventsProps) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">Room events</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <ScrollArea className="max-h-56">
          <div className="divide-y">
            {events.length === 0 ? (
              <p className="px-5 py-6 text-center text-xs text-muted-foreground">No recent events</p>
            ) : (
              events.map((ev, i) => {
                const { icon: Icon, label } = eventMeta(ev)
                return (
                  <div key={i} className="flex items-center gap-3 px-5 py-2.5">
                    <div className="flex h-7 w-7 items-center justify-center bg-primary/10 text-primary">
                      <Icon className="h-3.5 w-3.5" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-xs">
                        <span className="font-medium">{ev.userName ?? ev.roomName}</span>
                        <span className="text-muted-foreground"> {label}</span>
                      </p>
                      {ev.roomName && ev.userName && (
                        <p className="truncate text-[11px] text-muted-foreground">
                          {ev.type === 'room_created' ? ev.roomName : `${ev.userName} → ${ev.roomName}`}
                        </p>
                      )}
                    </div>
                    <span className="shrink-0 text-[10px] text-muted-foreground">{timeAgo(ev.timestamp)}</span>
                  </div>
                )
              })
            )}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  )
}
