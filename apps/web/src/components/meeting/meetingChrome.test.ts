// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const meetingPanelsSource = readFileSync(new URL('./MeetingPanels.tsx', import.meta.url), 'utf8')

/** The top-right cluster on phones, which carries the participants and chat toggles. */
const MOBILE_CLUSTER_CLASS = 'absolute z-[25] flex h-9 items-center gap-2 lg:hidden'

describe('the phone meeting chrome', () => {
  it('should keep the top-right cluster visible, since no surface covers it any more', () => {
    // Chat and the participants list are both sheets now. Both stop short of the header band, so the
    // cluster stays put and the participants icon keeps showing its active state.
    const clusterLine = meetingPanelsSource.split('\n').find((line) => line.includes(MOBILE_CLUSTER_CLASS))
    expect(clusterLine).toBeDefined()
    expect(clusterLine).not.toContain('mobileChromeHidden')
  })

  it('should still hide the controls bar beneath the sheet', () => {
    expect(meetingPanelsSource).toContain('const mobileControlsHidden = chatOpen || participantsOpen')
    expect(meetingPanelsSource).toContain('hideOnMobile={mobileControlsHidden}')
  })

  it('should no longer treat chat and participants as one overlay', () => {
    expect(meetingPanelsSource).not.toContain('mobileOverlayOpen')
  })
})
