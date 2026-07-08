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
import { Card } from '@/components/ui/card'

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

  const capacityLabel = room.maxParticipants > 0 ? `${room.maxParticipants}` : 'Open'

  return (
    <Card className="group flex flex-col gap-2.5 p-3 transition-colors hover:border-primary/25">
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1">
            {room.isActive && (
              <Badge
                variant="outline"
                className="h-5 gap-1 border-emerald-500/30 bg-emerald-500/10 px-1.5 text-[10px] text-emerald-600 dark:text-emerald-400"
              >
                <span className="h-1 w-1 rounded-full bg-emerald-500" />
                Live
              </Badge>
            )}
            <Badge variant="outline" className="h-5 gap-1 px-1.5 text-[10px]">
              {room.isPublic ? <Globe className="h-2.5 w-2.5" /> : <Lock className="h-2.5 w-2.5" />}
              {room.isPublic ? 'Public' : 'Private'}
            </Badge>
            {room.settings.e2ee && (
              <Badge className="h-5 gap-1 px-1.5 text-[10px]">
                <ShieldCheck className="h-2.5 w-2.5" />
                E2EE
              </Badge>
            )}
            {room.settings.requireApproval && (
              <Badge variant="outline" className="h-5 gap-1 px-1.5 text-[10px]">
                <UserCheck className="h-2.5 w-2.5" />
                Approval
              </Badge>
            )}
          </div>
          <h3 className="mt-1.5 truncate font-mono text-[13px] font-semibold leading-tight">{room.name}</h3>
        </div>

        <Button
          variant="ghost"
          size="icon"
          onClick={copyLink}
          className="h-7 w-7 shrink-0"
          aria-label="Copy link"
          title={copied ? 'Copied!' : 'Copy invite link'}
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-emerald-500" />
          ) : (
            <Copy className="h-3.5 w-3.5 text-muted-foreground" />
          )}
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[11px] text-muted-foreground">
        <span className="inline-flex items-center gap-1">
          <Users className="h-3 w-3 shrink-0" />
          {capacityLabel}
        </span>
        {capabilities.map(({ icon: Icon, label }) => (
          <span key={label} className="inline-flex items-center gap-0.5" title={label}>
            <Icon className="h-3 w-3 shrink-0" />
            {label}
          </span>
        ))}
      </div>

      <div className="flex items-center gap-1.5 pt-0.5">
        <Button
          variant={room.isActive ? 'default' : 'outline'}
          size="sm"
          onClick={onJoin}
          className="h-8 flex-1 gap-1.5 text-xs"
        >
          {room.isActive ? 'Join' : 'Open'}
          <ArrowRight className="h-3.5 w-3.5" />
        </Button>

        {onSettings && (
          <Button
            variant="outline"
            size="icon"
            onClick={onSettings}
            className="h-8 w-8"
            aria-label="Room settings"
            title="Room settings"
          >
            <Settings2 className="h-3.5 w-3.5 text-muted-foreground" />
          </Button>
        )}

        {onDelete && !confirmDelete && (
          <Button
            variant="outline"
            size="icon"
            onClick={() => setConfirmDelete(true)}
            className="h-8 w-8 border-destructive/30 bg-destructive/10 hover:bg-destructive/15"
            aria-label="Delete room"
            title="Delete room"
          >
            <Trash2 className="h-3.5 w-3.5 text-destructive" />
          </Button>
        )}
      </div>

      {confirmDelete && onDelete && (
        <div className="flex items-center justify-between gap-2 border border-destructive/30 bg-destructive/10 px-2 py-2">
          <p className="text-[11px] font-medium text-destructive">Delete room?</p>
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => setConfirmDelete(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              size="sm"
              className="h-7 px-2 text-xs"
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
