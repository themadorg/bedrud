import { createFileRoute } from '@tanstack/react-router'
import { AudioSettingsPanel } from '@/components/settings/AudioSettingsPanel'

export const Route = createFileRoute('/settings/audio')({
  head: () => ({ meta: [{ title: 'Audio — Bedrud' }] }),
  component: AudioSettingsPage,
})

function AudioSettingsPage() {
  return (
    <div className="space-y-3">
      <h1 className="text-lg font-semibold tracking-tight">Audio</h1>
      <AudioSettingsPanel />
    </div>
  )
}
