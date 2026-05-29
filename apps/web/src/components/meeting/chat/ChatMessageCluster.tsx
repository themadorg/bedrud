import { AlertCircle } from 'lucide-react'
import { type ReactNode } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { ClusterGroup } from './chatGrouping'
import { absoluteTime, avatarColor, avatarInitials, relativeTime } from './chatGrouping'

// Telegram-style radius per bubble position
function bubbleRadius(isLocal: boolean, pos: 'only' | 'first' | 'middle' | 'last'): string {
  if (isLocal) {
    if (pos === 'only' || pos === 'first') return '14px 14px 4px 14px'
    if (pos === 'middle') return '14px 4px 4px 14px'
    return '14px 4px 14px 14px'
  }
  if (pos === 'only' || pos === 'first') return '14px 14px 14px 4px'
  if (pos === 'middle') return '4px 14px 14px 4px'
  return '4px 14px 14px 14px'
}

function isSafeUrl(url: string): boolean {
  try {
    const parsed = new URL(url, window.location.origin)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:'
  } catch {
    return false
  }
}

function bubblePosition(idx: number, total: number): 'only' | 'first' | 'middle' | 'last' {
  if (total === 1) return 'only'
  if (idx === 0) return 'first'
  if (idx === total - 1) return 'last'
  return 'middle'
}

function ChatMarkdown({ content, isLocal }: { content: string; isLocal: boolean }) {
  const linkColor = isLocal ? 'rgba(255,255,255,0.9)' : 'color-mix(in oklab, var(--accent-400) 90%, transparent)'
  const codeBg = isLocal ? 'rgba(0,0,0,0.25)' : 'color-mix(in oklab, var(--primary) 15%, transparent)'

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
          borderLeft: `2px solid ${isLocal ? 'rgba(255,255,255,0.4)' : 'color-mix(in oklab, var(--accent-400) 40%, transparent)'}`,
          color: isLocal ? 'rgba(255,255,255,0.7)' : 'rgba(255,255,255,0.5)',
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
    <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
      {content}
    </ReactMarkdown>
  )
}

interface Props {
  cluster: ClusterGroup
}

export function ChatMessageCluster({ cluster }: Props) {
  const { isLocal, sender, identity, messages } = cluster
  const total = messages.length
  const color = avatarColor(identity)
  const initials = avatarInitials(sender)

  return (
    <div className="flex flex-col gap-0.5" style={{ alignItems: isLocal ? 'flex-end' : 'flex-start' }}>
      {/* Sender name (remote only) */}
      {!isLocal && <span className="text-white/50 text-[11px] pl-9">{sender}</span>}

      {messages.map((msg, idx) => {
        const pos = bubblePosition(idx, total)
        const hasAttachments = msg.attachments.length > 0

        return (
          <div
            key={msg.id}
            className="flex items-end gap-2 w-full"
            style={{ flexDirection: isLocal ? 'row-reverse' : 'row' }}
          >
            {/* Avatar slot — 28px wide on remote side for alignment */}
            {!isLocal && (
              <div className="w-7 shrink-0">
                {(pos === 'only' || pos === 'first') && (
                  <div
                    className="w-7 h-7 rounded-full flex items-center justify-center text-[11px] font-bold text-white/95 select-none"
                    style={{ background: color }}
                  >
                    {initials}
                  </div>
                )}
              </div>
            )}

            {/* Bubble */}
            <div
              className="text-[13px] leading-[1.45] break-words overflow-hidden"
              style={{
                maxWidth: '78%',
                padding: hasAttachments && !msg.message ? '4px' : '7px 12px',
                borderRadius: bubbleRadius(isLocal, pos),
                background: isLocal ? 'color-mix(in oklab, var(--primary) 75%, transparent)' : 'rgba(255,255,255,0.07)',
                border: isLocal
                  ? '1px solid color-mix(in oklab, var(--accent-400) 25%, transparent)'
                  : '1px solid rgba(255,255,255,0.06)',
                color: isLocal ? 'rgba(255,255,255,0.95)' : 'rgba(255,255,255,0.75)',
              }}
            >
              {/* Image attachments */}
              {msg.attachments.map((att, ai) => {
                if (att.kind !== 'image') return null
                if (!isSafeUrl(att.url)) return null
                return (
                  <a key={ai} href={att.url} target="_blank" rel="noopener noreferrer">
                    <img
                      src={att.url}
                      alt="attachment"
                      loading="lazy"
                      width={att.w}
                      height={att.h}
                      className="block max-w-full max-h-60 rounded-xl object-contain"
                    />
                  </a>
                )
              })}
              {/* Text */}
              {msg.message && (
                <div className={hasAttachments ? 'pt-1.5 px-2 pb-0.5' : 'p-0'}>
                  <ChatMarkdown content={msg.message} isLocal={isLocal} />
                </div>
              )}
              {isLocal && msg.status === 'failed' && (
                <div className="flex items-center gap-1 mt-1 opacity-90">
                  <AlertCircle size={12} style={{ color: 'var(--destructive, #ef4444)' }} />
                  <span className="text-[11px]" style={{ color: 'var(--destructive, #ef4444)' }}>
                    Failed to send
                  </span>
                </div>
              )}
            </div>
          </div>
        )
      })}

      {/* Timestamp on last bubble */}
      <span
        title={absoluteTime(messages[total - 1].timestamp)}
        className="text-[10px] text-white/50 cursor-default select-none"
        style={{
          paddingLeft: isLocal ? 0 : 36,
          paddingRight: isLocal ? 2 : 0,
        }}
      >
        {relativeTime(messages[total - 1].timestamp)}
      </span>
    </div>
  )
}
