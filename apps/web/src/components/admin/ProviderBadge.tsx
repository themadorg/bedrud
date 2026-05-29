import { Badge } from '#/components/ui/badge'

const PROVIDER_STYLE: Record<string, { bg: string; color: string }> = {
  local: { bg: 'color-mix(in oklab, var(--primary) 8%, transparent)', color: 'var(--accent-400)' },
  google: { bg: '#ef444415', color: '#f87171' },
  github: { bg: '#71717a15', color: '#a1a1aa' },
  guest: { bg: '#f59e0b15', color: '#fbbf24' },
  passkey: { bg: '#10b98115', color: '#34d399' },
}

interface ProviderBadgeProps {
  provider: string
  className?: string
}

export function ProviderBadge({ provider, className }: ProviderBadgeProps) {
  const s = PROVIDER_STYLE[provider] ?? {
    bg: 'color-mix(in oklab, var(--primary) 8%, transparent)',
    color: 'var(--accent-400)',
  }
  return (
    <Badge
      variant="outline"
      className={`text-xs font-semibold uppercase tracking-wider px-2.5 py-1 ${className ?? ''}`}
      style={{ background: s.bg, color: s.color, borderColor: s.color + '30' }}
    >
      {provider}
    </Badge>
  )
}
