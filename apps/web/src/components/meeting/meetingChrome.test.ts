// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const meetingPanelsSource = readFileSync(new URL('./MeetingPanels.tsx', import.meta.url), 'utf8')

/** The top-right cluster on phones, which carries the participants and chat toggles. */
const MOBILE_CLUSTER_CLASS = "'absolute z-[25] flex h-9 items-center gap-2 lg:hidden'"

describe('the phone meeting chrome', () => {
  it('should keep the top-right cluster visible while chat is open', () => {
    // Chat is a sheet that stops short of the header band, so the cluster it was hidden for is no
    // longer covered — and the chat toggle has to keep showing its active state.
    const clusterLine = meetingPanelsSource.split('\n').find((line) => line.includes(MOBILE_CLUSTER_CLASS))
    expect(clusterLine).toBeDefined()
    expect(clusterLine).toContain('mobileChromeHidden')
  })

  it('should hide the top-right cluster under the full-screen participants list', () => {
    expect(meetingPanelsSource).toContain('const mobileChromeHidden = participantsOpen')
  })

  it('should still hide the controls bar beneath the sheet', () => {
    expect(meetingPanelsSource).toContain('const mobileControlsHidden = chatOpen || participantsOpen')
    expect(meetingPanelsSource).toContain('hideOnMobile={mobileControlsHidden}')
  })

  it('should no longer treat chat and participants as one overlay', () => {
    expect(meetingPanelsSource).not.toContain('mobileOverlayOpen')
  })
})
