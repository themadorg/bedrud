import { createFileRoute } from '@tanstack/react-router'

import { AdminOverviewPage } from '#/components/admin/overview'

export const Route = createFileRoute('/dashboard/admin/')({
  component: AdminOverviewRoute,
})

function AdminOverviewRoute() {
  return <AdminOverviewPage />
}
