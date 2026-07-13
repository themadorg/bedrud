import { type ReactNode, useCallback, useEffect, useRef, useState } from 'react'
import { MeetingHeader } from '@/components/meeting/MeetingHeader'
import { MeetingPanels } from '@/components/meeting/MeetingPanels'
import {
  isWebxdcExpandSource,
  MEETING_CLOSE_ELEVATED_CHROME,
  MEETING_OPEN_CHAT,
  MEETING_OPEN_ROOM_INFO,
  MEETING_OPEN_SETTINGS,
  publishMeetingChromeState,
  requestCloseMeetingSettings,
} from '@/components/meeting/meetingChromeEvents'
import { MeetingUILayoutProvider, participantsDockOffset } from '@/components/meeting/MeetingUILayoutContext'
import { MeetingViewportPanProvider } from '@/components/meeting/MeetingViewportPan'
import { ParticipantVideoSidebar } from '@/components/meeting/ParticipantVideoSidebar'
import { MeetingPresenceCursors } from '@/components/meeting/presence/MeetingPresenceCursors'
import { useMeetingStage } from '@/components/meeting/stage/MeetingStageContext'

interface MeetingRoomShellProps {
  meetId: string
  navigate: () => void
  children: ReactNode
}

export function MeetingRoomShell({ meetId, navigate, children }: MeetingRoomShellProps) {
  // Desktop: open chat sidebar by default. Mobile: closed — chat is a full-screen modal when opened.
  const [chatOpen, setChatOpen] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia('(min-width: 640px)').matches : true,
  )
  const [chatStuck, setChatStuck] = useState(false)
  /** Left when opened from expanded WebXDC; right otherwise. */
  const [chatSide, setChatSide] = useState<'left' | 'right'>('right')
  /** Which elevated left-dock panel is active (`null` = none). */
  const [elevatedPanel, setElevatedPanel] = useState<'chat' | 'info' | null>(null)
  /** Bumps when elevating so ChatPanel remounts into a body portal (already-open case). */
  const [chatSurfaceKey, setChatSurfaceKey] = useState(0)
  const [videoSidebarOpen, setVideoSidebarOpen] = useState(true)
  const [participantsOpen, setParticipantsOpen] = useState(false)
  const [infoOpen, setInfoOpen] = useState(false)
  const { stage } = useMeetingStage()
  const hadStageRef = useRef(false)
  const chatOpenRef = useRef(chatOpen)
  const infoOpenRef = useRef(infoOpen)
  const elevatedPanelRef = useRef(elevatedPanel)
  chatOpenRef.current = chatOpen
  infoOpenRef.current = infoOpen
  elevatedPanelRef.current = elevatedPanel

  const chatDocked = chatOpen && chatStuck
  const videoSidebarDocked = Boolean(stage) && videoSidebarOpen
  const chatElevated = elevatedPanel === 'chat'
  const infoElevated = elevatedPanel === 'info'

  const clearElevatedChrome = useCallback(() => {
    setElevatedPanel(null)
    setChatSide('right')
  }, [])

  const toggleInfo = useCallback(() => {
    setInfoOpen((open) => !open)
    if (!chatStuck) setChatOpen(false)
    setParticipantsOpen(false)
    clearElevatedChrome()
    requestCloseMeetingSettings()
  }, [chatStuck, clearElevatedChrome])

  const toggleParticipants = useCallback(() => {
    setParticipantsOpen((open) => !open)
    if (!chatStuck) setChatOpen(false)
    setInfoOpen(false)
    clearElevatedChrome()
    requestCloseMeetingSettings()
  }, [chatStuck, clearElevatedChrome])

  const handleSetChatOpen = useCallback((open: boolean | ((prev: boolean) => boolean)) => {
    setChatOpen((prev) => {
      const next = typeof open === 'function' ? open(prev) : open
      if (!next) {
        setElevatedPanel((p) => {
          if (p === 'chat') {
            publishMeetingChromeState(null)
            return null
          }
          return p
        })
        setChatSide('right')
      }
      return next
    })
  }, [])

  useEffect(() => {
    if (stage && !hadStageRef.current) setVideoSidebarOpen(true)
    if (!stage) setVideoSidebarOpen(true)
    hadStageRef.current = Boolean(stage)
  }, [stage])

  // Keep left-rail icons in sync for chat/info (settings publishes itself).
  useEffect(() => {
    if (elevatedPanel === 'chat' && chatOpen) publishMeetingChromeState('chat')
    else if (elevatedPanel === 'info' && infoOpen) publishMeetingChromeState('info')
  }, [elevatedPanel, chatOpen, infoOpen])

  // WebXDC left rail (and other chrome) can request panels without prop drilling.
  useEffect(() => {
    const onChat = (e: Event) => {
      const fromWx = isWebxdcExpandSource((e as CustomEvent).detail)
      setParticipantsOpen(false)
      setInfoOpen(false)

      if (fromWx) {
        // Second click on left-rail chat while already elevated → close.
        if (chatOpenRef.current && elevatedPanelRef.current === 'chat') {
          setChatOpen(false)
          setChatStuck(false)
          setElevatedPanel(null)
          setChatSide('right')
          publishMeetingChromeState(null)
          return
        }
        requestCloseMeetingSettings()
        setElevatedPanel('chat')
        setChatSide('left')
        // Desktop chat often starts open under the shell; bump key so the elevated
        // body-portaled panel always mounts fresh above WebXDC (z-200).
        setChatSurfaceKey((k) => k + 1)
        setChatOpen(true)
        return
      }

      setElevatedPanel(null)
      setChatSide('right')
      setChatOpen(true)
    }

    const onInfo = (e: Event) => {
      const fromWx = isWebxdcExpandSource((e as CustomEvent).detail)
      setParticipantsOpen(false)

      if (fromWx) {
        // Toggle close when info already elevated.
        if (infoOpenRef.current && elevatedPanelRef.current === 'info') {
          setInfoOpen(false)
          setElevatedPanel(null)
          publishMeetingChromeState(null)
          return
        }
        requestCloseMeetingSettings()
        setElevatedPanel('info')
        setInfoOpen(true)
        if (!chatStuck) {
          setChatOpen(false)
          setChatSide('right')
        }
        return
      }

      setElevatedPanel(null)
      setInfoOpen(true)
      if (!chatStuck) {
        setChatOpen(false)
        setChatSide('right')
      }
    }

    // Settings opened from rail: close elevated chat/info so only one left dock shows.
    const onSettings = (e: Event) => {
      if (!isWebxdcExpandSource((e as CustomEvent).detail)) return
      setParticipantsOpen(false)
      setInfoOpen(false)
      if (!chatStuck) {
        setChatOpen(false)
        setChatSide('right')
      }
      setElevatedPanel(null)
      // Glow is published by ControlsBar when settings actually opens (toggle close handled there).
    }

    // Expand collapsed: drop elevated chat/info docks (settings handled in ControlsBar).
    const onCloseElevated = () => {
      if (elevatedPanelRef.current === 'chat') {
        setChatOpen(false)
        setChatStuck(false)
      }
      if (elevatedPanelRef.current === 'info') {
        setInfoOpen(false)
      }
      setElevatedPanel(null)
      setChatSide('right')
      publishMeetingChromeState(null)
    }

    window.addEventListener(MEETING_OPEN_CHAT, onChat)
    window.addEventListener(MEETING_OPEN_ROOM_INFO, onInfo)
    window.addEventListener(MEETING_OPEN_SETTINGS, onSettings)
    window.addEventListener(MEETING_CLOSE_ELEVATED_CHROME, onCloseElevated)
    return () => {
      window.removeEventListener(MEETING_OPEN_CHAT, onChat)
      window.removeEventListener(MEETING_OPEN_ROOM_INFO, onInfo)
      window.removeEventListener(MEETING_OPEN_SETTINGS, onSettings)
      window.removeEventListener(MEETING_CLOSE_ELEVATED_CHROME, onCloseElevated)
    }
  }, [chatStuck])

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
          setChatOpen={handleSetChatOpen}
          chatSurfaceKey={chatSurfaceKey}
          onChatOpenFromToggle={() => {
            setChatSide('right')
            setElevatedPanel(null)
          }}
          chatStuck={chatStuck}
          setChatStuck={setChatStuck}
          chatSide={chatSide}
          chatElevated={chatElevated}
          videoSidebarOpen={videoSidebarOpen}
          onToggleVideoSidebar={() => setVideoSidebarOpen((open) => !open)}
          infoOpen={infoOpen}
          infoElevated={infoElevated}
          onCloseInfo={() => {
            setInfoOpen(false)
            setElevatedPanel((p) => {
              if (p === 'info') {
                publishMeetingChromeState(null)
                return null
              }
              return p
            })
          }}
          onToggleInfo={toggleInfo}
          participantsOpen={participantsOpen}
          onToggleParticipants={toggleParticipants}
          onCloseParticipants={() => setParticipantsOpen(false)}
        />
      </MeetingViewportPanProvider>
    </MeetingUILayoutProvider>
  )
}
