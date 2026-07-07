export type ParticipantPalette = {
  tile: string
  avatar: string
  glow: string
}

export const PALETTES: ParticipantPalette[] = [
  { tile: 'rgba(13,148,136,0.09)', avatar: 'linear-gradient(135deg,#0d9488,#14b8a6)', glow: 'rgba(13,148,136,0.28)' },
  { tile: 'rgba(59,130,246,0.08)', avatar: 'linear-gradient(135deg,#3b82f6,#6366f1)', glow: 'rgba(59,130,246,0.26)' },
  { tile: 'rgba(99,102,241,0.08)', avatar: 'linear-gradient(135deg,#6366f1,#818cf8)', glow: 'rgba(99,102,241,0.26)' },
  { tile: 'rgba(14,165,233,0.08)', avatar: 'linear-gradient(135deg,#0ea5e9,#38bdf8)', glow: 'rgba(14,165,233,0.26)' },
  { tile: 'rgba(16,185,129,0.08)', avatar: 'linear-gradient(135deg,#10b981,#34d399)', glow: 'rgba(16,185,129,0.26)' },
  { tile: 'rgba(139,92,246,0.08)', avatar: 'linear-gradient(135deg,#8b5cf6,#a78bfa)', glow: 'rgba(139,92,246,0.26)' },
  { tile: 'rgba(6,182,212,0.08)', avatar: 'linear-gradient(135deg,#0891b2,#22d3ee)', glow: 'rgba(6,182,212,0.26)' },
  { tile: 'rgba(71,85,105,0.08)', avatar: 'linear-gradient(135deg,#475569,#64748b)', glow: 'rgba(71,85,105,0.22)' },
]

export function getPalette(name: string) {
  const hash = name.split('').reduce((a, c) => a + c.charCodeAt(0), 0)
  return PALETTES[Math.abs(hash) % PALETTES.length]
}
