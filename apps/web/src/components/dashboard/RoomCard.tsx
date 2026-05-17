import {
  ArrowRight,
  Check,
  Copy,
  Globe,
  Lock,
  MessageSquare,
  Mic,
  Settings2,
  ShieldCheck,
  Trash2,
  UserCheck,
  Users,
  Video,
} from 'lucide-react'
import { useState } from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

interface Room {
  id: string
  name: string
  isPublic: boolean
  maxParticipants: number
  isActive: boolean
  settings: {
    allowChat: boolean
    allowVideo: boolean
    allowAudio: boolean
    requireApproval: boolean
    e2ee?: boolean
  }
}

interface Props {
  room: Room
  onJoin: () => void
  onDelete?: () => void
  onSettings?: () => void
}

export function RoomCard({ room, onJoin, onDelete, onSettings }: Props) {
  const [copied, setCopied] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const capabilities = [
    room.settings.allowAudio ? { icon: Mic, label: 'Audio' } : null,
    room.settings.allowVideo ? { icon: Video, label: 'Video' } : null,
    room.settings.allowChat ? { icon: MessageSquare, label: 'Chat' } : null,
  ].filter((item): item is { icon: typeof Mic; label: string } => Boolean(item))

  function copyLink() {
    void navigator.clipboard.writeText(`${window.location.origin}/m/${room.name}`)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <Card className="group flex h-full flex-col p-4 transition-all hover:-translate-y-0.5 hover:border-primary/20">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            {room.isActive && (
              <Badge
                variant="outline"
                className="border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 gap-1"
              >
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                Live
              </Badge>
            )}
            <Badge variant="outline" className="gap-1">
              {room.isPublic ? <Globe className="h-3 w-3" /> : <Lock className="h-3 w-3" />}
              {room.isPublic ? 'Public' : 'Private'}
            </Badge>
            {room.settings.e2ee && (
              <Badge className="gap-1">
                <ShieldCheck className="h-3 w-3" />
                Encrypted
              </Badge>
            )}
            {room.settings.requireApproval && (
              <Badge variant="outline" className="gap-1">
                <UserCheck className="h-3 w-3" />
                Approval
              </Badge>
            )}
          </div>

          <h3 className="mt-3 truncate font-mono text-sm font-semibold">{room.name}</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            {room.isActive ? 'Participants can join immediately.' : 'Ready for the next session.'}
          </p>
        </div>

        <Button
          variant="outline"
          size="icon"
          onClick={copyLink}
          aria-label="Copy link"
          title={copied ? 'Copied!' : 'Copy invite link'}
        >
          {copied ? <Check className="h-4 w-4 text-emerald-500" /> : <Copy className="h-4 w-4 text-muted-foreground" />}
        </Button>
      </div>

      <CardContent className="mt-4 grid gap-2 p-0 sm:grid-cols-2">
        <div className="border bg-background/70 p-3">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Capacity</p>
          <p className="mt-2 flex items-center gap-2 text-sm font-medium">
            <Users className="h-4 w-4 text-muted-foreground" />
            {room.maxParticipants} seats
          </p>
        </div>

        <div className="border bg-background/70 p-3">
          <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Access</p>
          <p className="mt-2 text-sm font-medium">
            {room.isPublic ? 'Anyone with the link can join.' : 'Only invited participants can enter.'}
          </p>
        </div>
      </CardContent>

      <CardContent className="mt-4 border bg-background/70 p-3">
        <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Enabled</p>
        <div className="mt-2 flex flex-wrap gap-2">
          {capabilities.length > 0 ? (
            capabilities.map(({ icon: Icon, label }) => (
              <Badge key={label} variant="outline" className="gap-1">
                <Icon className="h-3.5 w-3.5" />
                {label}
              </Badge>
            ))
          ) : (
            <p className="text-xs text-muted-foreground">No participant features enabled.</p>
          )}
        </div>
      </CardContent>

      <div className="mt-4 flex items-center gap-2">
        <Button variant={room.isActive ? 'default' : 'outline'} onClick={onJoin} className="flex-1">
          {room.isActive ? 'Join live room' : 'Open room'}
          <ArrowRight className="h-4 w-4" />
        </Button>

        {onSettings && (
          <Button variant="outline" size="icon" onClick={onSettings} aria-label="Room settings" title="Room settings">
            <Settings2 className="h-4 w-4 text-muted-foreground" />
          </Button>
        )}

        {onDelete && !confirmDelete && (
          <Button
            variant="outline"
            size="icon"
            onClick={() => setConfirmDelete(true)}
            className="border-destructive/30 bg-destructive/10 hover:bg-destructive/15"
            aria-label="Delete room"
            title="Delete room"
          >
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        )}
      </div>

      {confirmDelete && onDelete && (
        <div className="mt-3 flex flex-wrap items-center justify-between gap-3 border border-destructive/30 bg-destructive/10 px-3 py-3">
          <div>
            <p className="text-sm font-medium text-destructive">Delete this room?</p>
            <p className="text-xs text-destructive/80">This removes the room from the dashboard.</p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={() => {
                onDelete()
                setConfirmDelete(false)
              }}
            >
              Delete
            </Button>
          </div>
        </div>
      )}
    </Card>
  )
}
