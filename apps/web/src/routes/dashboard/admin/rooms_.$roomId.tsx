// TODO oncoming feature
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import {
  Activity,
  ArrowLeft,
  Globe,
  Lock,
  Mic,
  MicOff,
  Monitor,
  Pin,
  Power,
  RefreshCw,
  Trash2,
  Users,
  UserX,
  Video,
  Volume2,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { toast } from 'sonner'

import { Button } from '#/components/ui/button'
import { api } from '#/lib/api'
import { getErrorMessage } from '#/lib/errors'
import { useAdminContext } from '#/routes/dashboard/admin.tsx'

export const Route = createFileRoute('/dashboard/admin/rooms_/$roomId')({ component: RoomDetailPage })

interface Track {
  sid: string
  type: string // AUDIO | VIDEO
  source: string // MICROPHONE | CAMERA | SCREEN_SHARE | …
  muted: boolean
  bitrate: number // target bitrate from highest quality layer (bps)
}

interface LiveParticipant {
  sid: string
  identity: string
  name: string
  state: string
  joinedAt: number // unix seconds
  isPublisher: boolean
  tracks: Track[]
}

interface RoomInfo {
  id: string
  name: string
  isPublic: boolean
  isActive: boolean
  maxParticipants: number
  createdAt: string
  settings?: { allowChat: boolean; allowVideo: boolean; allowAudio: boolean; e2ee: boolean; isPersistent?: boolean }
}

function formatBitrate(bps: number) {
  if (!bps) return '—'
  if (bps < 1000) return `${bps} bps`
  if (bps < 1_000_000) return `${(bps / 1000).toFixed(0)} kbps`
  return `${(bps / 1_000_000).toFixed(2)} Mbps`
}

function TrackBadge({ track }: { track: Track }) {
  const isAudio = track.type === 'AUDIO'
  const Icon = track.source === 'SCREEN_SHARE' ? Monitor : isAudio ? Volume2 : Video
  const bitrateLabel = track.muted ? 'muted' : isAudio ? 'live' : formatBitrate(track.bitrate)
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium"
      style={
        track.muted
          ? { background: 'var(--muted)', color: 'var(--muted-foreground)' }
          : isAudio
            ? { background: '#10b98115', color: '#10b981' }
            : { background: 'color-mix(in oklab, var(--primary) 8%, transparent)', color: 'var(--accent-400)' }
      }
      title={isAudio ? `${track.source} · audio (bitrate N/A)` : `${track.source} · ${formatBitrate(track.bitrate)}`}
    >
      <Icon className="h-3 w-3" />
      {bitrateLabel}
    </span>
  )
}

// Rolling window: last 60 samples (~3 minutes at 3 s poll)
const MAX_HISTORY = 60

type BitratePoint = { time: string; total: number; [identity: string]: number | string }

