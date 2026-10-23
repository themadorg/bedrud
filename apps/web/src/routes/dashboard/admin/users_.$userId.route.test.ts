// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const userDetailRouteSource = readFileSync(new URL('./users_.$userId.tsx', import.meta.url), 'utf8')

// The ban, logout and delete buttons sit between the role picker and the name block.
const headerActionsStart = userDetailRouteSource.indexOf('setBanDialogOpen(true)')
const headerActionsEnd = userDetailRouteSource.indexOf('{/* Name + badges */}')
const headerActionsSource = userDetailRouteSource.slice(headerActionsStart, headerActionsEnd)

// The whole header control row, the role picker included, overlaps the cover banner.
const headerRowSource = userDetailRouteSource.slice(userDetailRouteSource.indexOf('{/* Actions */}'), headerActionsEnd)

function buttonSource(ariaLabel: string) {
  const labelIndex = headerActionsSource.indexOf(`aria-label="${ariaLabel}"`)
  const buttonStart = headerActionsSource.lastIndexOf('<Button', labelIndex)
  return headerActionsSource.slice(buttonStart, headerActionsSource.indexOf('</Button>', labelIndex))
}

describe('the admin user detail header actions', () => {
  it('should find the header action block', () => {
    expect(headerActionsStart).toBeGreaterThan(-1)
    expect(headerActionsEnd).toBeGreaterThan(headerActionsStart)
  })

  // Light amber and red text on a nearly transparent hex tint read as disabled on the light theme.
  it('should take its colours from theme tokens rather than inline hex', () => {
    expect(headerActionsSource).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
    expect(headerActionsSource).not.toContain('style=')
  })

  // The button used to show the state, so clicking "Active" banned the user. The badge under the
  // name already shows the state, so the button names what it does.
  it('should name the status action rather than the current state', () => {
    expect(headerActionsSource).toMatch(/user\.isActive \? 'Ban' : 'Unban'/)
    expect(headerActionsSource).not.toMatch(/>\s*Active\s*</)
  })

  // Destructive colour is reserved for irreversible actions; a ban and a logout can both be undone.
  it('should reserve destructive colour for deleting the user', () => {
    expect(buttonSource('Delete user')).toContain('text-destructive')
    expect(buttonSource('Force logout')).not.toContain('destructive')
    expect(headerActionsSource.slice(0, headerActionsSource.indexOf('</Button>'))).not.toContain('destructive')
  })

  // The buttons overlap the cover banner, so a translucent hover lets the banner show through and
  // the button fades the moment it is pressed.
  it('should keep every header action opaque on hover', () => {
    expect(headerActionsSource).not.toMatch(/hover:bg-[\w-]+\/\d+/)
    expect(buttonSource('Delete user')).toContain('hover:bg-destructive hover:text-destructive-foreground')
  })

  // A disabled control drops to half opacity, which over the banner reads as a fade on every save.
  // Each control that disables itself while saving stays opaque and shows a spinner instead.
  it('should keep header controls opaque while a save is pending', () => {
    const pendingControlCount = headerRowSource.match(/disabled=\{\w+\.isPending\}/g)?.length ?? 0
    const opaqueControlCount = headerRowSource.match(/disabled:opacity-100/g)?.length ?? 0

    expect(pendingControlCount).toBeGreaterThan(0)
    expect(opaqueControlCount).toBe(pendingControlCount)
  })

  it('should show a spinner while the status is saving', () => {
    expect(headerRowSource).toMatch(/toggleStatus\.isPending \? \(?\s*<Loader2 className="[^"]*animate-spin/)
  })

  // Ban and Unban swap their label and icon, so a content-sized button jumps between states.
  it('should give the status button a fixed width', () => {
    // The attributes run from the opening tag to the comment that starts the button's children.
    const buttonStart = userDetailRouteSource.lastIndexOf('<Button', headerActionsStart)
    const buttonAttributes = userDetailRouteSource.slice(
      buttonStart,
      userDetailRouteSource.indexOf('{/*', headerActionsStart),
    )

    expect(buttonAttributes).toMatch(/className="[^"]*\bw-\d+\b/)
  })

  // Confirmations open a popup instead of growing extra buttons inside the header row.
  it('should confirm role changes and bans in a popup rather than in the header row', () => {
    expect(headerRowSource).not.toContain('changeRole.mutate(')
    expect(headerRowSource).not.toMatch(/['>]\s*Confirm\s*['<]/)
    expect(userDetailRouteSource).toMatch(/<AlertConfirmDialog[\s\S]*?onConfirm=\{\(\) =>\s*changeRole\.mutate\(/)
    expect(userDetailRouteSource).toMatch(
      /<AlertConfirmDialog[\s\S]*?onConfirm=\{\(\) => toggleStatus\.mutate\(false\)\}/,
    )
  })

  // Unban restores access, so it acts at once; only a ban asks first.
  it('should ask before banning but not before unbanning', () => {
    expect(headerRowSource).toMatch(/user\.isActive\s*\?\s*setBanDialogOpen\(true\)\s*:\s*toggleStatus\.mutate\(true\)/)
  })

  // The bulk Ban on the users table confirms in red, so the single-user ban matches it.
  it('should confirm a ban with the destructive colour', () => {
    const banDialog = userDetailRouteSource.slice(
      userDetailRouteSource.indexOf('open={banDialogOpen}'),
      userDetailRouteSource.indexOf('/>', userDetailRouteSource.indexOf('open={banDialogOpen}')),
    )

    expect(banDialog).not.toContain('variant="default"')
  })

  // The role dropdown sits in the same row as the small buttons, which are h-9 (36px) tall.
  it('should give the role dropdown the same height as the header buttons', () => {
    expect(headerRowSource).toMatch(/<SelectTrigger className="[^"]*\bh-9\b/)
  })
})
