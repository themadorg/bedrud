import { createFileRoute } from '@tanstack/react-router'
import { AppearanceSettingsPanel } from '@/components/settings/AppearanceSettingsPanel'

export const Route = createFileRoute('/settings/appearance')({
  head: () => ({ meta: [{ title: 'Appearance — Bedrud' }] }),
  component: AppearanceSettingsPage,
})

function AppearanceSettingsPage() {
  return (
    <div className="space-y-3">
      <h1 className="text-lg font-semibold tracking-tight">Appearance</h1>
      <AppearanceSettingsPanel />
    </div>
  )
}
