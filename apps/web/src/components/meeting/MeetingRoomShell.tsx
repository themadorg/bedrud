import { type ReactNode, useCallback, useEffect, useRef, useState } from 'react'
import { MeetingHeader } from '@/components/meeting/MeetingHeader'
import { MeetingPanels } from '@/components/meeting/MeetingPanels'
import { MeetingUILayoutProvider, participantsDockOffset } from '@/components/meeting/MeetingUILayoutContext'
import { MeetingPresenceCursors } from '@/components/meeting/presence/MeetingPresenceCursors'
import { MeetingViewportPanProvider } from '@/components/meeting/MeetingViewportPan'
import { ParticipantVideoSidebar } from '@/components/meeting/ParticipantVideoSidebar'
import { useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'

interface MeetingRoomShellProps {
  meetId: string
  navigate: () => void
  children: ReactNode
}

export function MeetingRoomShell({ meetId, navigate, children }: MeetingRoomShellProps) {
  const [chatOpen, setChatOpen] = useState(true)
  const [chatStuck, setChatStuck] = useState(false)
  const [videoSidebarOpen, setVideoSidebarOpen] = useState(true)
  const [participantsOpen, setParticipantsOpen] = useState(false)
  const [infoOpen, setInfoOpen] = useState(false)
  const { stage } = useMeetingStage()
  const hadStageRef = useRef(false)
  const chatDocked = chatOpen && chatStuck
  const videoSidebarDocked = Boolean(stage) && videoSidebarOpen

  const toggleInfo = useCallback(() => {
    setInfoOpen((open) => !open)
    if (!chatStuck) setChatOpen(false)
    setParticipantsOpen(false)
  }, [chatStuck])

  const toggleParticipants = useCallback(() => {
    setParticipantsOpen((open) => !open)
    if (!chatStuck) setChatOpen(false)
    setInfoOpen(false)
  }, [chatStuck])

  useEffect(() => {
    if (stage && !hadStageRef.current) setVideoSidebarOpen(true)
    if (!stage) setVideoSidebarOpen(true)
    hadStageRef.current = Boolean(stage)
  }, [stage])

  return (
    <MeetingUILayoutProvider chatDocked={chatDocked} participantsDocked={videoSidebarDocked}>
      <MeetingViewportPanProvider>
        <MeetingPresenceCursors />
        {children}
      {stage && videoSidebarOpen && (
        <ParticipantVideoSidebar
          stackOffset={participantsDockOffset(chatDocked)}
          onClose={() => setVideoSidebarOpen(false)}
        />
      )}
      <MeetingHeader meetId={meetId} infoOpen={infoOpen} onToggleInfo={toggleInfo} />
        <MeetingPanels
          navigate={navigate}
          chatOpen={chatOpen}
          setChatOpen={setChatOpen}
          chatStuck={chatStuck}
          setChatStuck={setChatStuck}
          videoSidebarOpen={videoSidebarOpen}
          onToggleVideoSidebar={() => setVideoSidebarOpen((open) => !open)}
          infoOpen={infoOpen}
          onCloseInfo={() => setInfoOpen(false)}
          participantsOpen={participantsOpen}
          onToggleParticipants={toggleParticipants}
          onCloseParticipants={() => setParticipantsOpen(false)}
        />
      </MeetingViewportPanProvider>
    </MeetingUILayoutProvider>
  )
}
