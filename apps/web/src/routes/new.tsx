import { createFileRoute, redirect } from '@tanstack/react-router'
import { api } from '#/lib/api'
import { useAuthStore } from '#/lib/auth.store'
import { isGuestToken } from '#/lib/jwt-user'
import { useRecentRoomsStore } from '#/lib/recent-rooms.store'
import { isGuestUser, useUserStore } from '#/lib/user.store'
import { ErrorPage } from '@/components/ErrorPage'
import { MeetLoadingScreen } from '@/components/meeting/MeetLoadingScreen'

const NEW_MEETING_DEFAULTS = {
  isPublic: true,
  maxParticipants: 20,
  settings: {
    allowChat: true,
    allowVideo: false,
    allowAudio: true,
    requireApproval: false,
    e2ee: false,
    isPersistent: false,
  },
} as const

export const Route = createFileRoute('/new')({
  ssr: false,
  head: () => ({ meta: [{ title: 'New Meeting — Bedrud' }] }),
  beforeLoad: async () => {
    if (typeof window === 'undefined') return
    await useAuthStore.getState().initialize()
    const tokens = useAuthStore.getState().tokens
    if (!tokens) {
      throw redirect({ to: '/auth/login', search: { redirect: '/new' } })
    }
    if (isGuestToken(tokens.accessToken) || isGuestUser(useUserStore.getState().user)) {
      throw redirect({ to: '/' })
    }
  },
  loader: async () => {
    if (typeof window === 'undefined') return
    const room = await api.post<{ name: string }>('/api/room/create', NEW_MEETING_DEFAULTS)
    useRecentRoomsStore.getState().add(room.name)
    throw redirect({ to: '/m/$meetId', params: { meetId: room.name }, replace: true })
  },
  pendingComponent: NewMeetingPending,
  errorComponent: ({ error }) => (
    <ErrorPage variant="room-error" error={error instanceof Error ? error.message : String(error)} />
  ),
  component: () => null,
})

function NewMeetingPending() {
  return <MeetLoadingScreen label="Creating meeting…" />
}
