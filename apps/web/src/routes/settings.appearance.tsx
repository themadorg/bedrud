import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/settings/appearance')({
  head: () => ({ meta: [{ title: 'Appearance — Bedrud' }] }),
  component: AppearanceSettingsPage,
})

// The `/settings` layout renders every panel and scrolls to the one this route names, so the route
// itself carries only its title.
function AppearanceSettingsPage() {
  return null
}
