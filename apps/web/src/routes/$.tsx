import { createFileRoute } from '@tanstack/react-router'
import { ErrorPage } from '@/components/ErrorPage'

// @ts-expect-error - Catch-all route ($). Route types are generated at dev/build time.
export const Route = createFileRoute('/$')({
  component: NotFound,
})

function NotFound() {
  return <ErrorPage variant="not-found" />
}
