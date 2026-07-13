import { createFileRoute } from '@tanstack/react-router'
import { AdminSettingsPage, requireAdminSettingsAccess } from './settings'

/**
 * Deep link: /dashboard/admin/settings/audio
 * Instance-level Krisp enablement (not personal user settings).
 */
export const Route = createFileRoute('/dashboard/admin/settings_/audio')({
  beforeLoad: requireAdminSettingsAccess,
  component: () => <AdminSettingsPage initialTab="audio" />,
})
