import { Building2, Globe2, Loader2, Package, RefreshCw, Search } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import { WebxdcPackageIcon } from './WebxdcPackageIcon'
import { WebxdcPanel } from './WebxdcPanel'
import { useOptionalWebxdcWatch } from './webxdc-watch-context'
import {
  fetchWebxdcConfig,
  importWebxdcGalleryApp,
  listWebxdcGallery,
  type WebxdcGalleryEntry,
  type WebxdcPublicConfig,
} from './webxdcApi'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  roomId: string
  selfName: string
  userId: string
}

/** Three gallery scopes in the meeting Apps dialog. */
type Tab = 'external' | 'instance' | 'room'

function firstDescriptionLine(text?: string): string {
  if (!text) return ''
  return text.split(/\n+/)[0]?.trim() ?? ''
}

function isInstanceEntry(e: WebxdcGalleryEntry): boolean {
  return e.origin === 'instance' || Boolean(e.packageId && !e.xdcUrl)
}

function isExternalEntry(e: WebxdcGalleryEntry): boolean {
  return !isInstanceEntry(e)
}

function isGalleryActive(cfg: WebxdcPublicConfig | null | undefined): boolean {
  return Boolean(cfg?.galleryEnabled && cfg?.enabled)
}

function isRemoteGallerySource(source: string): boolean {
  const s = (source || 'local').toLowerCase()
  return s === 'remote' || s === 'both' || s === 'semi-remote'
}

const TAB_LABELS: Record<Tab, string> = {
  external: 'External gallery',
  instance: 'Instance',
  room: 'This room',
}

/**
 * App gallery dialog: External (remote/semi-remote catalog), Instance (admin catalog),
 * and This room (upload + room packages). Share stages via secure host path.
 */
