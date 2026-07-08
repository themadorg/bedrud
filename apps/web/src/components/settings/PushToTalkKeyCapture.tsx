import { useEffect, useState } from 'react'
import { formatKeyboardCode, isModifierKey } from '#/lib/push-to-talk-key'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface PushToTalkKeyCaptureProps {
  value: string
  onChange: (code: string) => void
  disabled?: boolean
  meeting?: boolean
}

export function PushToTalkKeyCapture({ value, onChange, disabled, meeting }: PushToTalkKeyCaptureProps) {
  const [capturing, setCapturing] = useState(false)

  useEffect(() => {
    if (!capturing) return

    const onKeyDown = (e: KeyboardEvent) => {
      e.preventDefault()
      e.stopPropagation()
      if (isModifierKey(e.code)) return
      onChange(e.code)
      setCapturing(false)
    }

    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [capturing, onChange])

  return (
    <Button
      type="button"
      variant="secondary"
      size="sm"
      disabled={disabled}
      onClick={() => setCapturing(true)}
      className={cn(
        'min-w-[88px] font-mono text-xs',
        meeting && 'border-white/10 bg-white/[0.06] text-white/80 hover:bg-white/10',
        capturing && 'ring-1 ring-primary/40',
      )}
    >
      {capturing ? 'Press a key…' : formatKeyboardCode(value)}
    </Button>
  )
}
