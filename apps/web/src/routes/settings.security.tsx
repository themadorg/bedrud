import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/settings/security')({
  head: () => ({ meta: [{ title: 'Security — Bedrud' }] }),
  component: SecuritySettingsPage,
})

// The `/settings` layout renders every panel and scrolls to the one this route names, so the route
// itself carries only its title.
function SecuritySettingsPage() {
  return null
}
