import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/settings/video')({
  head: () => ({ meta: [{ title: 'Video — Bedrud' }] }),
  component: VideoSettingsPage,
})

// The `/settings` layout renders every panel and scrolls to the one this route names, so the route
// itself carries only its title.
function VideoSettingsPage() {
  return null
}
