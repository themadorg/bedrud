import { useRoomContext } from '@livekit/components-react'
import { AlertCircle } from 'lucide-react'
import { type ReactNode, Suspense } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { textDirectionFor } from '#/lib/text-direction'
import { cn } from '@/lib/utils'
import { ChatMessageContextMenu } from './ChatMessageContextMenu'
import { ChatPollBubble } from './ChatPollBubble'
import { ChatReactionList } from './ChatReactionList'
import { ChatReactionPicker } from './ChatReactionPicker'
import { bubbleChrome, bubblePosition } from './chatBubbleStyles'
import { isSingleEmojiMessage } from './chatEmojiMessage'
import type { ClusterGroup } from './chatGrouping'
import { absoluteTime, avatarColor, avatarInitials, relativeTime } from './chatGrouping'

function isSafeUrl(url: string): boolean {
  if (url.startsWith('data:image/')) return true
  try {
    const parsed = new URL(url, window.location.origin)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:'
  } catch {
    return false
  }
}

function ChatMarkdown({ content, isLocal }: { content: string; isLocal: boolean }) {
  const direction = textDirectionFor(content)
  const linkColor = isLocal ? 'var(--meet-chat-md-link-local)' : 'var(--meet-chat-md-link-remote)'
  const codeBg = isLocal ? 'var(--meet-chat-md-code-local)' : 'var(--meet-chat-md-code-remote)'
  const quoteBorder = isLocal ? 'var(--meet-chat-md-quote-border-local)' : 'var(--meet-chat-md-quote-border-remote)'
  const quoteColor = isLocal ? 'var(--meet-chat-md-quote-fg-local)' : 'var(--meet-chat-md-quote-fg-remote)'

  type C = { children?: ReactNode }
  type CA = { href?: string; children?: ReactNode }
  type CC = { children?: ReactNode; className?: string }

  const components = {
    a: ({ href, children }: CA) => {
      if (!href || (!href.startsWith('http://') && !href.startsWith('https://') && !href.startsWith('/'))) {
        return <span>{children}</span>
      }
      return (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          style={{ color: linkColor, textDecoration: 'underline', wordBreak: 'break-all' }}
        >
          {children}
        </a>
      )
    },
    p: ({ children }: C) => <p style={{ margin: 0, lineHeight: 1.45 }}>{children}</p>,
    code: ({ children, className }: CC) => {
      const isBlock = Boolean(className)
      return isBlock ? (
        <pre
          style={{
            margin: '4px 0',
            padding: '6px 9px',
            borderRadius: 6,
            background: codeBg,
            overflowX: 'auto',
            fontSize: 12,
          }}
        >
          <code>{children}</code>
        </pre>
      ) : (
        <code style={{ background: codeBg, borderRadius: 4, padding: '1px 5px', fontSize: 12 }}>{children}</code>
      )
    },
    ul: ({ children }: C) => <ul style={{ margin: '2px 0', paddingInlineStart: 18 }}>{children}</ul>,
    ol: ({ children }: C) => <ol style={{ margin: '2px 0', paddingInlineStart: 18 }}>{children}</ol>,
    li: ({ children }: C) => <li style={{ lineHeight: 1.45 }}>{children}</li>,
    strong: ({ children }: C) => <strong style={{ fontWeight: 700 }}>{children}</strong>,
    em: ({ children }: C) => <em style={{ fontStyle: 'italic' }}>{children}</em>,
    blockquote: ({ children }: C) => (
      <blockquote
        style={{
          margin: '4px 0',
          paddingLeft: 10,
          borderLeft: `2px solid ${quoteBorder}`,
          color: quoteColor,
        }}
      >
        {children}
      </blockquote>
    ),
    h1: ({ children }: C) => <strong style={{ fontSize: 15 }}>{children}</strong>,
    h2: ({ children }: C) => <strong style={{ fontSize: 14 }}>{children}</strong>,
    h3: ({ children }: C) => <strong style={{ fontSize: 13 }}>{children}</strong>,
  }

  return (
    <Suspense fallback={<span className="text-xs text-[var(--meet-fg-subtle)]">Loading…</span>}>
      <div dir={direction} className="text-start meet-rtl-text">
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
          {content}
        </ReactMarkdown>
      </div>
    </Suspense>
  )
}

interface Props {
  cluster: ClusterGroup
  currentIdentity: string
  onImageOpen?: (url: string) => void
  onVotePoll: (messageId: string, optionId: string) => void
  onReactToMessage: (messageId: string, emoji: string) => void
}

