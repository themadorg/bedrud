import { useRoomContext } from '@livekit/components-react'
import { useNavigate } from '@tanstack/react-router'
import { type CSSProperties, type MouseEvent, type ReactNode, useState } from 'react'
import { toast } from 'sonner'
import { AlertConfirmDialog } from '@/components/admin/AlertConfirmDialog'
import { useMeetingRoomContext } from '../MeetingContext'
import { resolveChatLink } from './chatLinkTarget'

interface ChatLinkProps {
  href: string
  style: CSSProperties
  children?: ReactNode
}

/**
 * A chat link. A link to another room on this server leaves the call and joins that room in this tab,
 * after asking, instead of opening a second call in a new tab.
 */
export function ChatLink({ href, style, children }: ChatLinkProps) {
  const room = useRoomContext()
  const { roomName } = useMeetingRoomContext()
  const navigate = useNavigate()
  const [pendingRoom, setPendingRoom] = useState<string | null>(null)

  function handleClick(event: MouseEvent<HTMLAnchorElement>) {
    // A modified click is the reader asking for a new tab on purpose.
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) return
    const target = resolveChatLink(href, window.location.origin, roomName)
    if (target.kind === 'page') return
    event.preventDefault()
    if (target.kind === 'currentRoom') toast("You're already in this room")
    else setPendingRoom(target.roomName)
  }

  function leaveAndJoin() {
    if (!pendingRoom) return
    const meetId = pendingRoom
    setPendingRoom(null)
    room.disconnect()
    navigate({ to: '/m/$meetId', params: { meetId } })
  }

  return (
    <>
      <a href={href} target="_blank" rel="noopener noreferrer" style={style} onClick={handleClick}>
        {children}
      </a>
      <AlertConfirmDialog
        open={pendingRoom !== null}
        onOpenChange={(open) => !open && setPendingRoom(null)}
        title="Leave this meeting?"
        description={`You'll leave ${roomName} and join ${pendingRoom ?? ''}.`}
        confirmLabel="Leave and join"
        onConfirm={leaveAndJoin}
        variant="default"
      />
    </>
  )
}
