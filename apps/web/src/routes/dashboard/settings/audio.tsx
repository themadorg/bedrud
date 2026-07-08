import { createFileRoute } from '@tanstack/react-router'
import { AudioSettingsPanel } from '#/components/settings/AudioSettingsPanel'

export const Route = createFileRoute('/dashboard/settings/audio')({
  component: AudioPage,
})

function AudioPage() {
  return <AudioSettingsPanel />
}
