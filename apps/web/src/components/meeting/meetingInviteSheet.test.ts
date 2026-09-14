// @vitest-environment node
//
// This file reads component sources from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const sheetSource = readFileSync(new URL('./MeetingInviteSheet.tsx', import.meta.url), 'utf8')
const gridSource = readFileSync(new URL('./MeetingInviteGrid.tsx', import.meta.url), 'utf8')
const targetsSource = readFileSync(new URL('./MeetingInviteTargets.tsx', import.meta.url), 'utf8')
const listSource = readFileSync(new URL('./ParticipantsList.tsx', import.meta.url), 'utf8')
const panelsSource = readFileSync(new URL('./MeetingPanels.tsx', import.meta.url), 'utf8')
const chatSource = readFileSync(new URL('./ChatPanel.tsx', import.meta.url), 'utf8')

describe('MeetingInviteSheet', () => {
  it('should be the app sheet rather than a second one', () => {
    expect(sheetSource).toContain('<BedrudSheet')
  })

  it('should leave the sheet chrome to the sheet', () => {
    expect(sheetSource).not.toContain('snapPoints=')
    expect(sheetSource).not.toMatch(/\brounded-t-/)
    expect(sheetSource).not.toContain('BedrudSheetHandle')
    expect(sheetSource).not.toContain('--meet-sheet-max-height')
  })

  it('should let its body scroll, since the grid and the QR can outgrow the half height', () => {
    expect(sheetSource).toContain('overflow-y-auto')
  })

  it('should grow to full when the QR is opened, so the targets under it stay reachable', () => {
    expect(sheetSource).toContain('SHEET_SNAP_POINTS[SHEET_SNAP_POINTS.length - 1]')
    expect(sheetSource).toContain('onExpandSheet')
  })

  it('should leave the sheet at its current height when the QR is closed again', () => {
    // The expand belongs to opening the QR. Closing it must not also drag the sheet to full, which
    // is why the next visibility is computed before the state is set rather than inside the updater.
    expect(targetsSource).toContain('if (nextVisible) onExpandSheet()')
  })

  it('should not introduce a breakpoint other than the shared one', () => {
    expect(sheetSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
    expect(gridSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
    expect(targetsSource).not.toMatch(/(^|[\s"'`])(max-)?(sm|md):/)
  })
})

describe('the invite sizes', () => {
  it('should come from the meeting tokens rather than from the call sites', () => {
    expect(gridSource).toContain('var(--meet-invite-grid-max-height)')
    expect(gridSource).toContain('var(--meet-invite-avatar-size)')
    expect(targetsSource).toContain('var(--meet-invite-target-size)')
    expect(targetsSource).toContain('var(--meet-invite-qr-size)')
  })
})

describe('ParticipantsList after the sheet', () => {
  it('should no longer trap focus, because it is no longer a modal surface', () => {
    expect(listSource).not.toContain('useFocusTrap')
    expect(listSource).not.toContain('aria-modal')
  })

  it('should no longer carry the full-screen phone sizing', () => {
    // The offsets and the full visual-viewport height are the phone overlay's alone. `--app-width`
    // is not asserted on: the desktop sidebar still clamps its own width with it.
    expect(listSource).not.toContain('--app-offset-left')
    expect(listSource).not.toContain('--app-offset-top')
    expect(listSource).not.toContain('h-[var(--app-height,100svh)]')
  })

  it('should keep the body portal, which is what puts the desktop sidebar above the stage', () => {
    // The portal was never about the phone overlay: the sidebar is `lg:fixed` at z-40 and has to sit
    // above the body-portalled stage WebXDC at z-15.
    expect(listSource).toContain('createPortal')
  })

  it('should drop the marker attribute nothing selects on', () => {
    expect(listSource).not.toContain('data-participants-overlay')
  })
})

describe('the phone meeting after the sheet', () => {
  it('should no longer hide the top-right cluster for a full-screen list', () => {
    expect(panelsSource).not.toContain('mobileChromeHidden')
  })

  it('should still hide the controls bar beneath an open sheet', () => {
    expect(panelsSource).toContain('const mobileControlsHidden = chatOpen || participantsOpen')
  })

  it('should stop telling chat to yield its focus trap to the list', () => {
    expect(chatSource).not.toContain('participantsOpen')
    expect(panelsSource).not.toContain('participantsOpen={participantsOpen}')
  })
})
