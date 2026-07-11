import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Package, Trash2, Upload } from 'lucide-react'
import { type DragEvent, useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { API_URL, api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { getErrorMessage } from '#/lib/errors'
import { cn } from '#/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Field, Section, TextInput, Toggle } from './shared'
import type { SystemSettings } from './types'

/** Public xdcget lockfile — server fetches JSON; Bedrud builds the gallery UI (semi-remote). */
export const WEBXDC_SEMI_REMOTE_CATALOG_URL = 'https://apps.testrun.org/xdcget-lock.json'

/** Package id from server is UUID hex only — never pass arbitrary strings into fetch paths. */
const SAFE_PACKAGE_ID = /^[0-9a-fA-F-]{8,64}$/

type CatalogPackage = {
  id: string
  name: string
  description?: string
  category?: string
  sourceCodeUrl?: string
  sizeBytes: number
  contentHash: string
  iconPath?: string
  createdAt?: string
}

function isAllowedXdcFile(file: File): boolean {
  const name = file.name.toLowerCase()
  // Extension check only — never shell out or interpret filename as a path.
  return name.endsWith('.xdc') || name.endsWith('.zip')
}

/**
 * Loads a catalog icon via authenticated fetch → blob URL.
 * Never uses <img src="/api/..."> (no Authorization header on plain img).
 * Revokes the object URL on unmount / id change.
 */
function CatalogIcon({
  packageId,
  hasIcon,
  name,
  className,
}: {
  packageId: string
  hasIcon: boolean
  name: string
  className?: string
}) {
  const [src, setSrc] = useState<string | null>(null)

  useEffect(() => {
    setSrc(null)
    if (!hasIcon || !SAFE_PACKAGE_ID.test(packageId)) {
      return
    }
    let cancelled = false
    let objectUrl: string | null = null

    const load = async () => {
      try {
        const token = useAuthStore.getState().tokens?.accessToken
        const headers: Record<string, string> = {}
        if (token) headers.Authorization = `Bearer ${token}`
        // Encode id — still constrained by SAFE_PACKAGE_ID above.
        const path = `/api/admin/webxdc/catalog/${encodeURIComponent(packageId)}/icon`
        const res = await fetch(`${API_URL}${path}`, {
          credentials: 'include',
          headers,
        })
        if (!res.ok || cancelled) return
        const ct = (res.headers.get('Content-Type') || '').split(';')[0]?.trim().toLowerCase() ?? ''
        // Only raster types — reject anything unexpected (defense in depth; server already sniffs).
        if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(ct)) {
          return
        }
        const blob = await res.blob()
        if (cancelled) return
        objectUrl = URL.createObjectURL(blob)
        setSrc(objectUrl)
      } catch {
        // Leave placeholder — icon is optional decoration.
      }
    }

    void load()
    return () => {
      cancelled = true
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [packageId, hasIcon])

  if (!src) {
    return (
      <div
        className={cn(
          'bg-muted text-muted-foreground flex items-center justify-center border border-border',
          className,
        )}
        aria-hidden
      >
        <Package className="h-6 w-6" />
      </div>
    )
  }

  return (
    <img
      src={src}
      alt=""
      // Decorative — name is shown as text next to the icon.
      title={name}
      className={cn('border border-border object-cover', className)}
      draggable={false}
    />
  )
}

export function WebxdcTab({
  settings,
  setSettings,
  errors,
  clearFieldError,
}: {
  settings: SystemSettings
  setSettings: (s: SystemSettings) => void
  errors?: Record<string, string>
  clearFieldError?: (field: string) => void
}) {
  const ce = (field: string) => clearFieldError?.(field)
  const source = settings.webxdcGallerySource || 'local'
  const qc = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState<string | null>(null)
  const [dragOver, setDragOver] = useState(false)
  const dragDepth = useRef(0)
  const [editPkg, setEditPkg] = useState<CatalogPackage | null>(null)
  const [editName, setEditName] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [editCategory, setEditCategory] = useState('')
  const [editSourceURL, setEditSourceURL] = useState('')

  const isSemiRemotePreset =
    settings.webxdcGalleryEnabled &&
    (source === 'semi-remote' || source === 'remote') &&
    (settings.webxdcGalleryRemoteCatalogUrl || '').trim() === WEBXDC_SEMI_REMOTE_CATALOG_URL &&
    !!settings.webxdcGalleryAllowRemoteDownload

  const applySemiRemotePreset = () => {
    ce('webxdcGallerySource')
    ce('webxdcGalleryRemoteCatalogUrl')
    setSettings({
      ...settings,
      webxdcGalleryEnabled: true,
      webxdcGallerySource: 'semi-remote',
      webxdcGalleryRemoteCatalogUrl: WEBXDC_SEMI_REMOTE_CATALOG_URL,
      webxdcGalleryAllowRemoteDownload: true,
      // Keep instance catalog on so admin-uploaded apps stay visible next to external.
      webxdcGalleryInstanceCatalogEnabled: true,
    })
  }

  const catalogQuery = useQuery({
    queryKey: ['admin-webxdc-catalog'],
    queryFn: async () => {
      const data = await api.get<{ packages: CatalogPackage[] }>('/api/admin/webxdc/catalog')
      return data.packages ?? []
    },
    enabled: !!settings.webxdcGalleryInstanceCatalogEnabled || source === 'local',
  })

  const deleteMut = useMutation({
    mutationFn: async (id: string) => {
      if (!SAFE_PACKAGE_ID.test(id)) throw new Error('invalid package id')
      await api.delete(`/api/admin/webxdc/catalog/${encodeURIComponent(id)}`)
    },
    onSuccess: () => {
      toast.success('Package removed from instance catalog')
      void qc.invalidateQueries({ queryKey: ['admin-webxdc-catalog'] })
    },
    onError: (e) => toast.error(getErrorMessage(e, 'Failed to delete package')),
  })

  const openEdit = (p: CatalogPackage) => {
    if (!SAFE_PACKAGE_ID.test(p.id)) {
      toast.error('Invalid package id')
      return
    }
    setEditPkg(p)
    setEditName(p.name || '')
    setEditDescription(p.description || '')
    setEditCategory(p.category || '')
    setEditSourceURL(p.sourceCodeUrl || '')
  }

  const closeEdit = () => {
    setEditPkg(null)
  }

  const saveMut = useMutation({
    mutationFn: async () => {
      if (!editPkg || !SAFE_PACKAGE_ID.test(editPkg.id)) throw new Error('invalid package id')
      const name = editName.trim()
      if (!name) throw new Error('Name is required')
      if (name.length > 255) throw new Error('Name is too long')
      if (editDescription.length > 2000) throw new Error('Description is too long')
      if (editCategory.trim().length > 64) throw new Error('Category is too long')
      const src = editSourceURL.trim()
      if (src && !/^https:\/\//i.test(src)) throw new Error('Source code URL must be https://')
      return await api.put<CatalogPackage>(`/api/admin/webxdc/catalog/${encodeURIComponent(editPkg.id)}`, {
        name,
        description: editDescription.trim(),
        category: editCategory.trim(),
        sourceCodeUrl: src,
      })
    },
    onSuccess: () => {
      toast.success('Package info updated')
      void qc.invalidateQueries({ queryKey: ['admin-webxdc-catalog'] })
      closeEdit()
    },
    onError: (e) => toast.error(getErrorMessage(e, 'Failed to save package info')),
  })

  const uploadFiles = useCallback(
    async (files: FileList | File[]) => {
      const list = Array.from(files).filter(isAllowedXdcFile)
      if (list.length === 0) {
        toast.error('Only .xdc or .zip packages are accepted')
        return
      }
      const rejected = Array.from(files).length - list.length
      if (rejected > 0) {
        toast.message(`Skipped ${rejected} non-.xdc file${rejected === 1 ? '' : 's'}`)
      }

      setUploading(true)
      let ok = 0
      let fail = 0
      try {
        for (let i = 0; i < list.length; i++) {
          const file = list[i]!
          setUploadProgress(`${i + 1}/${list.length}: ${file.name}`)
          try {
            const fd = new FormData()
            fd.append('file', file)
            await api.post('/api/admin/webxdc/catalog', fd)
            ok++
          } catch (e) {
            fail++
            toast.error(`${file.name}: ${getErrorMessage(e, 'Upload failed')}`)
          }
        }
        if (ok > 0) {
          toast.success(
            ok === 1 ? 'Uploaded package to instance catalog' : `Uploaded ${ok} packages to instance catalog`,
          )
          void qc.invalidateQueries({ queryKey: ['admin-webxdc-catalog'] })
        }
        if (fail > 0 && ok === 0) {
          // per-file errors already toasted
        }
      } finally {
        setUploading(false)
        setUploadProgress(null)
        if (fileRef.current) fileRef.current.value = ''
      }
    },
    [qc],
  )

  const onDragEnter = (e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    dragDepth.current += 1
    setDragOver(true)
  }
  const onDragLeave = (e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    dragDepth.current -= 1
    if (dragDepth.current <= 0) {
      dragDepth.current = 0
      setDragOver(false)
    }
  }
  const onDragOver = (e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    // Indicate copy intent for file drops.
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
  }
  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    dragDepth.current = 0
    setDragOver(false)
    if (uploading) return
    const files = e.dataTransfer?.files
    if (files && files.length > 0) void uploadFiles(files)
  }

  return (
    <Section
      title="WebXDC gallery"
      description="Experimental app catalog for meetings. Requires webxdc.enabled and a real domain in config.yaml. Mini-apps still have no internet — only the server may fetch remote catalogs/packages."
    >
      <Toggle
        checked={!!settings.webxdcGalleryEnabled}
        onChange={(v) => setSettings({ ...settings, webxdcGalleryEnabled: v })}
        label="Enable app gallery"
        hint="Shows the Gallery tab in the meeting Apps panel when WebXDC is enabled."
      />

      <div className="rounded-lg border border-border bg-muted/30 px-3 py-3">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 space-y-1">
            <p className="text-sm font-medium">Semi-remote community catalog</p>
            <p className="text-muted-foreground text-xs leading-relaxed">
              Uses <code className="text-[11px]">{WEBXDC_SEMI_REMOTE_CATALOG_URL}</code>. Server fetches JSON; Bedrud
              builds the UI. Share stages the app into the meeting.
            </p>
          </div>
          <button
            type="button"
            onClick={applySemiRemotePreset}
            className={
              isSemiRemotePreset
                ? 'shrink-0 rounded-md border border-emerald-500/40 bg-emerald-500/10 px-3 py-1.5 text-xs font-semibold text-emerald-700 dark:text-emerald-300'
                : 'bg-primary text-primary-foreground hover:bg-primary/90 shrink-0 rounded-md px-3 py-1.5 text-xs font-semibold'
            }
          >
            {isSemiRemotePreset ? 'Semi-remote active' : 'Use semi-remote catalog'}
          </button>
        </div>
      </div>

      <Field
        label="Catalog source"
        hint="local — instance catalog / room uploads. semi-remote — JSON catalog + Bedrud UI. both — instance catalog + remote."
      >
        <Select
          value={source}
          onValueChange={(v) => {
            ce('webxdcGallerySource')
            if (v === 'semi-remote' && !(settings.webxdcGalleryRemoteCatalogUrl || '').trim()) {
              setSettings({
                ...settings,
                webxdcGallerySource: v,
                webxdcGalleryRemoteCatalogUrl: WEBXDC_SEMI_REMOTE_CATALOG_URL,
                webxdcGalleryAllowRemoteDownload: true,
              })
              return
            }
            setSettings({ ...settings, webxdcGallerySource: v })
          }}
        >
          <SelectTrigger className="h-8 w-full text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="local">Local only</SelectItem>
            <SelectItem value="semi-remote">Semi-remote (JSON + in-app UI)</SelectItem>
            <SelectItem value="remote">Remote catalog</SelectItem>
            <SelectItem value="both">Local + remote</SelectItem>
          </SelectContent>
        </Select>
      </Field>

      <Field
        label="Remote catalog URL"
        hint="HTTPS catalog JSON. Recommended: https://apps.testrun.org/xdcget-lock.json"
      >
        <TextInput
          type="url"
          value={settings.webxdcGalleryRemoteCatalogUrl || ''}
          onChange={(v) => {
            ce('webxdcGalleryRemoteCatalogUrl')
            setSettings({ ...settings, webxdcGalleryRemoteCatalogUrl: v })
          }}
          placeholder={WEBXDC_SEMI_REMOTE_CATALOG_URL}
          mono
          error={errors?.webxdcGalleryRemoteCatalogUrl}
        />
      </Field>

      <Toggle
        checked={!!settings.webxdcGalleryAllowRemoteDownload}
        onChange={(v) => setSettings({ ...settings, webxdcGalleryAllowRemoteDownload: v })}
        label="Allow server to download .xdc packages from remote catalog"
        hint="Required for Share from semi-remote/remote entries."
      />

      <Toggle
        checked={source === 'local' || !!settings.webxdcGalleryInstanceCatalogEnabled}
        onChange={(v) => {
          if (source === 'local') return
          setSettings({ ...settings, webxdcGalleryInstanceCatalogEnabled: v })
        }}
        label="Enable instance catalog (admin-uploaded apps)"
        hint="In addition to remote/semi-remote: packages you upload below appear in every meeting gallery. Always on when catalog source is local."
      />

      <div className="space-y-3 rounded-lg border border-border px-3 py-3">
        <div>
          <p className="text-sm font-medium">Package size limits</p>
          <p className="text-muted-foreground text-xs leading-relaxed">
            Applied on upload, gallery import, and Share. Defaults (if unset): archive 10 MiB, uncompressed 30 MiB,
            single file 5 MiB, 500 entries. <strong>Max single file</strong> is the usual cause of “entry too large”
            when a mini-app embeds large JS/media.
          </p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Max archive (MiB)" hint="Compressed .xdc zip size.">
            <TextInput
              type="number"
              value={String(settings.webxdcMaxArchiveMB ?? 10)}
              onChange={(v) => {
                ce('webxdcMaxArchiveMB')
                const n = Number.parseInt(v, 10)
                setSettings({
                  ...settings,
                  webxdcMaxArchiveMB: Number.isFinite(n) ? n : 0,
                })
              }}
              mono
              error={errors?.webxdcMaxArchiveMB}
            />
          </Field>
          <Field label="Max uncompressed total (MiB)" hint="Sum of all files after unzip.">
            <TextInput
              type="number"
              value={String(settings.webxdcMaxUncompressedMB ?? 30)}
              onChange={(v) => {
                ce('webxdcMaxUncompressedMB')
                const n = Number.parseInt(v, 10)
                setSettings({
                  ...settings,
                  webxdcMaxUncompressedMB: Number.isFinite(n) ? n : 0,
                })
              }}
              mono
              error={errors?.webxdcMaxUncompressedMB}
            />
          </Field>
          <Field label="Max single file (MiB)" hint="Largest single file inside the zip (raises “entry too large”).">
            <TextInput
              type="number"
              value={String(settings.webxdcMaxSingleFileMB ?? 5)}
              onChange={(v) => {
                ce('webxdcMaxSingleFileMB')
                const n = Number.parseInt(v, 10)
                setSettings({
                  ...settings,
                  webxdcMaxSingleFileMB: Number.isFinite(n) ? n : 0,
                })
              }}
              mono
              error={errors?.webxdcMaxSingleFileMB}
            />
          </Field>
          <Field label="Max entries" hint="Max files/folders inside the package.">
            <TextInput
              type="number"
              value={String(settings.webxdcMaxEntries ?? 500)}
              onChange={(v) => {
                ce('webxdcMaxEntries')
                const n = Number.parseInt(v, 10)
                setSettings({
                  ...settings,
                  webxdcMaxEntries: Number.isFinite(n) ? n : 0,
                })
              }}
              mono
              error={errors?.webxdcMaxEntries}
            />
          </Field>
        </div>
      </div>

      {(source === 'local' || settings.webxdcGalleryInstanceCatalogEnabled) && (
        <div className="space-y-3 rounded-lg border border-border px-3 py-3">
          <div>
            <p className="text-sm font-medium">Instance catalog packages</p>
            <p className="text-muted-foreground text-xs">
              Upload .xdc files for this Bedrud instance. They appear in the meeting gallery with origin “instance”.
              Icons are extracted server-side (PNG/JPEG/GIF/WebP only — never SVG).
            </p>
          </div>

          {/* Drag-and-drop zone — <label> is natively associated with the file input */}
          <label
            onDragEnter={onDragEnter}
            onDragLeave={onDragLeave}
            onDragOver={onDragOver}
            onDrop={onDrop}
            className={cn(
              'flex cursor-pointer flex-col items-center justify-center gap-2 border border-dashed px-4 py-6 text-center transition-colors',
              dragOver
                ? 'border-primary bg-primary/5'
                : 'border-border bg-muted/20 hover:border-primary/40 hover:bg-muted/40',
              uploading && 'pointer-events-none opacity-60',
            )}
          >
            <input
              ref={fileRef}
              type="file"
              accept=".xdc,application/zip,.zip"
              multiple
              disabled={uploading}
              className="sr-only"
              onChange={(e) => {
                const files = e.target.files
                if (files && files.length > 0) void uploadFiles(files)
              }}
            />
            {uploading ? (
              <>
                <Loader2 className="text-muted-foreground h-6 w-6 animate-spin" aria-hidden />
                <span className="text-muted-foreground max-w-full truncate text-xs">
                  {uploadProgress ?? 'Uploading…'}
                </span>
              </>
            ) : (
              <>
                <Upload className={cn('h-6 w-6', dragOver ? 'text-primary' : 'text-muted-foreground')} aria-hidden />
                <span className="space-y-0.5">
                  <span className="block text-sm font-medium">
                    {dragOver ? 'Drop packages here' : 'Drag & drop .xdc packages'}
                  </span>
                  <span className="text-muted-foreground block text-xs">or click to browse · multiple files OK</span>
                </span>
              </>
            )}
          </label>

          {catalogQuery.isLoading ? (
            <p className="text-muted-foreground flex items-center gap-2 text-xs">
              <Loader2 className="h-3.5 w-3.5 animate-spin" /> Loading catalog…
            </p>
          ) : catalogQuery.isError ? (
            <p className="text-destructive text-xs">
              Could not load instance catalog (is WebXDC enabled on the server?).
            </p>
          ) : (catalogQuery.data?.length ?? 0) === 0 ? (
            <p className="text-muted-foreground text-xs">No instance packages yet.</p>
          ) : (
            <ul className="m-0 grid max-h-80 list-none grid-cols-2 gap-2 overflow-y-auto p-0 sm:grid-cols-3 md:grid-cols-4">
              {catalogQuery.data?.map((p) => (
                <li key={p.id} className="relative min-w-0">
                  <button
                    type="button"
                    className="bg-card hover:bg-muted/40 focus-visible:ring-ring flex w-full cursor-pointer flex-col items-center gap-2 border border-border p-3 text-center text-xs transition-colors focus-visible:ring-2 focus-visible:outline-none"
                    onClick={() => openEdit(p)}
                    aria-label={`Edit ${p.name}`}
                  >
                    <CatalogIcon packageId={p.id} hasIcon={!!p.iconPath} name={p.name} className="h-14 w-14 shrink-0" />
                    <div className="w-full min-w-0 space-y-0.5">
                      <p className="truncate font-medium" title={p.name}>
                        {p.name}
                      </p>
                      {p.category ? (
                        <p className="text-muted-foreground truncate text-[10px] uppercase tracking-wide">
                          {p.category}
                        </p>
                      ) : null}
                      <p className="text-muted-foreground truncate" title={`${(p.sizeBytes / 1024).toFixed(1)} KB`}>
                        {(p.sizeBytes / 1024).toFixed(1)} KB
                      </p>
                    </div>
                  </button>
                  <button
                    type="button"
                    className="text-destructive hover:bg-destructive/10 absolute top-1.5 right-1.5 z-10 inline-flex h-7 w-7 items-center justify-center"
                    aria-label={`Delete ${p.name}`}
                    disabled={deleteMut.isPending}
                    onClick={(e) => {
                      e.stopPropagation()
                      deleteMut.mutate(p.id)
                    }}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </li>
              ))}
            </ul>
          )}

          <Dialog
            open={!!editPkg}
            onOpenChange={(open) => {
              if (!open) closeEdit()
            }}
          >
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>Edit package</DialogTitle>
                <DialogDescription>
                  Update how this app appears in the instance catalog and meeting gallery. The .xdc archive is not
                  changed.
                </DialogDescription>
              </DialogHeader>

              {editPkg ? (
                <div className="flex flex-col gap-4">
                  <div className="flex items-center gap-3">
                    <CatalogIcon
                      packageId={editPkg.id}
                      hasIcon={!!editPkg.iconPath}
                      name={editName || editPkg.name}
                      className="h-14 w-14 shrink-0"
                    />
                    <div className="min-w-0 text-xs">
                      <p className="text-muted-foreground truncate font-mono" title={editPkg.id}>
                        {editPkg.id}
                      </p>
                      <p className="text-muted-foreground">
                        {(editPkg.sizeBytes / 1024).toFixed(1)} KB · {editPkg.contentHash.slice(0, 12)}…
                      </p>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="webxdc-pkg-name">Name</Label>
                    <Input
                      id="webxdc-pkg-name"
                      value={editName}
                      onChange={(e) => setEditName(e.target.value)}
                      maxLength={255}
                      autoComplete="off"
                      className="border-border border bg-transparent"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="webxdc-pkg-desc">Description</Label>
                    <textarea
                      id="webxdc-pkg-desc"
                      value={editDescription}
                      onChange={(e) => setEditDescription(e.target.value)}
                      maxLength={2000}
                      rows={4}
                      className="border-border placeholder:text-muted-foreground/40 focus-visible:border-primary flex w-full resize-y border bg-transparent px-2 py-2 text-sm focus-visible:outline-none disabled:opacity-50"
                      placeholder="Short blurb shown in the meeting gallery"
                    />
                    <p className="text-muted-foreground text-[11px]">{editDescription.length}/2000</p>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="webxdc-pkg-cat">Category</Label>
                    <Input
                      id="webxdc-pkg-cat"
                      value={editCategory}
                      onChange={(e) => setEditCategory(e.target.value)}
                      maxLength={64}
                      autoComplete="off"
                      placeholder="e.g. tools, games"
                      className="border-border border bg-transparent"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="webxdc-pkg-src">Source code URL</Label>
                    <Input
                      id="webxdc-pkg-src"
                      type="url"
                      value={editSourceURL}
                      onChange={(e) => setEditSourceURL(e.target.value)}
                      maxLength={512}
                      autoComplete="off"
                      placeholder="https://…"
                      className="border-border border bg-transparent font-mono text-xs"
                    />
                    <p className="text-muted-foreground text-[11px]">Optional. HTTPS only.</p>
                  </div>
                </div>
              ) : null}

              <DialogFooter className="gap-2">
                <Button type="button" variant="outline" onClick={closeEdit} disabled={saveMut.isPending}>
                  Cancel
                </Button>
                <Button type="button" disabled={saveMut.isPending || !editName.trim()} onClick={() => saveMut.mutate()}>
                  {saveMut.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Save'}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      )}
    </Section>
  )
}
