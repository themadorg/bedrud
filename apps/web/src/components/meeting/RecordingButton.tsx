// TODO oncoming feature
import { cn } from '#/lib/utils'

interface RecordingButtonProps {
  isRecording: boolean
  isStarting: boolean
  isStopping: boolean
  onToggle: () => void
  isMobile?: boolean
}

export function btnRecordingCn(active: boolean, isMobile = false) {
  return cn(
    'flex items-center justify-center shrink-0 border-none cursor-pointer transition-[background,color] duration-150',
    isMobile ? 'h-[38px] w-[38px] rounded-[10px]' : 'h-11 w-11 rounded-xl',
    active
      ? 'bg-primary/25 text-teal-400 hover:bg-primary/30 ring-1 ring-primary/30'
      : 'bg-white/[0.07] text-white/75 hover:bg-white/[0.12]',
  )
}

export function RecordingButton(_props: RecordingButtonProps) {
  return null
}
