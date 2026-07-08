import { Search, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { cn } from '@/lib/utils'
import { loadEmojiGroups } from './loadEmojiGroups'
import { searchEmojis } from './search'
import type { EmojiGroup, EmojiItem, EmojiPickerProps } from './types'

const DEFAULT_EMOJIS_PER_ROW = 8
const DEFAULT_EMOJI_SIZE = 28
const DEFAULT_CONTAINER_HEIGHT = 280

function EmojiButton({ item, size, onSelect }: { item: EmojiItem; size: number; onSelect: (emoji: string) => void }) {
  return (
    <button
      type="button"
      onMouseDown={(e) => e.preventDefault()}
      onClick={() => onSelect(item.emoji)}
      className="flex items-center justify-center rounded-lg leading-none hover:bg-[var(--meet-control-hover)] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[color-mix(in_oklab,var(--meet-accent)_50%,transparent)]"
      style={{ width: size + 8, height: size + 8, fontSize: size - 4 }}
      aria-label={item.name}
      title={item.name}
    >
      {item.emoji}
    </button>
  )
}

function CategorySection({
  category,
  emojis,
  emojisPerRow,
  emojiSize,
  onSelect,
}: {
  category: string
  emojis: EmojiItem[]
  emojisPerRow: number
  emojiSize: number
  onSelect: (emoji: string) => void
}) {
  return (
    <section className="px-1.5">
      <h3 className="sticky top-0 z-[1] bg-[var(--meet-bg-panel)] px-1 py-1.5 text-[11px] font-semibold text-[var(--meet-fg-muted)] backdrop-blur-sm">
        {category}
      </h3>
      <div className="grid gap-0.5 pb-2" style={{ gridTemplateColumns: `repeat(${emojisPerRow}, minmax(0, 1fr))` }}>
        {emojis.map((item) => (
          <EmojiButton key={`${category}-${item.slug}`} item={item} size={emojiSize} onSelect={onSelect} />
        ))}
      </div>
    </section>
  )
}

export function EmojiPicker({
  onEmojiSelect,
  className,
  emojisPerRow = DEFAULT_EMOJIS_PER_ROW,
  emojiSize = DEFAULT_EMOJI_SIZE,
  containerHeight = DEFAULT_CONTAINER_HEIGHT,
  autoFocus = true,
}: EmojiPickerProps) {
  const [groups, setGroups] = useState<EmojiGroup[] | null>(null)
  const [search, setSearch] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    let cancelled = false
    void loadEmojiGroups().then((data) => {
      if (cancelled) return
      setGroups(data)
      if (autoFocus) {
        setTimeout(() => inputRef.current?.focus(), 40)
      }
    })
    return () => {
      cancelled = true
    }
  }, [autoFocus])

  const searchResults = useMemo(() => {
    if (!groups || !search.trim()) return null
    return searchEmojis(groups, search)
  }, [groups, search])

  const handleSelect = useCallback(
    (emoji: string) => {
      onEmojiSelect(emoji)
      setSearch('')
    },
    [onEmojiSelect],
  )

  const clearSearch = useCallback(() => setSearch(''), [])

  return (
    <div
      className={cn(
        'flex w-[min(300px,85vw)] flex-col overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-bg-panel)] text-[var(--meet-fg)] shadow-[var(--meet-shadow)]',
        className,
      )}
    >
      <div className="flex items-center gap-2 border-b border-[var(--meet-border)] px-2.5 py-2">
        <div className="relative flex-1">
          <Search
            size={14}
            className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--meet-fg-subtle)]"
          />
          <input
            ref={inputRef}
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search emoji"
            className="h-8 w-full rounded-lg border border-[var(--meet-border)] bg-[var(--meet-control)] pl-8 pr-8 text-[13px] text-[var(--meet-fg-strong)] placeholder:text-[var(--meet-fg-subtle)] focus:border-[color-mix(in_oklab,var(--meet-accent)_40%,transparent)] focus:outline-none"
          />
          {search && (
            <button
              type="button"
              onClick={clearSearch}
              className="absolute right-1.5 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-md text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control-hover)] hover:text-[var(--meet-fg-strong)]"
              aria-label="Clear search"
            >
              <X size={13} />
            </button>
          )}
        </div>
      </div>

      <div className="meet-scroll overflow-y-auto overscroll-contain" style={{ height: containerHeight }}>
        {!groups ? (
          <div className="flex h-full items-center justify-center text-xs text-[var(--meet-fg-muted)]">
            Loading emojis…
          </div>
        ) : searchResults ? (
          searchResults.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center gap-1 px-4 text-center">
              <span className="text-2xl opacity-40">🔍</span>
              <p className="text-xs text-[var(--meet-fg-muted)]">No emoji found</p>
            </div>
          ) : (
            <div
              className="grid gap-0.5 p-2"
              style={{ gridTemplateColumns: `repeat(${emojisPerRow}, minmax(0, 1fr))` }}
            >
              {searchResults.map((item) => (
                <EmojiButton key={item.slug} item={item} size={emojiSize} onSelect={handleSelect} />
              ))}
            </div>
          )
        ) : (
          groups.map((group) => (
            <CategorySection
              key={group.slug}
              category={group.category}
              emojis={group.emojis}
              emojisPerRow={emojisPerRow}
              emojiSize={emojiSize}
              onSelect={handleSelect}
            />
          ))
        )}
      </div>
    </div>
  )
}
