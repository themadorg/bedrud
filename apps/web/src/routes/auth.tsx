import { createFileRoute, Link, Outlet, redirect } from '@tanstack/react-router'
import { Radio } from 'lucide-react'
import { useAuthStore } from '#/lib/auth.store'
import { ThemeToggle } from '@/components/ThemeToggle'

export const Route = createFileRoute('/auth')({
  beforeLoad: async () => {
    if (typeof window === 'undefined') return
    await useAuthStore.getState().initialize()
    if (useAuthStore.getState().tokens) throw redirect({ to: '/dashboard' })
  },
  component: AuthLayout,
})

// Waveform is a purely decorative CSS-animated visualization.
// No shadcn equivalent exists — this is a custom presentational component.
function Waveform() {
  const bars = [3, 6, 9, 5, 8, 4, 7, 10, 6, 4, 8, 5, 9, 3, 7]
  return (
    <div className="flex items-end gap-[3px]" aria-hidden>
      {bars.map((h, i) => (
        <span
          key={i}
          className="w-[3px] bg-white/30"
          style={{
            height: `${h * 4}px`,
            animation: `wave 1.4s ease-in-out ${(i * 0.09).toFixed(2)}s infinite alternate`,
          }}
        />
      ))}
    </div>
  )
}

function AuthLayout() {
  return (
    <div className="flex min-h-screen bg-background">
      <style>{`
        @keyframes wave {
          from { transform: scaleY(0.3); opacity: 0.3; }
          to   { transform: scaleY(1);   opacity: 1;   }
        }
      `}</style>

      {/* ── Left brand panel (always dark) ─────────────────────────────── */}
      <div
        className="relative hidden w-[420px] shrink-0 flex-col justify-between overflow-hidden p-10 lg:flex"
        style={{
          background:
            'linear-gradient(160deg, oklch(0.14 0.025 270) 0%, oklch(0.12 0.03 270) 40%, oklch(0.10 0.03 270) 100%)',
        }}
      >
        {/* Grid texture */}
        <div
          className="pointer-events-none absolute inset-0 opacity-[0.04]"
          style={{
            backgroundImage:
              'linear-gradient(var(--primary) 1px, transparent 1px), linear-gradient(90deg, var(--primary) 1px, transparent 1px)',
            backgroundSize: '60px 60px',
          }}
          aria-hidden
        />

        {/* Single static radial glow — no animated blobs per DESIGN.md */}

        {/* Logo */}
        <div className="relative flex items-center gap-3">
          <div
            className="flex h-9 w-9 items-center justify-center"
            style={{
              background: 'linear-gradient(135deg, var(--primary) 0%, var(--accent-600) 100%)',
              boxShadow: '0 2px 16px color-mix(in oklab, var(--primary) 31%, transparent)',
            }}
          >
            <Radio className="h-4 w-4 text-white" />
          </div>
          <span className="text-lg font-semibold text-white">bedrud</span>
        </div>

        {/* Center content */}
        <div className="relative space-y-8">
          <Waveform />
          <div className="space-y-4">
            <p className="text-2xl font-bold leading-snug text-white">
              Voice-first meetings,
              <br />
              <span className="text-[var(--accent-300)]">built for humans.</span>
            </p>
            <p className="text-sm leading-relaxed text-white/40">
              Instant rooms. No installs. Just open a room and start talking — with anyone, anywhere.
            </p>
          </div>

          {/* Trust badges */}
          <div className="flex flex-col gap-2">
            {['End-to-end encrypted', 'Zero telemetry', 'Your infrastructure'].map((item) => (
              <div key={item} className="flex items-center gap-2 text-xs text-white/40">
                <span
                  className="h-1.5 w-1.5 shrink-0"
                  style={{ background: 'linear-gradient(135deg, var(--primary), var(--accent-600))' }}
                />
                {item}
              </div>
            ))}
          </div>
        </div>

        {/* Bottom */}
        <a
          href="https://bedrud.org?utm_source=app&utm_medium=footer"
          target="_blank"
          rel="noopener noreferrer"
          className="relative text-xs text-white/20 transition-colors hover:text-white/40"
        >
          <span suppressHydrationWarning>© {new Date().getFullYear()} Bedrud</span>
        </a>
      </div>

      {/* ── Right form panel ───────────────────────────────────────────── */}
      <div className="flex flex-1 flex-col">
        {/* Top bar */}
        <div className="flex items-center justify-between px-6 py-4">
          <Link
            to="/"
            className="flex items-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground lg:hidden"
          >
            <Radio className="h-4 w-4" />
            bedrud
          </Link>
          <div className="ml-auto">
            <ThemeToggle />
          </div>
        </div>

        {/* Form area */}
        <div className="flex flex-1 items-center justify-center px-6 py-8">
          <div className="w-full max-w-[360px]">
            <Outlet />
          </div>
        </div>
      </div>
    </div>
  )
}
