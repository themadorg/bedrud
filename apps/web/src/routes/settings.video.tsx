import { createFileRoute } from '@tanstack/react-router'
import { VideoSettingsPanel } from '@/components/settings/VideoSettingsPanel'

export const Route = createFileRoute('/settings/video')({
  head: () => ({ meta: [{ title: 'Video — Bedrud' }] }),
  component: VideoSettingsPage,
})

function VideoSettingsPage() {
  return (
    <div className="space-y-3">
      <h1 className="text-lg font-semibold tracking-tight">Video</h1>
      <VideoSettingsPanel />
    </div>
  )
}
