import { createFileRoute } from '@tanstack/react-router'
import { VideoSettingsPanel } from '#/components/settings/VideoSettingsPanel'

export const Route = createFileRoute('/dashboard/settings/video')({
  component: VideoPage,
})

function VideoPage() {
  return <VideoSettingsPanel />
}
