import { BarChart3, Image as ImageIcon, Send, X } from 'lucide-react'
import {
  type ChangeEvent,
  type ClipboardEvent,
  forwardRef,
  type KeyboardEvent,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react'
import { ApiError } from '#/lib/api'
import { textDirectionFor } from '#/lib/text-direction'
import { getPublicSettings } from '#/lib/use-public-settings'
import { cn } from '@/lib/utils'
import type { ChatAttachment, ChatPoll } from '../MeetingContext'
import { ChatEmojiPicker } from './ChatEmojiPicker'
import { ChatPollComposer } from './ChatPollComposer'

const LINE_HEIGHT = 20
const VERTICAL_PADDING = 8
const MIN_ROWS = 2
const MAX_ROWS = 10
const DEFAULT_MAX_BYTES = 10 * 1024 * 1024
const DEFAULT_MAX_DIM = 8192

function formatBytes(n: number): string {
  if (n >= 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(n % (1024 * 1024) === 0 ? 0 : 1)} MB`
  if (n >= 1024) return `${Math.round(n / 1024)} KB`
  return `${n} bytes`
}

async function readImageDimensions(file: File): Promise<{ w: number; h: number } | null> {
  try {
    if (typeof createImageBitmap === 'function') {
      const bmp = await createImageBitmap(file)
      const dims = { w: bmp.width, h: bmp.height }
      bmp.close()
      return dims
    }
  } catch {
    /* fall through */
  }
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.onload = () => {
      resolve({ w: img.naturalWidth, h: img.naturalHeight })
      URL.revokeObjectURL(url)
    }
    img.onerror = () => {
      URL.revokeObjectURL(url)
      resolve(null)
    }
    img.src = url
  })
}

interface Props {
  onSend: (text: string, attachments?: ChatAttachment[], poll?: ChatPoll) => void
  onUpload: (file: File) => Promise<ChatAttachment>
  disabled?: boolean
  elevated?: boolean
}

export interface ChatInputHandle {
  focus: () => void
  /** Validate + upload an image and stage it as a pending attachment (does not send). */
  attachFile: (file: File) => void
}

function generateID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 10)
}

const iconBtnClass = (enabled: boolean) =>
  cn(
    'flex h-7 w-7 shrink-0 items-center justify-center border-none bg-transparent p-0 transition-colors',
    enabled
      ? 'cursor-pointer text-[var(--meet-fg-muted)] hover:text-[var(--meet-accent)]'
      : 'cursor-default text-[var(--meet-fg-subtle)]',
  )

export const ChatInput = forwardRef<ChatInputHandle, Props>(function ChatInput(
  { onSend, onUpload, disabled, elevated = false },
  ref,
) {
  const [draft, setDraft] = useState('')
  const [pendingAttachments, setPendingAttachments] = useState<ChatAttachment[]>([])
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showPollComposer, setShowPollComposer] = useState(false)
  const [inputScrollable, setInputScrollable] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const focusInput = useCallback(() => {
    requestAnimationFrame(() => textareaRef.current?.focus())
  }, [])

  const minHeight = MIN_ROWS * LINE_HEIGHT + VERTICAL_PADDING
  const maxHeight = MAX_ROWS * LINE_HEIGHT + VERTICAL_PADDING

  const resizeTextarea = useCallback(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    const nextHeight = Math.min(Math.max(el.scrollHeight, minHeight), maxHeight)
    el.style.height = `${nextHeight}px`
    setInputScrollable(el.scrollHeight > maxHeight)
  }, [minHeight, maxHeight])

  useEffect(() => {
    resizeTextarea()
  }, [resizeTextarea])

  const send = useCallback(() => {
    const text = draft.trim()
    if (disabled || uploading) return
    if (!text && pendingAttachments.length === 0) return
    onSend(text, pendingAttachments.length > 0 ? pendingAttachments : undefined)
    setDraft('')
    setPendingAttachments([])
    setError(null)
    if (textareaRef.current) textareaRef.current.style.height = `${minHeight}px`
    setInputScrollable(false)
    focusInput()
  }, [draft, pendingAttachments, disabled, uploading, onSend, focusInput, minHeight])

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if ((e.key === 'Enter' && !e.shiftKey) || (e.key === 'Enter' && e.ctrlKey)) {
        e.preventDefault()
        send()
      }
    },
    [send],
  )

  const removePendingAttachment = useCallback((index: number) => {
    setPendingAttachments((prev) => prev.filter((_, i) => i !== index))
  }, [])

  const uploadFile = useCallback(
    async (file: File) => {
      setError(null)
      setUploading(true)
      try {
        let maxBytes = DEFAULT_MAX_BYTES
        let maxDim = DEFAULT_MAX_DIM
        try {
          const s = await getPublicSettings()
          if (s.chatUploadMaxBytes > 0) maxBytes = s.chatUploadMaxBytes
          if (s.chatUploadMaxDimension > 0) maxDim = s.chatUploadMaxDimension
        } catch {
          /* use defaults */
        }

        if (file.size > maxBytes) {
          setError(`Image must be under ${formatBytes(maxBytes)}`)
          return
        }

        if (file.type.startsWith('image/') || !file.type) {
          const dims = await readImageDimensions(file)
          if (dims && (dims.w > maxDim || dims.h > maxDim)) {
            setError(`Image dimensions too large (max ${maxDim}×${maxDim})`)
            return
          }
        }

        const attachment = await onUpload(file)
        setPendingAttachments((prev) => [...prev, attachment])
        focusInput()
      } catch (err) {
        if (err instanceof ApiError) {
          const msg =
            (typeof err.parsedBody?.error === 'string' && err.parsedBody.error) || err.message || 'Upload failed'
          setError(msg)
        } else {
          setError(err instanceof Error ? err.message : 'Upload failed')
        }
      } finally {
        setUploading(false)
      }
    },
    [onUpload, focusInput],
  )

  useImperativeHandle(
    ref,
    () => ({
      focus: focusInput,
      attachFile: (file: File) => {
        void uploadFile(file)
      },
    }),
    [focusInput, uploadFile],
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

  const openImagePicker = useCallback(() => {
    setShowPollComposer(false)
    fileInputRef.current?.click()
  }, [])

  const openPollComposer = useCallback(() => {
    setShowPollComposer(true)
  }, [])

  const sendPoll = useCallback(
    (question: string, optionTexts: string[]) => {
      const poll: ChatPoll = {
        id: generateID(),
        question,
        options: optionTexts.map((text) => ({ id: generateID(), text })),
        votes: {},
      }
      onSend('', undefined, poll)
      focusInput()
    },
    [onSend, focusInput],
  )

  const canSend = (Boolean(draft.trim()) || pendingAttachments.length > 0) && !uploading && !disabled
  const actionsEnabled = !uploading && !disabled

  const insertEmoji = useCallback(
    (emoji: string) => {
      const el = textareaRef.current
      if (!el) {
        setDraft((prev) => prev + emoji)
        return
      }

      const start = el.selectionStart ?? draft.length
      const end = el.selectionEnd ?? draft.length
      const next = `${draft.slice(0, start)}${emoji}${draft.slice(end)}`
      setDraft(next)

      requestAnimationFrame(() => {
        el.focus()
        const caret = start + emoji.length
        el.setSelectionRange(caret, caret)
        resizeTextarea()
      })
    },
    [draft, resizeTextarea],
  )

  const attachButtons = (
    <>
      <button
        type="button"
        disabled={!actionsEnabled}
        title="Upload image"
        onMouseDown={(e) => e.preventDefault()}
        onClick={openImagePicker}
        className={iconBtnClass(actionsEnabled)}
        aria-label="Upload image"
      >
        <ImageIcon size={16} />
      </button>
      <button
        type="button"
        disabled={!actionsEnabled}
        title="Create poll"
        onMouseDown={(e) => e.preventDefault()}
        onClick={openPollComposer}
        className={iconBtnClass(actionsEnabled)}
        aria-label="Create poll"
      >
        <BarChart3 size={16} />
      </button>
    </>
  )

  const emojiButton = (
    <ChatEmojiPicker
      onEmojiSelect={insertEmoji}
      mode="full"
      disabled={disabled || uploading}
      side="top"
      align="end"
      variant="ghost"
      className="h-7 w-7"
      ariaLabel="Insert emoji"
      elevated={elevated}
    />
  )

  const sendButton = (
    <button
      type="button"
      onMouseDown={(e) => e.preventDefault()}
      onClick={send}
      disabled={!canSend}
      className={cn(
        'flex h-7 w-7 shrink-0 items-center justify-center border-none bg-transparent p-0 transition-colors',
        canSend
          ? 'cursor-pointer text-[var(--meet-accent)] hover:text-[var(--meet-accent-fg)]'
          : 'cursor-default text-[var(--meet-fg-subtle)]',
      )}
      aria-label="Send message"
    >
      <Send size={16} />
    </button>
  )

  const textarea = (
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
      rows={MIN_ROWS}
      dir={textDirectionFor(draft)}
      className={cn(
        'meet-scroll meet-rtl-text w-full resize-none border-none bg-transparent px-2 py-1 text-[13px] text-[var(--meet-fg-strong)] outline-none box-border placeholder:text-[var(--meet-fg-subtle)]',
        inputScrollable ? 'overflow-y-auto' : 'overflow-y-hidden',
      )}
      style={{
        minHeight,
        maxHeight,
        lineHeight: `${LINE_HEIGHT}px`,
      }}
    />
  )

  return (
    <div className="shrink-0 border-t border-[var(--meet-border)] px-1.5 py-1.5 pb-[max(0.375rem,env(safe-area-inset-bottom,0px))] sm:pb-1.5">
      {error && <p className="m-0 mb-1.5 text-[11px] text-red-400/90">{error}</p>}
      {uploading && <p className="m-0 mb-1.5 text-[11px] text-[var(--meet-accent)]">Uploading image…</p>}

      {pendingAttachments.length > 0 && (
        <ul className="m-0 mb-1.5 flex list-none flex-wrap gap-1.5 p-0" aria-label="Attached images (not sent yet)">
          {pendingAttachments.map((att, index) => (
            <li
              key={`${att.url}-${index}`}
              className="group relative h-16 w-16 shrink-0 overflow-hidden border border-[var(--meet-border)] bg-white/[0.04]"
            >
              <img
                src={att.url}
                alt={`Attachment ${index + 1}`}
                className="h-full w-full object-cover"
                draggable={false}
              />
              <button
                type="button"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => removePendingAttachment(index)}
                className="absolute top-0.5 right-0.5 flex h-5 w-5 items-center justify-center border-none bg-black/65 text-white/90 transition-colors hover:bg-black/80 hover:text-white"
                aria-label={`Remove attachment ${index + 1}`}
                title="Remove"
              >
                <X size={12} />
              </button>
            </li>
          ))}
        </ul>
      )}

      <ChatPollComposer
        open={showPollComposer}
        onOpenChange={(open) => {
          setShowPollComposer(open)
          if (!open) focusInput()
        }}
        onCreate={sendPoll}
        disabled={disabled || uploading}
      />

      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handleFileChange}
        aria-label="Upload an image"
      />

      <div className="flex flex-col gap-1">
        <div className="w-full">{textarea}</div>
        <div className="flex items-center justify-between gap-1">
          <div className="flex items-center gap-0.5">{attachButtons}</div>
          <div className="flex items-center gap-0.5">
            {emojiButton}
            {sendButton}
          </div>
        </div>
      </div>
    </div>
  )
})
