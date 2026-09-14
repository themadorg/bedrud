/**
 * The share targets the invite sheet offers, and the only place that decides which ones exist.
 *
 * On a phone `navigator.share` opens the system sheet, which already lists every installed app and
 * hands the link to the installed Telegram rather than to a browser tab, so a named Telegram button
 * beside it would be the worse version of a target the platform already provides. Where sharing is
 * missing — a desktop browser at phone width, an older engine — the row would be a dead end, so the
 * named services take its place there and only there.
 */

/** Identifies a target to the sheet, which decides what activating it does. */
export type InviteShareTargetId = 'share' | 'copy' | 'qr' | 'email' | 'telegram' | 'whatsapp'

export interface InviteShareTarget {
  id: InviteShareTargetId
  label: string
  /** Set on the targets that are plain links. The sheet handles the others itself. */
  href?: string
}

/** Returns the targets for a room link, given whether this engine can open the system share sheet. */
export function inviteShareTargets(roomLink: string, canShare: boolean): InviteShareTarget[] {
  const encodedLink = encodeURIComponent(roomLink)
  const alwaysPresent: InviteShareTarget[] = [
    { id: 'copy', label: 'Copy link' },
    { id: 'qr', label: 'QR code' },
  ]

  if (canShare) {
    return [{ id: 'share', label: 'Share' }, ...alwaysPresent]
  }

  return [
    ...alwaysPresent,
    { id: 'email', label: 'Email', href: `mailto:?body=${encodedLink}` },
    { id: 'telegram', label: 'Telegram', href: `https://t.me/share/url?url=${encodedLink}` },
    { id: 'whatsapp', label: 'WhatsApp', href: `https://wa.me/?text=${encodedLink}` },
  ]
}
