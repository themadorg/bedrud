import { GripVertical, Plus, X } from 'lucide-react'
import { type DragEvent, useCallback, useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import { createPollOptionId, reorderItems } from './pollOptionReorder'

interface PollOptionDraft {
  id: string
  text: string
}

function emptyOptions(): PollOptionDraft[] {
  return [
    { id: createPollOptionId(), text: '' },
    { id: createPollOptionId(), text: '' },
  ]
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreate: (question: string, options: string[]) => void
  disabled?: boolean
}

export function ChatPollComposer({ open, onOpenChange, onCreate, disabled }: Props) {
  const [question, setQuestion] = useState('')
  const [options, setOptions] = useState<PollOptionDraft[]>(emptyOptions)
  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const [dropIndex, setDropIndex] = useState<number | null>(null)

  useEffect(() => {
    if (!open) {
      setQuestion('')
      setOptions(emptyOptions())
      setDragIndex(null)
      setDropIndex(null)
    }
  }, [open])

  const updateOption = useCallback((index: number, value: string) => {
    setOptions((prev) => prev.map((opt, i) => (i === index ? { ...opt, text: value } : opt)))
  }, [])

  const addOption = useCallback(() => {
    setOptions((prev) => (prev.length < 6 ? [...prev, { id: createPollOptionId(), text: '' }] : prev))
  }, [])

  const removeOption = useCallback((index: number) => {
    setOptions((prev) => (prev.length > 2 ? prev.filter((_, i) => i !== index) : prev))
  }, [])

  const reorderOption = useCallback((from: number, to: number) => {
    setOptions((prev) => reorderItems(prev, from, to))
  }, [])

  const handleDragStart = useCallback(
    (index: number) => (e: DragEvent<HTMLButtonElement>) => {
      if (disabled) return
      e.dataTransfer.effectAllowed = 'move'
      e.dataTransfer.setData('text/plain', String(index))
      setDragIndex(index)
      setDropIndex(index)
    },
    [disabled],
  )

  const handleDragOver = useCallback(
    (index: number) => (e: DragEvent<HTMLLIElement>) => {
      if (disabled || dragIndex === null) return
      e.preventDefault()
      e.dataTransfer.dropEffect = 'move'
      setDropIndex(index)
    },
    [disabled, dragIndex],
  )

  const handleDrop = useCallback(
    (index: number) => (e: DragEvent<HTMLLIElement>) => {
      e.preventDefault()
      if (disabled || dragIndex === null) return
      reorderOption(dragIndex, index)
      setDragIndex(null)
      setDropIndex(null)
    },
    [disabled, dragIndex, reorderOption],
  )

  const handleDragEnd = useCallback(() => {
    setDragIndex(null)
    setDropIndex(null)
  }, [])

  const submit = useCallback(() => {
    const q = question.trim()
    const opts = options.map((o) => o.text.trim()).filter(Boolean)
    if (!q || opts.length < 2) return
    onCreate(q, opts)
    onOpenChange(false)
  }, [question, options, onCreate, onOpenChange])

  const canCreate = Boolean(question.trim()) && options.filter((o) => o.text.trim()).length >= 2

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="meet-dialog max-w-[min(360px,calc(var(--app-width,100svw)-2rem))] gap-0 p-0 shadow-2xl">
        <DialogHeader className="border-b border-white/[0.08] px-4 py-3">
          <DialogTitle className="text-[15px] font-semibold text-white/90">Create poll</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-3 px-4 py-4">
          <Input
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="Ask a question…"
            disabled={disabled}
            className="h-9 rounded-lg border border-white/[0.09] border-b-white/[0.09] bg-white/[0.06] px-2.5 text-[13px] leading-normal text-white/90 placeholder:text-white/35"
          />

          <ul className="m-0 flex list-none flex-col gap-1.5 p-0" aria-label="Poll options">
            {options.map((opt, i) => (
              <li
                key={opt.id}
                onDragOver={handleDragOver(i)}
                onDrop={handleDrop(i)}
                className={cn(
                  'flex items-center gap-1.5 rounded-lg transition-colors',
                  dropIndex === i && dragIndex !== null && dragIndex !== i && 'bg-white/[0.04]',
                  dragIndex === i && 'opacity-60',
                )}
              >
                <button
                  type="button"
                  draggable={!disabled}
                  onDragStart={handleDragStart(i)}
                  onDragEnd={handleDragEnd}
                  disabled={disabled}
                  className={cn(
                    'flex h-8 w-7 shrink-0 cursor-grab items-center justify-center rounded-lg border border-white/[0.09] bg-white/[0.04] text-white/40 active:cursor-grabbing disabled:cursor-default disabled:opacity-40',
                    !disabled && 'hover:text-accent-400/90',
                  )}
                  aria-label={`Reorder option ${i + 1}`}
                >
                  <GripVertical size={14} />
                </button>
                <Input
                  value={opt.text}
                  onChange={(e) => updateOption(i, e.target.value)}
                  placeholder={`Option ${i + 1}`}
                  disabled={disabled}
                  className="h-8 flex-1 rounded-lg border border-white/[0.09] border-b-white/[0.09] bg-white/[0.06] px-2.5 text-[13px] leading-normal text-white/90 placeholder:text-white/35"
                />
                {options.length > 2 && (
                  <button
                    type="button"
                    onClick={() => removeOption(i)}
                    className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-white/[0.09] bg-white/[0.04] text-white/50 hover:text-white/80"
                    aria-label={`Remove option ${i + 1}`}
                  >
                    <X size={12} />
                  </button>
                )}
              </li>
            ))}
          </ul>

          {options.length < 6 && (
            <button
              type="button"
              onClick={addOption}
              disabled={disabled}
              className="flex items-center gap-1 self-start border-none bg-transparent p-0 text-[11px] text-accent-400/90 hover:text-accent-400 disabled:opacity-40"
            >
              <Plus size={12} />
              Add option
            </button>
          )}
        </div>

        <DialogFooter className="border-t border-white/[0.08] px-4 py-3 sm:justify-end">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onOpenChange(false)}
            disabled={disabled}
            className="h-8 text-white/60 hover:bg-white/10 hover:text-white/90"
          >
            Cancel
          </Button>
          <Button type="button" size="sm" onClick={submit} disabled={disabled || !canCreate} className="h-8">
            Send poll
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
