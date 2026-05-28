import { Avatar, AvatarFallback } from '#/components/ui/avatar'
import { Badge } from '#/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '#/components/ui/card'
import { ScrollArea } from '#/components/ui/scroll-area'
import type { RecentUser } from '#/lib/use-admin-overview'
import { Skeleton } from '@/components/ui/skeleton'

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

interface RecentSignupsProps {
  users: RecentUser[]
  isLoading?: boolean
}

export function AdminRecentSignups({ users, isLoading }: RecentSignupsProps) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium">Recent sign-ups</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <ScrollArea className="max-h-56">
          <div className="divide-y">
            {isLoading ? (
              Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="flex items-center gap-3 px-5 py-2.5">
                  <Skeleton className="h-7 w-7 rounded-full" />
                  <div className="min-w-0 flex-1 space-y-1">
                    <Skeleton className="h-3 w-24" />
                    <Skeleton className="h-2.5 w-32" />
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Skeleton className="h-4 w-10" />
                    <Skeleton className="h-3 w-8" />
                  </div>
                </div>
              ))
            ) : users.length === 0 ? (
              <p className="px-5 py-6 text-center text-xs text-muted-foreground">No recent sign-ups</p>
            ) : (
              users.map((u) => (
                <div key={u.id} className="flex items-center gap-3 px-5 py-2.5">
                  <Avatar className="h-7 w-7">
                    <AvatarFallback className="bg-primary/10 text-[10px] font-medium text-primary">
                      {initials(u.name)}
                    </AvatarFallback>
                  </Avatar>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-xs font-medium">{u.name}</p>
                    <p className="truncate text-[11px] text-muted-foreground">{u.email}</p>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Badge variant="outline" className="text-[10px]">
                      {u.provider}
                    </Badge>
                    <span className="text-[10px] text-muted-foreground">{timeAgo(u.createdAt)}</span>
                  </div>
                </div>
              ))
            )}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  )
}
