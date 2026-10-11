import { createFileRoute, Outlet, useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'
import { loadRegisteredUser, requireRegisteredUser } from '#/lib/require-registered-user'
import { AppVersion } from '@/components/AppVersion'
import { MobileOnlyGate } from '@/components/dashboard/MobileOnlyGate'
import { AppearanceSettingsPanel } from '@/components/settings/AppearanceSettingsPanel'
import { AudioSettingsPanel } from '@/components/settings/AudioSettingsPanel'
import { ExperimentalSettingsPanel } from '@/components/settings/ExperimentalSettingsPanel'
import { SecuritySettingsPanel } from '@/components/settings/SecuritySettingsPanel'
import {
  SETTINGS_SECTIONS,
  type SettingsSectionId,
  sectionIdFromHash,
  sectionIdFromPathname,
} from '@/components/settings/settingsSections'
import { VideoSettingsPanel } from '@/components/settings/VideoSettingsPanel'

export const Route = createFileRoute('/settings')({
  beforeLoad: requireRegisteredUser,
  loader: loadRegisteredUser,
  staleTime: Infinity,
  head: () => ({ meta: [{ title: 'Settings — Bedrud' }] }),
  component: SettingsLayout,
})

/** Matches the Android section header: a small label in the primary colour above its card. */
const SECTION_HEADER_CLASS = 'text-xs font-semibold uppercase tracking-wide text-primary'

/** Renders the panel that belongs to a section. */
function SettingsSectionPanel({ id }: { id: SettingsSectionId }) {
  switch (id) {
    case 'appearance':
      return <AppearanceSettingsPanel />
    case 'audio':
      return <AudioSettingsPanel />
    case 'video':
      return <VideoSettingsPanel />
    case 'security':
      return <SecuritySettingsPanel />
    case 'experimental':
      return <ExperimentalSettingsPanel />
  }
}

/**
 * Scrolls the section named by the current path, or by the hash, into view. A sub-route such as
 * `/settings/audio` renders this same page, so the scroll is what makes a bookmark or a link land
 * on its section.
 *
 * This renders inside the gate rather than beside it. `MobileOnlyGate` returns null until its own
 * effect has confirmed the viewport, so an effect placed in the layout would run while the sections
 * are still absent from the document, find nothing, and never run again.
 */
function SettingsSectionScroll({ pathname, hash }: { pathname: string; hash: string }) {
  useEffect(() => {
    const sectionId = sectionIdFromPathname(pathname) ?? sectionIdFromHash(hash)
    if (!sectionId) return
    document.getElementById(sectionId)?.scrollIntoView({ block: 'start' })
  }, [pathname, hash])

  return null
}

function SettingsLayout() {
  const { location } = useRouterState()

  return (
    <MobileOnlyGate desktopTo="/dashboard/settings">
      <SettingsSectionScroll pathname={location.pathname} hash={location.hash} />

      {/* Sections sit 24px apart, matching the gap the panels already use between their own cards. */}
      <div className="space-y-6">
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>

        {SETTINGS_SECTIONS.map(({ id, label }) => (
          <section key={id} aria-labelledby={id} className="space-y-2">
            {/* The heading is the scroll target, so the breathing room belongs on it, not the section. */}
            <h2 id={id} className={`scroll-mt-4 ${SECTION_HEADER_CLASS}`}>
              {label}
            </h2>
            <SettingsSectionPanel id={id} />
          </section>
        ))}

        <AppVersion className="text-center" />
      </div>

      {/* Renders the matched sub-route so its document title applies; the sub-routes draw nothing. */}
      <Outlet />
    </MobileOnlyGate>
  )
}
