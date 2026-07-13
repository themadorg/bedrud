import { Loader2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { cn } from '#/lib/utils'
import { WebxdcPackageIcon } from './WebxdcPackageIcon'
import { useOptionalWebxdcWatch } from './webxdc-watch-context'
import {
  fetchWebxdcConfig,
  listWebxdcInstances,
  listWebxdcPackages,
  uploadWebxdcPackage,
  type WebxdcInstance,
  type WebxdcPackage,
  type WebxdcPublicConfig,
} from './webxdcApi'

type Props = {
  roomId: string
  selfName: string
  userId: string
  canUpload?: boolean
}

/**
 * Experimental WebXDC Apps panel: upload, list, start on stage for the room.
 * Uses optional stage watch context (must be under WebxdcWatchProvider for share-on-stage).
 */
export function WebxdcPanel({ roomId, canUpload = true }: Props) {
  const watch = useOptionalWebxdcWatch()
  const shareFile = watch?.shareFile
  const sharePackage = watch?.sharePackage
  const stageBusy = watch?.busy ?? false
  const session = watch?.session ?? null
  const [cfg, setCfg] = useState<WebxdcPublicConfig | null>(null)
  const [packages, setPackages] = useState<WebxdcPackage[]>([])
  const [instances, setInstances] = useState<WebxdcInstance[]>([])
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [openingId, setOpeningId] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    if (!cfg?.enabled) return
    const [p, i] = await Promise.all([listWebxdcPackages(roomId), listWebxdcInstances(roomId)])
    setPackages(p)
    setInstances(i)
  }, [cfg?.enabled, roomId])

  useEffect(() => {
    fetchWebxdcConfig()
      .then(setCfg)
      .catch(() => setCfg({ enabled: false, experimental: true }))
  }, [])

  useEffect(() => {
    if (!cfg?.enabled) return
    refresh().catch((e) => setError(String(e)))
  }, [cfg, refresh])

  if (!cfg) {
    return <p className="text-muted-foreground p-3 text-sm">Loading WebXDC…</p>
  }
  if (!cfg.enabled) {
    return (
      <p className="text-muted-foreground p-3 text-sm">
        WebXDC is disabled on this server (experimental; requires domain + config).
      </p>
    )
  }

  const working = busy || stageBusy

  const onUpload = async (file: File | null) => {
    if (!file) return
    setBusy(true)
    setError(null)
    try {
      if (shareFile) {
        await shareFile(file)
      } else {
        await uploadWebxdcPackage(roomId, file)
      }
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const onStart = async (packageId: string, name?: string) => {
    setBusy(true)
    setOpeningId(packageId)
    setError(null)
    try {
      if (sharePackage) {
        await sharePackage(packageId, name)
      } else {
        setError('Stage sharing is not ready — reload the meeting.')
      }
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
      setOpeningId(null)
    }
  }

  const onUploadOnly = async (file: File | null) => {
    if (!file) return
    setBusy(true)
    setError(null)
    try {
      await uploadWebxdcPackage(roomId, file)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-3 p-3">
      <div>
        <h2 className="text-base font-semibold">
          Apps{' '}
          <span className="bg-muted text-muted-foreground ml-1 rounded px-1.5 py-0.5 text-xs font-normal">
            experimental
          </span>
        </h2>
        <p className="text-muted-foreground mt-1 text-xs">
          Drop a <code className="text-xs">.xdc</code> anywhere in the meeting, or upload here. Starting shares the
          mini-app on stage for everyone. Host: <code className="text-xs">{cfg.baseDomain}</code>
        </p>
        {session ? <p className="text-primary mt-1 text-xs">On stage: {session.name}</p> : null}
      </div>

      {error ? <p className="text-destructive text-sm">{error}</p> : null}

      {canUpload ? (
        <label className="flex cursor-pointer flex-col gap-1 text-sm">
          <span className="font-medium">Upload & share .xdc</span>
          <input
            type="file"
            accept=".xdc,application/zip"
            disabled={working}
            onChange={(e) => onUpload(e.target.files?.[0] ?? null)}
          />
        </label>
      ) : null}

      {canUpload ? (
        <label className="flex cursor-pointer flex-col gap-1 text-sm">
          <span className="text-muted-foreground font-medium">Upload only (no stage)</span>
          <input
            type="file"
            accept=".xdc,application/zip"
            disabled={working}
            onChange={(e) => onUploadOnly(e.target.files?.[0] ?? null)}
          />
        </label>
      ) : null}

      <section>
        <h3 className="mb-2 text-sm font-medium">This room</h3>
        {packages.length === 0 ? (
          <p className="text-muted-foreground text-sm">No packages yet.</p>
        ) : (
          <ul
            className="m-0 grid list-none grid-cols-2 gap-2 p-0 sm:grid-cols-3"
            role="listbox"
            aria-label="Room apps"
            aria-busy={working}
          >
            {packages.map((p) => {
              const opening = openingId === p.id
              return (
                <li key={p.id} className="min-w-0">
                  <button
                    type="button"
                    role="option"
                    aria-selected={opening}
                    aria-label={`Open ${p.name}`}
                    disabled={working}
                    onClick={() => void onStart(p.id, p.name)}
                    className={cn(
                      'meet-gallery-app-card flex h-full w-full cursor-pointer flex-col items-center gap-2 border bg-[var(--meet-surface-muted)] p-3 text-center text-sm transition-[border-color,background,box-shadow] duration-150',
                      'border-[var(--meet-border)] hover:border-[color-mix(in_oklab,var(--meet-accent)_45%,transparent)] hover:bg-[var(--meet-control)]',
                      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--meet-accent)_50%,transparent)]',
                      'disabled:cursor-not-allowed disabled:opacity-50',
                      opening &&
                        'border-[color-mix(in_oklab,var(--meet-accent)_55%,transparent)] bg-[var(--meet-btn-muted-bg)] shadow-[0_0_0_1px_color-mix(in_oklab,var(--meet-accent)_30%,transparent)]',
                    )}
                  >
                    <span className="meet-gallery-app-icon relative h-14 w-14 shrink-0 overflow-hidden">
                      <WebxdcPackageIcon
                        packageId={p.id}
                        hasIcon={Boolean(p.iconPath)}
                        name={p.name}
                        className="h-full w-full"
                      />
                      {opening ? (
                        <span className="absolute inset-0 flex items-center justify-center bg-black/40">
                          <Loader2 className="h-5 w-5 animate-spin text-white" />
                        </span>
                      ) : null}
                    </span>
                    {/* Match gallery card text slots (name + reserved description + category) */}
                    <span className="flex w-full min-w-0 flex-1 flex-col gap-0.5">
                      <span
                        className="block truncate text-sm font-semibold leading-5 text-[var(--meet-fg-strong)]"
                        title={p.name}
                      >
                        {p.name}
                      </span>
                      <span
                        className="invisible line-clamp-2 block h-8 overflow-hidden text-[11px] leading-4"
                        aria-hidden
                      >
                        {'\u00a0'}
                      </span>
                      <span className="invisible block h-4 truncate text-[10px] leading-4" aria-hidden>
                        {'\u00a0'}
                      </span>
                    </span>
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </section>

      <section>
        <h3 className="mb-1 text-sm font-medium">Open instances</h3>
        {instances.length === 0 ? (
          <p className="text-muted-foreground text-sm">None.</p>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-1.5 p-0" role="listbox" aria-label="Open instances">
            {instances.map((i) => {
              const label = i.package?.name ?? i.id
              const opening = openingId === i.packageId
              return (
                <li key={i.id} className="min-w-0">
                  <button
                    type="button"
                    role="option"
                    aria-selected={opening}
                    aria-label={`Open ${label}`}
                    disabled={working}
                    onClick={() => void onStart(i.packageId, i.package?.name)}
                    className={cn(
                      'meet-gallery-app-card flex w-full cursor-pointer items-center gap-2 border bg-[var(--meet-surface-muted)] px-2.5 py-2 text-start text-sm transition-[border-color,background,box-shadow] duration-150',
                      'border-[var(--meet-border)] hover:border-[color-mix(in_oklab,var(--meet-accent)_45%,transparent)] hover:bg-[var(--meet-control)]',
                      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklab,var(--meet-accent)_50%,transparent)]',
                      'disabled:cursor-not-allowed disabled:opacity-50',
                      opening &&
                        'border-[color-mix(in_oklab,var(--meet-accent)_55%,transparent)] bg-[var(--meet-btn-muted-bg)]',
                    )}
                  >
                    <span className="min-w-0 flex-1 truncate text-[var(--meet-fg-strong)]">
                      {label}
                      {i.summary ? (
                        <span className="text-[var(--meet-fg-muted)]">{` — ${i.summary}`}</span>
                      ) : null}
                    </span>
                    {opening ? <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-[var(--meet-accent)]" /> : null}
                  </button>
                </li>
              )
            })}
          </ul>
        )}
      </section>
    </div>
  )
}
