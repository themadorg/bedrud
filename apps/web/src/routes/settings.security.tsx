import { createFileRoute } from '@tanstack/react-router'
import { SecuritySettingsPanel } from '@/components/settings/SecuritySettingsPanel'

export const Route = createFileRoute('/settings/security')({
  head: () => ({ meta: [{ title: 'Security — Bedrud' }] }),
  component: SecuritySettingsPage,
})

function SecuritySettingsPage() {
  return (
    <div className="space-y-3">
      <h1 className="text-lg font-semibold tracking-tight">Security</h1>
      <SecuritySettingsPanel />
    </div>
  )
}
