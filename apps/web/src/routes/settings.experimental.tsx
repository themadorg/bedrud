import { createFileRoute } from '@tanstack/react-router'
import { ExperimentalSettingsPanel } from '@/components/settings/ExperimentalSettingsPanel'

export const Route = createFileRoute('/settings/experimental')({
  head: () => ({ meta: [{ title: 'Experimental — Bedrud' }] }),
  component: ExperimentalSettingsPage,
})

function ExperimentalSettingsPage() {
  return (
    <div className="space-y-3">
      <h1 className="text-lg font-semibold tracking-tight">Experimental</h1>
      <ExperimentalSettingsPanel />
    </div>
  )
}