function RoomDetailPage() {
  const { roomId } = Route.useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { isReadOnly } = useAdminContext()
  const [confirmKick, setConfirmKick] = useState<string | null>(null)
  const [confirmSuspend, setConfirmSuspend] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  // Rolling bitrate history — stored in a ref so appending doesn't retrigger queries
  const historyRef = useRef<BitratePoint[]>([])
  const [bitrateHistory, setBitrateHistory] = useState<BitratePoint[]>([])
  // Track which participant identities have appeared so we can assign stable colors
  const identitiesRef = useRef<string[]>([])

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['admin', 'room', roomId, 'participants'],
    queryFn: () =>
      api.get<{ participants: LiveParticipant[]; room: RoomInfo }>(`/api/admin/rooms/${roomId}/participants`),
    refetchInterval: 3_000,
  })

  // Accumulate a bitrate data point on every successful fetch
  useEffect(() => {
    if (!data) return
    const ps = data.participants ?? []
    const now = new Date()
    const label = `${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`

    const point: BitratePoint = { time: label, total: 0 }
    for (const p of ps) {
      const kbps = Math.round(p.tracks.reduce((s, t) => s + (!t.muted ? t.bitrate : 0), 0) / 1000)
      point[p.identity] = kbps
      point.total = (point.total as number) + kbps
      if (!identitiesRef.current.includes(p.identity)) {
        identitiesRef.current = [...identitiesRef.current, p.identity]
      }
    }

    historyRef.current = [...historyRef.current, point].slice(-MAX_HISTORY)
    setBitrateHistory([...historyRef.current])
  }, [data]) // fires only when the query returns new data

  const kick = useMutation({
    mutationFn: (identity: string) => api.post(`/api/admin/rooms/${roomId}/participants/${identity}/kick`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'room', roomId, 'participants'] })
      setConfirmKick(null)
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to kick participant')),
  })

  const suspendRoom = useMutation({
    mutationFn: () => api.post(`/api/admin/rooms/${roomId}/suspend`, {}),
    onSuccess: () => {
      toast.success('Room suspension queued')
      queryClient.invalidateQueries({ queryKey: ['admin', 'room', roomId, 'participants'] })
      queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'], exact: false })
      setConfirmSuspend(false)
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to queue suspension')),
  })

  const deleteRoom = useMutation({
    mutationFn: () => api.delete(`/api/admin/rooms/${roomId}`),
    onSuccess: () => {
      toast.success('Room deletion queued — will be removed shortly')
      queryClient.invalidateQueries({ queryKey: ['admin', 'rooms'], exact: false })
      setTimeout(() => navigate({ to: '/dashboard/admin/rooms' }), 500)
    },
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to queue deletion')),
  })

  const togglePersistent = useMutation({
    mutationFn: (isPersistent: boolean) => {
      const s = room?.settings
      if (!s) throw new Error('no room settings')
      return api.put(`/api/admin/rooms/${roomId}`, {
        settings: {
          allowChat: s.allowChat,
          allowVideo: s.allowVideo,
          allowAudio: s.allowAudio,
          e2ee: s.e2ee,
          isPersistent,
        },
      })
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'room', roomId, 'participants'] }),
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to update room persistence')),
  })

  const mute = useMutation({
    mutationFn: (identity: string) => api.post(`/api/admin/rooms/${roomId}/participants/${identity}/mute`, {}),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'room', roomId, 'participants'] }),
    onError: (err) => toast.error(getErrorMessage(err, 'Failed to mute participant')),
  })

  // ── Recordings section ──────────────────────────────────────────────────────

  const participants = data?.participants ?? []
  const room = data?.room
  const totalBitrate = participants.reduce(
    (sum, p) => sum + p.tracks.reduce((ts, t) => ts + (t.muted ? 0 : t.bitrate), 0),
    0,
  )
  const isAudioOnly =
    participants.length > 0 &&
    totalBitrate === 0 &&
    participants.some((p) => p.tracks.some((t) => !t.muted && t.type === 'AUDIO'))

  // Stable colors for per-participant lines
  // Per-instructions: data viz colors stay hardcoded for distinctness, but brand
  // entries use CSS vars so they respond to theme.
  const LINE_COLORS = [
    'var(--primary)',
    '#10b981',
    '#f59e0b',
    'var(--accent-700)',
    '#ec4899',
    '#f97316',
    '#a855f7',
    '#84cc16',
  ]
  const getColor = (identity: string) => {
    const idx = identitiesRef.current.indexOf(identity)
    return LINE_COLORS[idx % LINE_COLORS.length]
  }

  if (isError) {
    return (
      <div className="mx-auto max-w-5xl px-4 pt-8">
        <div className="border border-destructive/30 bg-destructive/10 px-4 py-4 text-sm flex items-center justify-between">
          <span className="text-destructive">
            {error instanceof Error ? error.message : 'Failed to load room details.'}
          </span>
          <Button variant="outline" size="sm" onClick={() => refetch()}>
            <RefreshCw className="mr-1.5 h-3 w-3" />
            Retry
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 px-4">
      {/* Back + header */}
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={() => navigate({ to: '/dashboard/admin/rooms' })}
          className="p-2 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl font-bold tracking-tight font-mono truncate">{room?.name ?? roomId}</h1>
          <p className="text-xs text-muted-foreground mt-0.5">
            Room detail · auto-refreshes every 10 s
            {dataUpdatedAt ? ` · last updated ${new Date(dataUpdatedAt).toLocaleTimeString()}` : ''}
          </p>
        </div>
        <button
          type="button"
          onClick={() => queryClient.invalidateQueries({ queryKey: ['admin', 'room', roomId, 'participants'] })}
          className="p-2 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
          title="Refresh now"
        >
          <RefreshCw className="h-4 w-4" />
        </button>
      </div>

      {/* Room meta cards */}
      {room && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          {[
            { label: 'Participants', value: participants.length, icon: Users, color: 'var(--primary)' },
            {
              label: 'Publishers',
              value: participants.filter((p) => p.isPublisher).length,
              icon: Activity,
              color: '#10b981',
            },
            {
              label: 'Total bitrate',
              value: isAudioOnly ? 'Audio' : formatBitrate(totalBitrate),
              icon: Activity,
              color: '#f59e0b',
            },
            {
              label: 'Visibility',
              value: room.isPublic ? 'Public' : 'Private',
              icon: room.isPublic ? Globe : Lock,
              color: room.isPublic ? 'var(--primary)' : 'var(--accent-700)',
            },
          ].map(({ label, value, icon: Icon, color }) => (
            <div key={label} className="border p-5" style={{ borderColor: `${color}25`, background: `${color}07` }}>
              <div className="flex items-center justify-between gap-2">
                <div>
                  <p className="text-xl font-bold tracking-tight" style={{ color }}>
                    {value}
                  </p>
                  <p className="text-xs text-muted-foreground mt-0.5">{label}</p>
                </div>
                <Icon className="h-5 w-5 shrink-0" style={{ color }} />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Room actions — suspend / delete */}
      {!isReadOnly && room && (
        <div className="flex items-center gap-3">
          {room.isActive ? (
            confirmSuspend ? (
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => suspendRoom.mutate()}
                  disabled={suspendRoom.isPending}
                  className="inline-flex items-center gap-2 rounded-md bg-amber-500 px-4 py-2 text-sm font-medium text-white transition-opacity hover:opacity-80 disabled:opacity-50"
                >
                  <Power className="h-4 w-4" />
                  Confirm Suspend
                </button>
                <button
                  type="button"
                  onClick={() => setConfirmSuspend(false)}
                  className="px-2 py-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                >
                  ×
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setConfirmSuspend(true)}
                className="inline-flex items-center gap-2 rounded-md border border-amber-500/30 bg-amber-500/10 px-4 py-2 text-sm font-medium text-amber-600 transition-colors hover:bg-amber-500/20"
              >
                <Power className="h-4 w-4" />
                Suspend Room
              </button>
            )
          ) : (
            <span className="inline-flex items-center gap-2 rounded-md border border-border bg-muted px-4 py-2 text-sm font-medium text-muted-foreground">
              <Power className="h-4 w-4" />
              Suspended
            </span>
          )}

          {confirmDelete ? (
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => deleteRoom.mutate()}
                disabled={deleteRoom.isPending}
                className="inline-flex items-center gap-2 rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground transition-opacity hover:opacity-80 disabled:opacity-50"
              >
                <Trash2 className="h-4 w-4" />
                Confirm Delete
              </button>
              <button
                type="button"
                onClick={() => setConfirmDelete(false)}
                className="px-2 py-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                ×
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setConfirmDelete(true)}
              className="inline-flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-4 py-2 text-sm font-medium text-destructive transition-colors hover:bg-destructive/20"
            >
              <Trash2 className="h-4 w-4" />
              Delete Room
            </button>
          )}
        </div>
      )}

      {/* Live bitrate chart — rolling 3-minute window, per-participant lines */}
      <div className="border overflow-hidden" style={{ borderColor: 'var(--border)' }}>
        <div
          className="flex items-center justify-between border-b px-5 py-3"
          style={{
            background: 'linear-gradient(135deg, color-mix(in oklab, var(--primary) 3%, transparent), #f59e0b08)',
          }}
        >
          <p className="text-sm font-semibold">Live bitrate</p>
          <span className="text-xs text-muted-foreground">
            {bitrateHistory.length > 0 ? `${bitrateHistory.length} samples · refreshes every 3 s` : 'waiting for data…'}
          </span>
        </div>
        <div className="p-5">
          {bitrateHistory.length === 0 ? (
            <div className="flex h-28 items-center justify-center">
              <p className="text-xs text-muted-foreground animate-pulse">Collecting data…</p>
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={160}>
              <LineChart data={bitrateHistory} margin={{ top: 4, right: 8, left: -8, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
                <XAxis
                  dataKey="time"
                  tick={{ fontSize: 10, fill: 'var(--muted-foreground)' }}
                  axisLine={false}
                  tickLine={false}
                  interval="preserveStartEnd"
                />
                <YAxis
                  tick={{ fontSize: 10, fill: 'var(--muted-foreground)' }}
                  axisLine={false}
                  tickLine={false}
                  tickFormatter={(v: number) => `${v}k`}
                  width={36}
                />
                <Tooltip
                  // eslint-disable-next-line @typescript-eslint/no-explicit-any
                  formatter={(v: any, name: any) => [`${v} kbps`, name === 'total' ? 'Total' : name]}
                  contentStyle={{
                    background: 'var(--card)',
                    border: '1px solid var(--border)',
                    borderRadius: '8px',
                    fontSize: '11px',
                  }}
                />
                {/* Total line always visible */}
                <Line
                  type="monotone"
                  dataKey="total"
                  stroke="#f59e0b"
                  strokeWidth={2}
                  dot={false}
                  strokeDasharray="4 2"
                  name="total"
                />
                {/* Per-participant lines */}
                {identitiesRef.current.map((identity) => (
                  <Line
                    key={identity}
                    type="monotone"
                    dataKey={identity}
                    stroke={getColor(identity)}
                    strokeWidth={1.5}
                    dot={false}
                    name={identity}
                  />
                ))}
                {identitiesRef.current.length > 1 && (
                  <Legend
                    wrapperStyle={{ fontSize: '11px', paddingTop: '8px' }}
                    formatter={(value) => (value === 'total' ? 'Total' : value)}
                  />
                )}
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Participants table */}
      <div className="border overflow-hidden" style={{ borderColor: 'var(--border)' }}>
        <div
          className="flex items-center justify-between border-b px-5 py-3"
          style={{
            background:
              'linear-gradient(135deg, color-mix(in oklab, var(--primary) 3%, transparent), color-mix(in oklab, var(--accent-700) 3%, transparent))',
          }}
        >
          <p className="text-sm font-semibold">Live participants</p>
          <span className="text-xs text-muted-foreground">{participants.length} in room</span>
        </div>

        {isLoading ? (
          <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
            {[...Array(3)].map((_, i) => (
              <div key={i} className="flex items-center gap-4 px-5 py-4 animate-pulse">
                <div className="h-8 w-8 rounded-full bg-muted" />
                <div className="flex-1 space-y-1.5">
                  <div className="h-4 w-32 rounded-full bg-muted" />
                  <div className="h-3 w-48 rounded-full bg-muted" />
                </div>
              </div>
            ))}
          </div>
        ) : participants.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-12 text-center">
            <Users className="h-8 w-8 text-muted-foreground opacity-40" />
            <p className="text-sm text-muted-foreground">No participants in room</p>
            <p className="text-xs text-muted-foreground">Room may be idle or not yet started in LiveKit</p>
          </div>
        ) : (
          <div className="divide-y" style={{ borderColor: 'var(--border)' }}>
            {participants.map((p) => {
              const audioTracks = p.tracks.filter((t) => t.type === 'AUDIO')
              const videoTracks = p.tracks.filter((t) => t.type === 'VIDEO')
              const totalKbps = Math.round(p.tracks.reduce((s, t) => s + (!t.muted ? t.bitrate : 0), 0) / 1000)
              return (
                <div
                  key={p.sid}
                  className="flex flex-wrap items-center gap-3 px-4 py-3.5 sm:flex-nowrap sm:gap-4 sm:px-5 sm:py-4 hover:bg-muted/20 transition-colors"
                >
                  {/* Avatar */}
                  <div
                    className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-bold text-white"
                    style={{ background: 'linear-gradient(135deg, var(--primary), var(--accent-700))' }}
                  >
                    {(p.name || p.identity).charAt(0).toUpperCase()}
                  </div>

                  {/* Name + identity + tracks */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium truncate">{p.name || p.identity}</p>
                      {p.isPublisher && (
                        <span
                          className="rounded-full px-1.5 py-0.5 text-[10px] font-semibold"
                          style={{ background: '#10b98115', color: '#10b981' }}
                        >
                          publishing
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground font-mono truncate">{p.identity}</p>
                    <div className="mt-1.5 flex flex-wrap gap-1">
                      {audioTracks.map((t) => (
                        <TrackBadge key={t.sid} track={t} />
                      ))}
                      {videoTracks.map((t) => (
                        <TrackBadge key={t.sid} track={t} />
                      ))}
                      {p.tracks.length === 0 && <span className="text-[11px] text-muted-foreground">no tracks</span>}
                    </div>
                  </div>

                  {/* Bitrate + join time */}
                  <div className="text-right shrink-0 space-y-0.5">
                    <p className="text-xs font-mono" style={{ color: '#f59e0b' }}>
                      {totalKbps > 0 ? `${totalKbps} kbps` : '—'}
                    </p>
                    <p className="text-[10px] text-muted-foreground">
                      joined {new Date(p.joinedAt * 1000).toLocaleTimeString()}
                    </p>
                  </div>

                  {/* Actions */}
                  {!isReadOnly && (
                    <div className="flex items-center gap-1 shrink-0">
                      {/* Mute */}
                      <button
                        type="button"
                        onClick={() => mute.mutate(p.identity)}
                        disabled={mute.isPending || audioTracks.every((t) => t.muted)}
                        className="p-1.5 text-muted-foreground transition-colors hover:bg-amber-500/10 hover:text-amber-500 disabled:opacity-30 disabled:cursor-not-allowed"
                        title="Mute audio"
                      >
                        {audioTracks.every((t) => t.muted) ? (
                          <MicOff className="h-4 w-4" />
                        ) : (
                          <Mic className="h-4 w-4" />
                        )}
                      </button>

                      {/* Kick */}
                      {confirmKick === p.identity ? (
                        <div className="flex items-center gap-1">
                          <button
                            type="button"
                            onClick={() => kick.mutate(p.identity)}
                            disabled={kick.isPending}
                            className="px-2 py-1 text-xs font-semibold text-white"
                            style={{ background: '#ef4444' }}
                          >
                            Kick
                          </button>
                          <button
                            type="button"
                            onClick={() => setConfirmKick(null)}
                            className="px-1.5 py-1 text-xs text-muted-foreground hover:text-foreground"
                          >
                            ×
                          </button>
                        </div>
                      ) : (
                        <button
                          type="button"
                          onClick={() => setConfirmKick(p.identity)}
                          className="p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                          title="Kick participant"
                        >
                          <UserX className="h-4 w-4" />
                        </button>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Room settings info */}
      {room?.settings && (
        <div className="border overflow-hidden" style={{ borderColor: 'var(--border)' }}>
          <div
            className="border-b px-5 py-3"
            style={{
              background:
                'linear-gradient(135deg, color-mix(in oklab, var(--primary) 3%, transparent), color-mix(in oklab, var(--accent-700) 3%, transparent))',
            }}
          >
            <p className="text-sm font-semibold">Room configuration</p>
          </div>
          <div
            className="grid grid-cols-2 sm:grid-cols-5 gap-0 divide-x divide-y"
            style={{ borderColor: 'var(--border)' }}
          >
            {[
              { label: 'Chat', enabled: room.settings.allowChat },
              { label: 'Video', enabled: room.settings.allowVideo },
              { label: 'Audio', enabled: room.settings.allowAudio },
              { label: 'E2EE', enabled: room.settings.e2ee },
            ].map(({ label, enabled }) => (
              <div key={label} className="flex flex-col items-center gap-1 py-5">
                <span className="h-2 w-2 rounded-full" style={{ background: enabled ? '#10b981' : '#ef4444' }} />
                <p className="text-xs font-medium">{label}</p>
                <p className="text-[10px] text-muted-foreground">{enabled ? 'allowed' : 'disabled'}</p>
              </div>
            ))}
            {!isReadOnly ? (
              <button
                type="button"
                onClick={() => togglePersistent.mutate(!room.settings!.isPersistent)}
                disabled={togglePersistent.isPending}
                className="flex flex-col items-center gap-1 py-5 transition-colors hover:bg-muted/30 disabled:opacity-50"
                title={
                  room.settings.isPersistent
                    ? 'Room stays active during idle periods'
                    : 'Room will be deactivated when idle'
                }
              >
                <Pin
                  className="h-2 w-2"
                  style={{ color: room.settings.isPersistent ? 'var(--primary)' : 'var(--muted-foreground)' }}
                />
                <p className="text-xs font-medium">Persistent</p>
                <p className="text-[10px] text-muted-foreground">{room.settings.isPersistent ? 'on' : 'off'}</p>
              </button>
            ) : (
              <div className="flex flex-col items-center gap-1 py-5">
                <Pin
                  className="h-2 w-2"
                  style={{ color: room.settings.isPersistent ? 'var(--primary)' : 'var(--muted-foreground)' }}
                />
                <p className="text-xs font-medium">Persistent</p>
                <p className="text-[10px] text-muted-foreground">{room.settings.isPersistent ? 'on' : 'off'}</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* TODO oncoming feature — Recordings section removed */}

      <p className="text-center text-xs text-muted-foreground">
        <Link to="/dashboard/admin/rooms" className="hover:text-foreground underline-offset-4 hover:underline">
          ← Back to all rooms
        </Link>
      </p>
    </div>
  )
}
