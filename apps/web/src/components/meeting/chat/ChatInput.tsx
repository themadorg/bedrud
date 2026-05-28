import { Image, Maximize2, Minimize2, Send } from 'lucide-react'
import {
  type ChangeEvent,
  type ClipboardEvent,
  forwardRef,
  type KeyboardEvent,
  useCallback,
  useImperativeHandle,
  useRef,
  useState,
} from 'react'
import type { ChatAttachment } from '../MeetingContext'

const LINE_HEIGHT = 20
const MIN_ROWS = 1
const NORMAL_MAX_ROWS = 4
const EXPANDED_MAX_ROWS = 10

interface Props {
  onSend: (text: string, attachments?: ChatAttachment[]) => void
  onUpload: (file: File) => Promise<ChatAttachment>
  disabled?: boolean
}

export interface ChatInputHandle {
  focus: () => void
}

export const ChatInput = forwardRef<ChatInputHandle, Props>(function ChatInput({ onSend, onUpload, disabled }, ref) {
  const [draft, setDraft] = useState('')
  const [expanded, setExpanded] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  useImperativeHandle(ref, () => ({ focus: () => textareaRef.current?.focus() }))

  const maxRows = expanded ? EXPANDED_MAX_ROWS : NORMAL_MAX_ROWS
  const minHeight = MIN_ROWS * LINE_HEIGHT + 16
  const maxHeight = maxRows * LINE_HEIGHT + 16

  // Auto-resize textarea on draft change
  const resizeTextarea = useCallback(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, maxHeight)}px`
  }, [maxHeight])

  const send = useCallback(() => {
    const text = draft.trim()
    if (!text || disabled || uploading) return
    onSend(text)
    setDraft('')
    setExpanded(false)
    if (textareaRef.current) textareaRef.current.style.height = `${minHeight}px`
  }, [draft, disabled, uploading, onSend, minHeight])

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if ((e.key === 'Enter' && !e.shiftKey) || (e.key === 'Enter' && e.ctrlKey)) {
        e.preventDefault()
        send()
      }
    },
    [send],
  )

  const uploadFile = useCallback(
    async (file: File) => {
      setError(null)
      setUploading(true)
      try {
        const attachment = await onUpload(file)
        onSend(draft.trim(), [attachment])
        setDraft('')
        setExpanded(false)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Upload failed')
      } finally {
        setUploading(false)
      }
    },
    [draft, onSend, onUpload],
  )

  const handlePaste = useCallback(
    (e: ClipboardEvent<HTMLTextAreaElement>) => {
      const items = Array.from(e.clipboardData.items)
      const imageItem = items.find((item) => item.kind === 'file' && item.type.startsWith('image/'))
      if (!imageItem) return
      e.preventDefault()
      const file = imageItem.getAsFile()
      if (file) void uploadFile(file)
    },
    [uploadFile],
  )

  const handleFileChange = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0]
      if (file) void uploadFile(file)
      e.target.value = ''
    },
    [uploadFile],
  )

  const canSend = Boolean(draft.trim()) && !uploading && !disabled

  return (
    <div className="border-t border-white/[0.06] px-3 py-2.5">
      {error && <p className="m-0 mb-1.5 text-[11px] text-red-400/90">{error}</p>}
      {uploading && <p className="m-0 mb-1.5 text-[11px] text-accent-400/80">Uploading image…</p>}

      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handleFileChange}
        aria-label="Upload an image"
      />

      <div className="flex gap-2 items-center">
        {/* Attach image */}
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={uploading || disabled}
          title="Attach image"
          className="w-9 h-9 rounded-xl shrink-0 border border-white/[0.09] bg-white/[0.04] flex items-center justify-center"
          style={{
            color: uploading || disabled ? 'rgba(255,255,255,0.25)' : 'var(--accent-400)',
            cursor: uploading || disabled ? 'default' : 'pointer',
          }}
          aria-label="Attach image"
        >
          <Image size={14} />
        </button>

        {/* Textarea wrapper */}
        <div className="flex-1 relative">
          <textarea
            ref={textareaRef}
            id="chat-input"
            name="chat-message"
            aria-label="Chat message"
            value={draft}
            onChange={(e) => {
              setDraft(e.target.value)
              resizeTextarea()
            }}
            onKeyDown={handleKeyDown}
            onPaste={handlePaste}
            placeholder="Type a message…"
            disabled={uploading || disabled}
            rows={1}
            dir="auto"
            className="w-full resize-none overflow-y-hidden bg-white/[0.06] border border-white/[0.09] rounded-xl px-3 py-[7px] text-white/85 text-[13px] outline-none box-border"
            style={{
              minHeight,
              maxHeight,
              lineHeight: `${LINE_HEIGHT}px`,
              paddingRight: 32,
            }}
          />
          {/* Expand / collapse button */}
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            title={expanded ? 'Collapse' : 'Expand'}
            className="absolute top-2 right-2 w-[18px] h-[18px] p-0 bg-transparent border-none text-white/50 cursor-pointer flex items-center justify-center"
            aria-label={expanded ? 'Collapse input' : 'Expand input'}
          >
            {expanded ? <Minimize2 size={11} /> : <Maximize2 size={11} />}
          </button>
        </div>

        {/* Send */}
        <button
          type="button"
          onClick={send}
          disabled={!canSend}
          className="w-9 h-9 rounded-xl shrink-0 border-none flex items-center justify-center transition-[background,color] duration-150"
          style={{
            background: canSend ? 'color-mix(in oklab, var(--primary) 80%, transparent)' : 'rgba(255,255,255,0.06)',
            color: canSend ? 'white' : 'rgba(255,255,255,0.25)',
            cursor: canSend ? 'pointer' : 'default',
          }}
          aria-label="Send message"
        >
          <Send size={14} />
        </button>
      </div>
    </div>
  )
})
