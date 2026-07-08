import { useIsSpeaking, useParticipants } from '@livekit/components-react'
import type { Participant } from 'livekit-client'
import { Users, X } from 'lucide-react'
import { ParticipantTile } from '@/components/meeting/ParticipantTile'
import { cn } from '@/lib/utils'
import { useFocusTrap } from './useFocusTrap'

interface Props {
  stackOffset?: string
  onClose: () => void
}

function SidebarTile({
  participant,
  index,
  totalCount,
}: {
  participant: Participant
  index: number
  totalCount: number
}) {
  const isSpeaking = useIsSpeaking(participant)

  return (
    <div
      className={cn(
        'relative aspect-video w-full shrink-0 overflow-hidden rounded-[10px] transition-[box-shadow] duration-200',
        isSpeaking &&
          'shadow-[0_0_0_1.5px_color-mix(in_oklab,var(--primary)_75%,transparent),0_0_14px_color-mix(in_oklab,var(--primary)_30%,transparent)]',
      )}
    >
      <ParticipantTile participant={participant} totalCount={totalCount} index={index} />
    </div>
  )
}

export function ParticipantVideoSidebar({ stackOffset, onClose }: Props) {
  const participants = useParticipants()
  const totalCount = participants.length
  const trapRef = useFocusTrap({ enabled: true, onClose })

  return (
    <aside
      ref={trapRef}
      className={cn(
        'fixed top-0 bottom-0 z-[6] flex flex-col border-l border-white/[0.07] bg-[#0a0a16]/94 pt-[env(safe-area-inset-top)] pb-[calc(env(safe-area-inset-bottom))] backdrop-blur-2xl transition-[right] duration-200',
        !stackOffset && 'right-0',
      )}
      style={{ width: 'min(288px, 100vw)', ...(stackOffset ? { right: stackOffset } : {}) }}
    >
      <div className="flex h-[52px] shrink-0 items-center justify-between border-b border-white/[0.06] px-4">
        <div className="flex items-center gap-[7px]">
          <Users size={14} className="text-[color-mix(in_oklab,var(--accent-400)_70%,transparent)]" />
          <span className="text-[13px] font-semibold text-white/80">Videos</span>
          <span
            className="rounded-md px-[6px] py-px text-[11px] font-semibold"
            style={{
              background: 'color-mix(in oklab, var(--primary) 18%, transparent)',
              border: '1px solid color-mix(in oklab, var(--primary) 25%, transparent)',
              color: 'color-mix(in oklab, var(--accent-400) 80%, transparent)',
            }}
          >
            {totalCount}
          </span>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="flex h-7 w-7 cursor-pointer items-center justify-center rounded-[7px] border-none bg-transparent text-white/50 transition-[background,color] duration-150"
          aria-label="Close video panel"
        >
          <X size={15} />
        </button>
      </div>
      <div className="meet-scroll flex flex-1 flex-col gap-2 overflow-y-auto p-2">
        {participants.map((p, i) => (
          <SidebarTile key={p.identity} participant={p} index={i} totalCount={totalCount} />
        ))}
      </div>
    </aside>
  )
}