export function ChatMessageCluster({ cluster, currentIdentity, onImageOpen, onVotePoll, onReactToMessage }: Props) {
  const room = useRoomContext()
  const localIdentity = room.localParticipant.identity
  const { sender, identity, messages } = cluster
  const ownIdentity = localIdentity || currentIdentity
  const isSelf =
    messages.some((m) => m.isLocal) ||
    (!!ownIdentity && (identity === ownIdentity || messages.every((m) => m.senderIdentity === ownIdentity)))
  const total = messages.length
  const lastIsLoneEmoji = isSingleEmojiMessage(messages[total - 1])
  const color = avatarColor(identity)
  const initials = avatarInitials(sender)

  const messageRows = messages.map((msg, idx) => {
            const pos = bubblePosition(idx, total)
            const stacked = total > 1
            const loneEmoji = isSingleEmojiMessage(msg)
            const chrome = loneEmoji ? null : bubbleChrome(isSelf, pos, stacked)
            const hasAttachments = msg.attachments.length > 0
            const hasPoll = Boolean(msg.poll)
            const hasRichContent = hasAttachments || hasPoll

            return (
              <div
                key={msg.id}
                className={cn(
                  'group/msg flex flex-col',
                  isSelf ? 'items-end' : 'w-full items-start',
                  loneEmoji && (isSelf ? 'py-0.5' : 'py-1 mb-2'),
                )}
              >
                <div className="relative max-w-full">
                  <div
                    className={cn(
                      'absolute top-1/2 z-[2] -translate-y-1/2 opacity-0 transition-opacity pointer-events-none',
                      'group-hover/msg:opacity-100 group-hover/msg:pointer-events-auto',
                      'group-focus-within/msg:opacity-100 group-focus-within/msg:pointer-events-auto',
                      isSelf ? 'right-full mr-0.5' : 'left-full ml-0.5',
                    )}
                  >
                    <ChatReactionPicker isLocal={isSelf} onReact={(emoji) => onReactToMessage(msg.id, emoji)} />
                  </div>
                  <ChatMessageContextMenu message={msg} senderName={sender} currentIdentity={currentIdentity}>
                    <div
                      className={
                        loneEmoji
                          ? 'min-w-0 text-[56px] leading-none select-none'
                          : 'min-w-0 text-[13px] leading-[1.45] break-words overflow-hidden'
                      }
                      style={
                        loneEmoji
                          ? undefined
                          : {
                              padding: hasRichContent && !msg.message ? '4px' : '7px 12px',
                              ...chrome,
                            }
                      }
                    >
                      {msg.attachments.map((att, ai) => {
                        if (att.kind !== 'image' && !att.mime.startsWith('image/')) return null
                        if (!isSafeUrl(att.url)) return null
                        return (
                          <button
                            key={ai}
                            type="button"
                            onClick={() => onImageOpen?.(att.url)}
                            className="block cursor-pointer rounded-xl border-none bg-transparent p-0"
                            aria-label="View image"
                          >
                            <img
                              src={att.url}
                              alt="attachment"
                              loading="lazy"
                              width={att.w}
                              height={att.h}
                              className="block max-h-60 max-w-full rounded-xl object-contain"
                            />
                          </button>
                        )
                      })}
                      {msg.poll && (
                        <div className={hasAttachments || msg.message ? 'px-2 py-1.5' : 'p-1'}>
                          <ChatPollBubble
                            poll={msg.poll}
                            messageId={msg.id}
                            isLocal={isSelf}
                            currentIdentity={currentIdentity}
                            onVote={onVotePoll}
                          />
                        </div>
                      )}
                      {msg.message &&
                        (loneEmoji ? (
                          <span aria-label="Emoji message" dir={textDirectionFor(msg.message)}>
                            {msg.message.trim()}
                          </span>
                        ) : (
                          <div className={hasRichContent ? 'px-2 pb-0.5 pt-1.5' : 'p-0'}>
                            <ChatMarkdown content={msg.message} isLocal={isSelf} />
                          </div>
                        ))}
                      {isSelf && msg.status === 'failed' && (
                        <div className="mt-1 flex items-center gap-1 opacity-90">
                          <AlertCircle size={12} style={{ color: 'var(--destructive, #ef4444)' }} />
                          <span className="text-[11px]" style={{ color: 'var(--destructive, #ef4444)' }}>
                            Failed to send
                          </span>
                        </div>
                      )}
                    </div>
                  </ChatMessageContextMenu>
                </div>
                <ChatReactionList
                  reactions={msg.reactions}
                  currentIdentity={currentIdentity}
                  isLocal={isSelf}
                  onReact={(emoji) => onReactToMessage(msg.id, emoji)}
                />
              </div>
            )
  })

  return (
    <div
      className={cn(
        'flex flex-col',
        isSelf ? 'ms-auto w-fit max-w-[min(100%,320px)] items-end gap-0.5' : 'w-full items-start gap-1',
        lastIsLoneEmoji && !isSelf && 'mb-1',
      )}
    >
      {!isSelf && <span className="pl-9 text-[11px] text-[var(--meet-fg-muted)]">{sender}</span>}

      {isSelf ? (
        <div className="flex flex-col items-end gap-0.5">{messageRows}</div>
      ) : (
        <div className="flex w-full items-end gap-2">
          <div className="w-7 shrink-0 self-end">
            <div
              className="flex h-7 w-7 select-none items-center justify-center rounded-full text-[11px] font-bold text-white/95"
              style={{ background: color }}
            >
              {initials}
            </div>
          </div>
          <div className="flex min-w-0 max-w-[78%] flex-col items-start gap-2">{messageRows}</div>
        </div>
      )}

      {/* Timestamp on last bubble */}
      <span
        title={absoluteTime(messages[total - 1].timestamp)}
        className={cn(
          'cursor-default select-none text-[10px] text-[var(--meet-fg-muted)]',
          lastIsLoneEmoji && (isSelf ? 'mt-0.5' : 'mt-1.5'),
        )}
        style={{
          paddingLeft: isSelf ? 0 : 36,
        }}
      >
        {relativeTime(messages[total - 1].timestamp)}
      </span>

    </div>
  )
}
