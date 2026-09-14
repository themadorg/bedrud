import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/settings/audio')({
  head: () => ({ meta: [{ title: 'Audio — Bedrud' }] }),
  component: AudioSettingsPage,
})

// The `/settings` layout renders every panel and scrolls to the one this route names, so the route
// itself carries only its title.
function AudioSettingsPage() {
  return null
}