export function WebxdcAppsDialog({ open, onOpenChange, roomId, selfName, userId }: Props) {
  const watch = useOptionalWebxdcWatch()
  const [cfg, setCfg] = useState<WebxdcPublicConfig | null>(null)
  const [tab, setTab] = useState<Tab>('room')
  const [entries, setEntries] = useState<WebxdcGalleryEntry[]>([])
  const [galleryWarning, setGalleryWarning] = useState<string | null>(null)
  const [loadingGallery, setLoadingGallery] = useState(false)
  const [importingId, setImportingId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [categoryFilter, setCategoryFilter] = useState<string>('all')
  const [roomRefreshKey, setRoomRefreshKey] = useState(0)
  const [reloading, setReloading] = useState(false)

  const galleryEnabled = isGalleryActive(cfg)
  const gallerySource = (cfg?.gallerySource || 'local').toLowerCase()
  const hasExternal = galleryEnabled && isRemoteGallerySource(gallerySource)
  const canImportRemote = galleryEnabled && isRemoteGallerySource(gallerySource)

  const loadGallery = useCallback(async () => {
    setLoadingGallery(true)
    setGalleryWarning(null)
    try {
      const res = await listWebxdcGallery()
      setEntries(res.entries ?? [])
      if (res.warning) setGalleryWarning(res.warning)
    } catch (e) {
      setEntries([])
      setGalleryWarning(e instanceof Error ? e.message : 'Could not load gallery')
    } finally {
      setLoadingGallery(false)
    }
  }, [])

  useEffect(() => {
    if (!open) return
    let cancelled = false
    fetchWebxdcConfig()
      .then((c) => {
        if (cancelled) return
        setCfg(c)
        const enabled = isGalleryActive(c)
        const remote = isRemoteGallerySource(c.gallerySource || 'local')
        if (enabled && remote) setTab('external')
        else if (enabled) setTab('instance')
        else setTab('room')
        if (enabled) void loadGallery()
      })
      .catch(() => {
        if (!cancelled) {
          setCfg({ enabled: false, experimental: true })
          setTab('room')
        }
      })
    return () => {
      cancelled = true
    }
  }, [open, loadGallery])

  const reloadCurrent = useCallback(async () => {
    setReloading(true)
    try {
      if (tab === 'room') {
        setRoomRefreshKey((k) => k + 1)
        return
      }
      try {
        const c = await fetchWebxdcConfig()
        setCfg(c)
      } catch {
        /* keep previous cfg */
      }
      await loadGallery()
    } finally {
      setReloading(false)
    }
  }, [tab, loadGallery])

  const externalEntries = useMemo(() => entries.filter(isExternalEntry), [entries])
  const instanceEntries = useMemo(() => entries.filter(isInstanceEntry), [entries])
  const activeCatalogEntries = tab === 'instance' ? instanceEntries : externalEntries

  const categories = useMemo(() => {
    const set = new Set<string>()
    for (const e of activeCatalogEntries) {
      const c = (e.category || '').trim()
      if (c && c.toLowerCase() !== 'instance') set.add(c)
    }
    return ['all', ...Array.from(set).sort((a, b) => a.localeCompare(b))]
  }, [activeCatalogEntries])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return activeCatalogEntries.filter((e) => {
      if (categoryFilter !== 'all' && (e.category || '').toLowerCase() !== categoryFilter.toLowerCase()) {
        return false
      }
      if (!q) return true
      const hay = `${e.name} ${e.description ?? ''} ${e.id} ${e.category ?? ''}`.toLowerCase()
      return hay.includes(q)
    })
  }, [activeCatalogEntries, query, categoryFilter])

  const shareToMeeting = async (entry: WebxdcGalleryEntry) => {
    if (!watch?.sharePackage) {
      toast.error('Stage is not ready', { description: 'Reload the meeting and try again.' })
      return
    }
    setImportingId(entry.id)
    try {
      if (entry.packageId) {
        await watch.sharePackage(entry.packageId, entry.name)
        onOpenChange(false)
        return
      }
      if (!entry.xdcUrl) {
        toast.error('This catalog entry has no download URL')
        return
      }
      const pkg = await importWebxdcGalleryApp(roomId, {
        xdcUrl: entry.xdcUrl,
        name: entry.name,
      })
      await watch.sharePackage(pkg.id, pkg.name || entry.name)
      onOpenChange(false)
    } catch (e) {
      toast.error('Could not share app', {
        description: e instanceof Error ? e.message : String(e),
      })
    } finally {
      setImportingId(null)
    }
  }

  const selectTab = (next: Tab) => {
    setTab(next)
    setQuery('')
    setCategoryFilter('all')
    if (next === 'external' || next === 'instance') {
      void loadGallery()
    }
  }

  const isBusy = reloading || loadingGallery

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn(
          'meet-dialog flex flex-col gap-0 overflow-hidden p-0 shadow-2xl',
          'sm:max-h-[min(90vh,720px)] sm:w-[min(520px,calc(var(--app-width,100svw)-2rem))] sm:max-w-[min(520px,calc(var(--app-width,100svw)-2rem))]',
          'max-sm:fixed max-sm:left-[var(--app-offset-left,0px)] max-sm:top-[var(--app-offset-top,0px)] max-sm:h-[var(--app-height,100svh)] max-sm:max-h-[var(--app-height,100svh)] max-sm:w-[var(--app-width,100svw)] max-sm:max-w-[var(--app-width,100svw)] max-sm:translate-x-0 max-sm:translate-y-0 max-sm:rounded-none max-sm:border-0',
        )}
      >
        <DialogTitle className="sr-only">WebXDC app gallery</DialogTitle>

        <header className="flex shrink-0 items-center gap-2 border-b border-[var(--meet-border)] px-4 py-3">
          <Package className="h-4 w-4 shrink-0 text-amber-400" aria-hidden />
          <p className="min-w-0 flex-1 truncate text-[15px] font-semibold text-[var(--meet-fg-strong)]">
            {TAB_LABELS[tab]}
          </p>
          <button
            type="button"
            onClick={() => void reloadCurrent()}
            disabled={isBusy}
            className={cn(
              'inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md transition-colors',
              'text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control)] hover:text-[var(--meet-fg-strong)]',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--meet-accent)_50%,transparent)]',
              'disabled:cursor-not-allowed disabled:opacity-50',
            )}
            aria-label="Reload"
            title="Reload"
          >
            <RefreshCw className={cn('h-4 w-4', isBusy && 'animate-spin')} aria-hidden />
          </button>
        </header>

        <div
          className="flex shrink-0 gap-1 overflow-x-auto border-b border-[var(--meet-border)] px-2 py-2"
          role="tablist"
          aria-label="Gallery scope"
        >
          {(
            [
              { id: 'external' as const, icon: Globe2 },
              { id: 'instance' as const, icon: Building2 },
              { id: 'room' as const, icon: Package },
            ] as const
          ).map(({ id, icon: Icon }) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              onClick={() => selectTab(id)}
              className={cn(
                'inline-flex shrink-0 items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-semibold transition-colors',
                tab === id
                  ? 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
                  : 'text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control)] hover:text-[var(--meet-fg-strong)]',
              )}
            >
              <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden />
              {TAB_LABELS[id]}
            </button>
          ))}
        </div>

        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          {tab === 'room' ? (
            <div className="meet-scroll min-h-0 flex-1 overflow-y-auto">
              <WebxdcPanel key={roomRefreshKey} roomId={roomId} selfName={selfName} userId={userId} />
            </div>
          ) : (
            <div className="flex min-h-0 flex-1 flex-col">
              <div className="shrink-0 space-y-2 border-b border-[var(--meet-border)] px-3 py-2">
                <div className="relative">
                  <Search className="pointer-events-none absolute start-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--meet-fg-muted)]" />
                  <Input
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder={tab === 'instance' ? 'Search instance apps…' : 'Search external apps…'}
                    className="h-9 border-[var(--meet-border)] bg-[var(--meet-control)] ps-8 text-sm text-[var(--meet-fg)] placeholder:text-[var(--meet-fg-muted)]"
                  />
                </div>
                {tab === 'external' && categories.length > 1 ? (
                  <div className="flex flex-wrap gap-1">
                    {categories.map((c) => (
                      <button
                        key={c}
                        type="button"
                        onClick={() => setCategoryFilter(c)}
                        className={cn(
                          'rounded-full px-2.5 py-1 text-[11px] font-semibold capitalize transition-colors',
                          categoryFilter === c
                            ? 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
                            : 'bg-[var(--meet-control)] text-[var(--meet-fg-muted)] hover:text-[var(--meet-fg-strong)]',
                        )}
                      >
                        {c === 'all' ? 'All' : c}
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>

              <div className="meet-scroll min-h-0 flex-1 overflow-y-auto p-3">
                {tab === 'external' ? (
                  <p className="mb-3 text-[11px] text-[var(--meet-fg-muted)]">
                    Community / remote catalog. The server fetches metadata; Share downloads, validates, and stages the
                    app — no external pages.
                  </p>
                ) : (
                  <p className="mb-3 text-[11px] text-[var(--meet-fg-muted)]">
                    Apps uploaded by admins for this Bedrud instance. Share stages them into this meeting.
                  </p>
                )}

                {!galleryEnabled ? (
                  <p className="py-6 text-sm text-[var(--meet-fg-muted)]">
                    App gallery is disabled. Enable it in Admin → Settings → WebXDC, or use <strong>This room</strong>{' '}
                    to upload a package.
                  </p>
                ) : tab === 'external' && !hasExternal ? (
                  <p className="py-6 text-sm text-[var(--meet-fg-muted)]">
                    External gallery is off. Set catalog source to <strong>semi-remote</strong>, <strong>remote</strong>
                    , or <strong>both</strong> in Admin → Settings → WebXDC.
                  </p>
                ) : (
                  <>
                    {tab === 'external' && galleryWarning ? (
                      <p className="mb-3 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-200">
                        {galleryWarning}
                      </p>
                    ) : null}
                    {tab === 'external' && !canImportRemote ? (
                      <p className="mb-3 text-sm text-[var(--meet-fg-muted)]">
                        Allow remote downloads in Admin → Settings → WebXDC so Share can fetch .xdc packages.
                      </p>
                    ) : null}
                    {loadingGallery ? (
                      <ul
                        className="m-0 grid list-none grid-cols-2 gap-2 p-0 sm:grid-cols-3"
                        aria-busy="true"
                        aria-label={tab === 'instance' ? 'Loading instance apps' : 'Loading external gallery'}
                      >
                        {Array.from({ length: 6 }, (_, i) => (
                          <li
                            key={`skel-${i}`}
                            className="flex flex-col items-center gap-2 border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] p-3"
                          >
                            <Skeleton className="h-14 w-14 shrink-0 bg-[var(--meet-control)]" />
                            <div className="flex w-full flex-col items-center gap-1.5">
                              <Skeleton className="h-4 w-[85%] bg-[var(--meet-control)]" />
                              <Skeleton className="h-3 w-full bg-[var(--meet-control)]" />
                              <Skeleton className="h-3 w-2/3 max-w-[5rem] bg-[var(--meet-control)]" />
                            </div>
                          </li>
                        ))}
                      </ul>
                    ) : filtered.length === 0 ? (
                      <p className="py-6 text-sm text-[var(--meet-fg-muted)]">
                        {activeCatalogEntries.length === 0
                          ? tab === 'instance'
                            ? 'No instance packages yet. Admins can upload them in Settings → WebXDC.'
                            : 'No external gallery entries.'
                          : 'No apps match your search.'}
                      </p>
                    ) : (
                      <ul
                        className="m-0 grid list-none grid-cols-2 gap-2 p-0 sm:grid-cols-3"
                        aria-label={tab === 'instance' ? 'Instance apps' : 'External apps'}
                        aria-busy={Boolean(importingId)}
                      >
                        {filtered.map((entry) => {
                          const busy = importingId === entry.id
                          const canOpen = Boolean(entry.packageId || entry.xdcUrl)
                          const disabled = !canOpen || Boolean(importingId)
                          const desc =
                            entry.description && entry.description !== 'Instance catalog'
                              ? firstDescriptionLine(entry.description)
                              : ''
                          const entryCategory = entry.category && entry.category !== 'instance' ? entry.category : ''
                          return (
                            <li key={entry.id} className="min-w-0">
                              <button
                                type="button"
                                aria-label={`Open ${entry.name}`}
                                disabled={disabled}
                                onClick={() => void shareToMeeting(entry)}
                                className={cn(
                                  'meet-gallery-app-card flex h-full w-full cursor-pointer flex-col items-center gap-2 border bg-[var(--meet-surface-muted)] p-3 text-center transition-[border-color,background,box-shadow] duration-150',
                                  'border-[var(--meet-border)] hover:border-[color-mix(in_oklab,var(--meet-accent)_45%,transparent)] hover:bg-[var(--meet-control)]',
                                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--meet-accent)_50%,transparent)]',
                                  'disabled:cursor-not-allowed disabled:opacity-50',
                                  busy &&
                                    'border-[color-mix(in_oklab,var(--meet-accent)_55%,transparent)] bg-[var(--meet-btn-muted-bg)] shadow-[0_0_0_1px_color-mix(in_oklab,var(--meet-accent)_30%,transparent)]',
                                )}
                              >
                                <span className="meet-gallery-app-icon relative h-14 w-14 shrink-0 overflow-hidden">
                                  <WebxdcPackageIcon
                                    packageId={entry.packageId}
                                    hasIcon={Boolean(entry.hasIcon || entry.packageId)}
                                    remoteIconUrl={entry.iconUrl}
                                    name={entry.name}
                                    className="h-full w-full"
                                  />
                                  {busy ? (
                                    <span className="absolute inset-0 flex items-center justify-center bg-black/40">
                                      <Loader2 className="h-5 w-5 animate-spin text-white" />
                                    </span>
                                  ) : null}
                                </span>
                                <span className="flex w-full min-w-0 flex-1 flex-col gap-0.5">
                                  <span
                                    className="block truncate text-sm font-semibold leading-5 text-[var(--meet-fg-strong)]"
                                    title={entry.name}
                                  >
                                    {entry.name}
                                  </span>
                                  <span
                                    className={cn(
                                      'line-clamp-2 block h-8 overflow-hidden text-[11px] leading-4 text-[var(--meet-fg-muted)]',
                                      !desc && 'invisible',
                                    )}
                                    title={desc || undefined}
                                    aria-hidden={!desc}
                                  >
                                    {desc || '\u00a0'}
                                  </span>
                                  <span
                                    className={cn(
                                      'block h-4 truncate text-[10px] uppercase leading-4 tracking-wide text-[var(--meet-fg-subtle)]',
                                      !entryCategory && 'invisible',
                                    )}
                                    title={entryCategory || undefined}
                                    aria-hidden={!entryCategory}
                                  >
                                    {entryCategory || '\u00a0'}
                                  </span>
                                </span>
                              </button>
                            </li>
                          )
                        })}
                      </ul>
                    )}
                  </>
                )}
              </div>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
