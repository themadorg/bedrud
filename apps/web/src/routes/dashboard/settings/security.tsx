import { createFileRoute } from '@tanstack/react-router'
import { SecuritySettingsPanel } from '#/components/settings/SecuritySettingsPanel'

export const Route = createFileRoute('/dashboard/settings/security')({
  component: SecurityPage,
})

function SecurityPage() {
  return <SecuritySettingsPanel />
}
