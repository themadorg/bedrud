import { Avatar, AvatarFallback } from '#/components/ui/avatar'
import { Badge } from '#/components/ui/badge'
import type { RecentUser } from '#/lib/use-admin-overview'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function initials(name: string): string {
  return name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

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

interface RecentSignupsTableProps {
  users: RecentUser[]
  isLoading: boolean
}

export function RecentSignupsTable({ users, isLoading }: RecentSignupsTableProps) {
  if (isLoading) {
    return (
      <div className="border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="h-9 px-3 text-[11px] font-medium">User</TableHead>
              <TableHead className="h-9 px-3 text-[11px] font-medium">Email</TableHead>
              <TableHead className="h-9 px-3 text-[11px] font-medium">Provider</TableHead>
              <TableHead className="h-9 px-3 text-[11px] font-medium">Signed up</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {Array.from({ length: 8 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell className="px-3 py-2.5">
                  <Skeleton className="h-5 w-32" />
                </TableCell>
                <TableCell className="px-3 py-2.5">
                  <Skeleton className="h-5 w-40" />
                </TableCell>
                <TableCell className="px-3 py-2.5">
                  <Skeleton className="h-5 w-16" />
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

  if (users.length === 0) {
    return <div className="border px-5 py-12 text-center text-xs text-muted-foreground">No sign-ups found</div>
  }

  return (
    <div className="border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="h-9 px-3 text-[11px] font-medium">User</TableHead>
            <TableHead className="h-9 px-3 text-[11px] font-medium">Email</TableHead>
            <TableHead className="h-9 px-3 text-[11px] font-medium">Provider</TableHead>
            <TableHead className="h-9 px-3 text-[11px] font-medium">Signed up</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((u) => (
            <TableRow key={u.id} className="group">
              <TableCell className="px-3 py-2.5">
                <div className="flex items-center gap-2.5">
                  <Avatar className="h-7 w-7">
                    <AvatarFallback className="bg-primary/10 text-[10px] font-medium text-primary">
                      {initials(u.name)}
                    </AvatarFallback>
                  </Avatar>
                  <span className="text-xs font-medium">{u.name}</span>
                </div>
              </TableCell>
              <TableCell className="px-3 py-2.5">
                <span className="text-xs text-muted-foreground">{u.email}</span>
              </TableCell>
              <TableCell className="px-3 py-2.5">
                <Badge variant="outline" className="text-[10px]">
                  {u.provider}
                </Badge>
              </TableCell>
              <TableCell className="px-3 py-2.5">
                <div className="flex flex-col">
                  <span className="text-xs tabular-nums text-muted-foreground">{timeAgo(u.createdAt)}</span>
                  <span className="text-[10px] text-muted-foreground/60">{formatDate(u.createdAt)}</span>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
