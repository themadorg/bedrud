import { createFileRoute } from '@tanstack/react-router'
import { QueueStatsPage } from '#/components/admin/queue-stats'

export const Route = createFileRoute('/dashboard/admin/queue')({
  component: QueueStatsRoute,
})

function QueueStatsRoute() {
  return <QueueStatsPage />
}
