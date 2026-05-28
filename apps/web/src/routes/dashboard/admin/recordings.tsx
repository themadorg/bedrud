// TODO oncoming feature
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/dashboard/admin/recordings')({
  component: AdminRecordingsPage,
})

function AdminRecordingsPage() {
  return (
    <div className="mx-auto max-w-6xl px-4 pb-8">
      <h1 className="text-sm font-semibold">Recordings</h1>
      <p className="text-sm text-muted-foreground mt-2">Coming in a future release.</p>
    </div>
  )
}
