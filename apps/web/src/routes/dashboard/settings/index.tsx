import { createFileRoute } from '@tanstack/react-router'
import { ProfileSettingsPanel } from '#/components/settings/ProfileSettingsPanel'

export const Route = createFileRoute('/dashboard/settings/')({
  component: ProfilePage,
})

function ProfilePage() {
  return <ProfileSettingsPanel />
}
