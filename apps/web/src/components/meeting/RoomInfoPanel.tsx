// TODO oncoming feature
import { useFocusTrap } from './useFocusTrap'

interface RoomInfoPanelProps {
  roomId: string
  onClose: () => void
}

/** Shows room metadata. Recordings removed — TODO oncoming feature. */
export function RoomInfoPanel({ roomId, onClose }: RoomInfoPanelProps) {
  const trapRef = useFocusTrap({ enabled: true, onClose })

  return (
    <div
      ref={trapRef}
      className="absolute z-[25] top-[calc(64px+env(safe-area-inset-top,0px))] right-[calc(14px+env(safe-area-inset-right,0px))] w-[340px] max-h-[60vh] overflow-y-auto rounded-xl backdrop-blur-lg transition-all duration-150"
      style={{
        background: 'rgba(12,12,22,0.85)',
        border: '1px solid rgba(255,255,255,0.08)',
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 pt-3 pb-2">
        <h2 className="text-[11px] font-semibold uppercase tracking-widest text-white/50">Room Info</h2>
        <button
          type="button"
          onClick={onClose}
          className="text-[10px] text-white/50 hover:text-white/70 cursor-pointer bg-white/[0.06] hover:bg-white/[0.1] rounded-lg px-2 py-1 transition-colors"
          aria-label="Close room info"
        >
          Close
        </button>
      </div>

      <div className="px-4 pb-4 space-y-4">
        {/* Room ID */}
        <div>
          <p className="text-[10px] font-medium uppercase tracking-wider text-white/50">Room ID</p>
          <p className="mt-0.5 font-mono text-[12px] text-white/70 break-all">{roomId}</p>
        </div>

        {/* TODO oncoming feature — recordings section removed */}
      </div>
    </div>
  )
}
