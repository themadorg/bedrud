import { toast } from 'sonner'

/**
 * The room's address is the page the participant is already on. Every affordance that hands the room
 * to somebody else — the controls bar's row, the invite sheet's target, the invite sheet's link row —
 * reads it from here, so there is one answer to what the room's address is.
 *
 * Returns an empty string during a server render, where there is no location to read.
 */
export function meetingLink(): string {
  if (typeof window === 'undefined') return ''
  return window.location.href
}

/** Copies the room link and reports the result, so every copy affordance says the same thing. */
export async function copyMeetingLink(): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(meetingLink())
    toast.success('Meeting link copied', {
      description: 'Share it so others can join this room.',
    })
    return true
  } catch {
    toast.error('Could not copy link', {
      description: 'Check clipboard permissions and try again.',
    })
    return false
  }
}
