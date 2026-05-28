// TODO oncoming feature
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Archive, ArrowLeft, Clock, Film } from 'lucide-react'

export const Route = createFileRoute('/dashboard/archived_/$roomId')({
  head: () => ({ meta: [{ title: 'Archived Room — Bedrud' }] }),
  component: ArchivedRoomDetailPage,
})

function ArchivedRoomDetailPage() {
  const { roomId } = Route.useParams()
  const navigate = useNavigate()

  // Get room name from roomId
  const roomName = roomId
  const createdAt = null

  return (
    <div className="mx-auto max-w-4xl space-y-6 px-4">
      {/* Back + header */}
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={() => navigate({ to: '/dashboard' })}
          className="p-2 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl font-bold tracking-tight font-mono truncate">{roomName}</h1>
          <div className="flex items-center gap-3 text-xs text-muted-foreground mt-0.5">
            <span className="inline-flex items-center gap-1">
              <Archive className="h-3 w-3" />
              Archived room
            </span>
            {createdAt && (
              <span className="inline-flex items-center gap-1">
                <Clock className="h-3 w-3" />
                Created {new Date(createdAt).toLocaleDateString()}
              </span>
            )}
            {/* TODO oncoming feature — recording count removed */}
          </div>
        </div>
      </div>

      {/* TODO oncoming feature — recordings loading/error/empty/list/clear removed */}
      <div className="flex flex-col items-center gap-3 py-16 text-center">
        <div className="rounded-full bg-muted p-3">
          <Film className="h-6 w-6 text-muted-foreground opacity-40" />
        </div>
        <p className="text-sm font-medium">Archived room</p>
        <p className="text-xs text-muted-foreground max-w-md">Recording details coming in a future release.</p>
        <Link
          to="/dashboard"
          className="text-xs text-muted-foreground hover:text-foreground underline-offset-4 hover:underline"
        >
          ← Back to dashboard
        </Link>
      </div>

      {/* TODO oncoming feature — Footer removed */}
    </div>
  )
}
