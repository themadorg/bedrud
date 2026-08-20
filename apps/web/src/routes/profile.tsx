import { createFileRoute } from '@tanstack/react-router'
import { loadRegisteredUser, requireRegisteredUser } from '#/lib/require-registered-user'
import { MobileOnlyGate } from '@/components/dashboard/MobileOnlyGate'
import { ProfileSettingsPanel } from '@/components/settings/ProfileSettingsPanel'

export const Route = createFileRoute('/profile')({
  beforeLoad: requireRegisteredUser,
  loader: loadRegisteredUser,
  staleTime: Infinity,
  head: () => ({ meta: [{ title: 'Profile — Bedrud' }] }),
  component: ProfilePage,
})

function ProfilePage() {
  return (
    <MobileOnlyGate desktopTo="/dashboard/settings">
      <h1 className="mb-4 text-xl font-semibold tracking-tight">Profile</h1>
      <ProfileSettingsPanel />
    </MobileOnlyGate>
  )
}
