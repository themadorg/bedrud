import {
  AudioLines,
  Check,
  Film,
  Globe,
  Headphones,
  Info,
  Link2,
  Lock,
  Maximize,
  Mic,
  Package,
  PenLine,
  Settings,
  Video,
  Volume2,
} from 'lucide-react'
import type { MeetingFixedRowId, MeetingOptionRow, MeetingOptionRowId } from '@/components/meeting/meetingOptionRows'
import { cn } from '@/lib/utils'

/** The fixed rows that draw an icon. Headings draw none, so they are excluded. */
type IconRowId = Exclude<MeetingFixedRowId, `heading:${string}`>

/**
 * The icon each fixed row wears. Keyed by id so the pure row list stays free of React values, and
 * so a new fixed row id fails the type check here until it is given one.
 */
const ROW_ICONS: Record<IconRowId, React.ReactNode> = {
  videos: <Video size={18} className="shrink-0" />,
  access: <Globe size={18} className="shrink-0" />,
  info: <Info size={18} className="shrink-0" />,
  'copy-link': <Link2 size={18} className="shrink-0" />,
  deafen: <Headphones size={18} className="shrink-0" />,
  settings: <Settings size={18} className="shrink-0" />,
  fullscreen: <Maximize size={18} className="shrink-0" />,
  whiteboard: <PenLine size={18} className="shrink-0" />,
  youtube: <Film size={18} className="shrink-0" />,
  'app-gallery': <Package size={18} className="shrink-0" />,
}

/** The icon a device row wears, chosen by the prefix that carries its device id. */
function deviceIcon(id: MeetingOptionRowId): React.ReactNode | undefined {
  if (id.startsWith('microphone:')) return <Mic size={18} className="shrink-0" />
  if (id.startsWith('speaker:')) return <Volume2 size={18} className="shrink-0" />
  if (id.startsWith('noise:')) return <AudioLines size={18} className="shrink-0" />
  return undefined
}

/** Reads the icon map without widening it, so its exhaustiveness survives the lookup. */
function fixedIcon(id: MeetingOptionRowId): React.ReactNode | undefined {
  return id in ROW_ICONS ? ROW_ICONS[id as IconRowId] : undefined
}

/**
 * The private-room row is the one place the icon depends on the label rather than the id.
 *
 * Exported because the desktop dropdown in `ControlsBar` draws the same rows; one lookup for both
 * keeps a new row from picking up an icon in the panel and none in the menu.
 */
export function meetingOptionIcon(row: MeetingOptionRow): React.ReactNode {
  if (row.id === 'access' && row.label === 'Private room') return <Lock size={18} className="shrink-0" />
  return deviceIcon(row.id) ?? fixedIcon(row.id)
}

interface MeetingOptionsPanelProps {
  rows: MeetingOptionRow[]
  expanded: boolean
  onSelect: (id: MeetingOptionRowId) => void
}

/**
 * The room options, unfolded above the controls inside the same surface. Collapsed it has no
 * height and no tab stops; it is not unmounted, so the height transition has something to animate
 * from.
 */
export function MeetingOptionsPanel({ rows, expanded, onSelect }: MeetingOptionsPanelProps) {
  return (
    <div
      // 320ms opening, 260ms closing — Android's two durations, which differ on purpose.
      className={cn(
        'w-full overflow-hidden transition-[max-height,opacity] ease-[cubic-bezier(0.4,0,0.2,1)]',
        expanded ? 'opacity-100 duration-[320ms]' : 'max-h-0 opacity-0 duration-[260ms]',
      )}
      style={expanded ? { maxHeight: 'var(--meet-controls-panel-max-height)' } : undefined}
      aria-hidden={!expanded}
    >
      <ul className="max-h-[calc(var(--meet-controls-panel-max-height)-6rem)] list-none overflow-y-auto px-2 py-1">
        {rows.map((row) =>
          row.kind === 'heading' ? (
            <li
              key={row.id}
              className="px-3 pt-3 pb-1 font-semibold text-[11px] text-[var(--meet-fg-subtle)] uppercase tracking-wide"
            >
              {row.label}
            </li>
          ) : (
            <li key={row.id}>
              <button
                type="button"
                disabled={row.disabled || !expanded}
                onClick={() => onSelect(row.id)}
                className={cn(
                  'flex w-full items-center gap-3 rounded-md px-3 py-3 text-left text-[14px] transition-colors duration-150',
                  row.checked ? 'text-[var(--meet-btn-muted-fg)]' : 'text-[var(--meet-fg-strong)]',
                  row.disabled ? 'cursor-not-allowed opacity-40' : 'hover:bg-[var(--meet-control-hover)]',
                )}
              >
                {meetingOptionIcon(row)}
                <span className="flex-1 truncate">{row.label}</span>
                {row.kind === 'toggle' && row.checked && (
                  <Check size={16} className="shrink-0 text-[var(--meet-btn-muted-fg)]" />
                )}
              </button>
            </li>
          ),
        )}
      </ul>
    </div>
  )
}
