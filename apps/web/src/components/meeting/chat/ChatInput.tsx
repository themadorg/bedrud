import { BarChart3, Image, Send } from 'lucide-react'
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
import { textDirectionFor } from '#/lib/text-direction'
import { cn } from '@/lib/utils'
import type { ChatAttachment, ChatPoll } from '../MeetingContext'
import { ChatEmojiPicker } from './ChatEmojiPicker'
import { ChatPollComposer } from './ChatPollComposer'

const LINE_HEIGHT = 20
const VERTICAL_PADDING = 8
const MIN_ROWS = 2
const MAX_ROWS = 10

interface Props {
  onSend: (text: string, attachments?: ChatAttachment[], poll?: ChatPoll) => void
  onUpload: (file: File) => Promise<ChatAttachment>
  disabled?: boolean
}

export interface ChatInputHandle {
  focus: () => void
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

export const ChatInput = forwardRef<ChatInputHandle, Props>(function ChatInput({ onSend, onUpload, disabled }, ref) {
  const [draft, setDraft] = useState('')
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showPollComposer, setShowPollComposer] = useState(false)
  const [inputScrollable, setInputScrollable] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const focusInput = useCallback(() => {
    requestAnimationFrame(() => textareaRef.current?.focus())
  }, [])

  useImperativeHandle(ref, () => ({ focus: focusInput }), [focusInput])

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
    if (!text || disabled || uploading) return
    onSend(text)
    setDraft('')
    if (textareaRef.current) textareaRef.current.style.height = `${minHeight}px`
    setInputScrollable(false)
    focusInput()
  }, [draft, disabled, uploading, onSend, focusInput, minHeight])

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
        focusInput()
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Upload failed')
      } finally {
        setUploading(false)
      }
    },
    [draft, onSend, onUpload, focusInput],
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

  const canSend = Boolean(draft.trim()) && !uploading && !disabled
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
        <Image size={16} />
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
    <div className="border-t border-[var(--meet-border)] px-1.5 py-1.5">
      {error && <p className="m-0 mb-1.5 text-[11px] text-red-400/90">{error}</p>}
      {uploading && <p className="m-0 mb-1.5 text-[11px] text-[var(--meet-accent)]">Uploading image…</p>}

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