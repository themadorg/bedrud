import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/settings/experimental')({
  head: () => ({ meta: [{ title: 'Experimental — Bedrud' }] }),
  component: ExperimentalSettingsPage,
})

// The `/settings` layout renders every panel and scrolls to the one this route names, so the route
// itself carries only its title.
function ExperimentalSettingsPage() {
  return null
}
