import { DoorOpen, PlusCircle } from 'lucide-react'
import type { AdminRoomEvent } from '#/lib/use-admin-overview'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

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

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function eventMeta(type: string): { icon: React.ElementType; label: string; color: string } {
  if (type === 'room_created')
    return { icon: PlusCircle, label: 'Created', color: 'text-emerald-500 bg-emerald-500/10' }
  if (type === 'room_joined') return { icon: DoorOpen, label: 'Joined', color: 'text-blue-500 bg-blue-500/10' }
  return { icon: PlusCircle, label: type, color: 'text-muted-foreground bg-muted' }
}

interface RoomEventsTableProps {
  events: AdminRoomEvent[]
  isLoading: boolean
}

export function RoomEventsTable({ events, isLoading }: RoomEventsTableProps) {
  if (isLoading) {
    return (
      <div className="border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="h-9 px-3 text-[11px] font-medium">Event</TableHead>
              <TableHead className="h-9 px-3 text-[11px] font-medium">Room</TableHead>
              <TableHead className="h-9 px-3 text-[11px] font-medium">User</TableHead>
              <TableHead className="h-9 px-3 text-[11px] font-medium">Time</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {Array.from({ length: 8 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell className="px-3 py-2.5">
                  <Skeleton className="h-5 w-20" />
                </TableCell>
                <TableCell className="px-3 py-2.5">
                  <Skeleton className="h-5 w-32" />
                </TableCell>
                <TableCell className="px-3 py-2.5">
                  <Skeleton className="h-5 w-24" />
                </TableCell>
                <TableCell className="px-3 py-2.5">
                  <Skeleton className="h-5 w-20" />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    )
  }

  if (events.length === 0) {
    return <div className="border px-5 py-12 text-center text-xs text-muted-foreground">No room events found</div>
  }

  return (
    <div className="border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="h-9 px-3 text-[11px] font-medium">Event</TableHead>
            <TableHead className="h-9 px-3 text-[11px] font-medium">Room</TableHead>
            <TableHead className="h-9 px-3 text-[11px] font-medium">User</TableHead>
            <TableHead className="h-9 px-3 text-[11px] font-medium">Time</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {events.map((ev, i) => {
            const { icon: Icon, label, color } = eventMeta(ev.type)
            return (
              <TableRow key={`${ev.roomId}-${ev.userId}-${ev.timestamp}-${i}`} className="group">
                <TableCell className="px-3 py-2.5">
                  <div className="flex items-center gap-2">
                    <div className={`flex h-7 w-7 items-center justify-center ${color}`}>
                      <Icon className="h-3.5 w-3.5" />
                    </div>
                    <Badge variant="outline" className="text-[10px]">
                      {label}
                    </Badge>
                  </div>
                </TableCell>
                <TableCell className="px-3 py-2.5">
                  <span className="text-xs font-medium">{ev.roomName || '—'}</span>
                </TableCell>
                <TableCell className="px-3 py-2.5">
                  <span className="text-xs text-muted-foreground">{ev.userName || '—'}</span>
                </TableCell>
                <TableCell className="px-3 py-2.5">
                  <div className="flex flex-col">
                    <span className="text-xs tabular-nums text-muted-foreground">{timeAgo(ev.timestamp)}</span>
                    <span className="text-[10px] text-muted-foreground/60">{formatDate(ev.timestamp)}</span>
                  </div>
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
