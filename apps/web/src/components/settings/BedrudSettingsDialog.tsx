import { useRoomContext } from '@livekit/components-react'
import { Camera, ChevronLeft, ChevronRight, FlaskConical, Lock, Mic, Palette, User } from 'lucide-react'
import { useEffect, useState } from 'react'
import { AppearanceSettingsPanel } from '#/components/settings/AppearanceSettingsPanel'
import { AudioSettingsPanel } from '#/components/settings/AudioSettingsPanel'
import { ExperimentalSettingsPanel } from '#/components/settings/ExperimentalSettingsPanel'
import { ProfileSettingsPanel } from '#/components/settings/ProfileSettingsPanel'
import { SecuritySettingsPanel } from '#/components/settings/SecuritySettingsPanel'
import { VideoSettingsPanel } from '#/components/settings/VideoSettingsPanel'
import { cn } from '#/lib/utils'
import { MeetingElevatedLeftDock } from '@/components/meeting/MeetingElevatedLeftDock'
import {
  MeetingElevatedPanelHeader,
  MeetingElevatedPanelSectionSubheader,
} from '@/components/meeting/MeetingElevatedPanelChrome'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { meetingPanelScopeClass, settingsDialogScrollClass, settingsSidebarTabClass } from './settingsPanelTone'

const TABS = [
  { id: 'profile', label: 'Profile', icon: User, description: 'Name and avatar' },
  { id: 'appearance', label: 'Appearance', icon: Palette, description: 'Theme and interface' },
  { id: 'audio', label: 'Audio', icon: Mic, description: 'Mic, noise, push-to-talk' },
  { id: 'video', label: 'Video', icon: Camera, description: 'Camera and quality' },
  { id: 'security', label: 'Security', icon: Lock, description: 'Password and sessions' },
  { id: 'experimental', label: 'Experimental', icon: FlaskConical, description: 'Whiteboard, YouTube, WebXDC, …' },
] as const

type TabId = (typeof TABS)[number]['id']

function MeetingVideoSettingsPanel() {
  const room = useRoomContext()
  return (
    <VideoSettingsPanel
      tone="meeting"
      onCameraDeviceChange={(deviceId) => {
        void room.switchActiveDevice('videoinput', deviceId).catch(() => {})
      }}
    />
  )
}

function SettingsPanelBody({ tab, inMeeting }: { tab: TabId; inMeeting: boolean }) {
  switch (tab) {
    case 'profile':
      return <ProfileSettingsPanel tone="meeting" />
    case 'appearance':
      return <AppearanceSettingsPanel tone="meeting" />
    case 'security':
      return <SecuritySettingsPanel />
    case 'audio':
      return <AudioSettingsPanel tone="meeting" />
    case 'video':
      return inMeeting ? <MeetingVideoSettingsPanel /> : <VideoSettingsPanel tone="meeting" />
    case 'experimental':
      return <ExperimentalSettingsPanel tone="meeting" />
    default:
      return null
  }
}

function SettingsListNav({
  page,
  navDir,
  tabs,
  inMeeting,
  onOpenSubPage,
  onBack,
  onClose,
}: {
  page: TabId | null
  navDir: 'forward' | 'back'
  tabs: readonly (typeof TABS)[number][]
  inMeeting: boolean
  onOpenSubPage: (id: TabId) => void
  onBack: () => void
  onClose: () => void
}) {
  const activeTabMeta = page ? tabs.find((t) => t.id === page) : null
  const pageAnimClass =
    navDir === 'forward'
      ? 'animate-in fade-in-0 slide-in-from-right duration-200 ease-out'
      : 'animate-in fade-in-0 slide-in-from-left duration-200 ease-out'

  return (
    <>
      <MeetingElevatedPanelHeader
        title="Settings"
        onClose={onClose}
        closeLabel="Close settings"
        leading={
          page ? (
            <button
              type="button"
              onClick={onBack}
              className="flex h-11 min-w-0 flex-1 items-center gap-0.5 border-none bg-transparent px-1 text-[var(--meet-accent)]"
              aria-label="Back to settings"
            >
              <ChevronLeft size={22} className="shrink-0" />
              <span className="truncate text-[15px]">Settings</span>
            </button>
          ) : undefined
        }
      />

      {page && activeTabMeta && (
        <MeetingElevatedPanelSectionSubheader title={activeTabMeta.label} className={pageAnimClass} />
      )}

      <div
        className={cn(
          'relative min-h-0 flex-1 overflow-hidden pb-[max(0.75rem,env(safe-area-inset-bottom,0px))]',
          meetingPanelScopeClass,
        )}
      >
        <div
          key={page ?? 'root'}
          className={cn('meet-scroll absolute inset-0 overflow-y-auto', settingsDialogScrollClass, pageAnimClass)}
        >
          {page === null ? (
            <nav className="p-3" aria-label="Settings categories">
              <ul className="m-0 list-none overflow-hidden rounded-xl border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] p-0">
                {tabs.map(({ id, label, icon: Icon, description }, index) => (
                  <li key={id} className={cn(index > 0 && 'border-t border-[var(--meet-border)]')}>
                    <button
                      type="button"
                      onClick={() => onOpenSubPage(id)}
                      className="flex w-full items-center gap-3 border-none bg-transparent px-3.5 py-3 text-start transition-colors active:bg-[var(--meet-control)] hover:bg-[var(--meet-control)]"
                    >
                      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]">
                        <Icon size={16} />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block text-[15px] font-medium text-[var(--meet-fg-strong)]">{label}</span>
                        <span className="block truncate text-[12px] text-[var(--meet-fg-muted)]">{description}</span>
                      </span>
                      <ChevronRight size={18} className="shrink-0 text-[var(--meet-fg-subtle)]" />
                    </button>
                  </li>
                ))}
              </ul>
            </nav>
          ) : (
            <div className="p-4">
              <SettingsPanelBody tab={page} inMeeting={inMeeting} />
            </div>
          )}
        </div>
      </div>
    </>
  )
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  /**
   * When true (WebXDC expand rail): dock as a left overlay like ChatPanel
   * instead of a centered dialog — does not collapse the mini-app.
   */
  elevated?: boolean
  /** LiveKit room device switching. False outside meetings. */
  inMeeting?: boolean
  includeSecurity?: boolean
  /** Desktop sidebar tab when dialog opens. */
  initialTab?: TabId
  /** Mobile drill-down page when dialog opens (`null` = category list). */
  initialMobilePage?: TabId | null
  /** Leave bottom clearance for dashboard mobile nav. */
  dockAboveMobileNav?: boolean
}

export function BedrudSettingsDialog({
  open,
  onOpenChange,
  elevated = false,
  inMeeting = true,
  includeSecurity = true,
  initialTab,
  initialMobilePage = null,
  dockAboveMobileNav = false,
}: Props) {
  const tabs = includeSecurity ? TABS : TABS.filter((t) => t.id !== 'security')
  const [tab, setTab] = useState<TabId>(initialTab ?? (inMeeting ? 'audio' : 'profile'))
  const [mobilePage, setMobilePage] = useState<TabId | null>(null)
  const [navDir, setNavDir] = useState<'forward' | 'back'>('forward')

  useEffect(() => {
    if (!open) {
      setMobilePage(null)
      setNavDir('forward')
      return
    }
    setTab(initialTab ?? (inMeeting ? 'audio' : 'profile'))
    setMobilePage(initialMobilePage)
    setNavDir('forward')
  }, [open, initialTab, initialMobilePage, inMeeting])

  const openSubPage = (id: TabId) => {
    setNavDir('forward')
    setMobilePage(id)
  }

  const goBackToRoot = () => {
    setNavDir('back')
    setMobilePage(null)
  }

  const close = () => onOpenChange(false)

  const listBody = (
    <SettingsListNav
      page={mobilePage}
      navDir={navDir}
      tabs={tabs}
      inMeeting={inMeeting}
      onOpenSubPage={openSubPage}
      onBack={goBackToRoot}
      onClose={close}
    />
  )

  if (elevated) {
    if (!open) return null
    return (
      <MeetingElevatedLeftDock label="Settings" marker="settings">
        {listBody}
      </MeetingElevatedLeftDock>
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        overlayClassName={
          dockAboveMobileNav
            ? 'max-sm:h-[calc(var(--app-height,100svh)-4.5rem-env(safe-area-inset-bottom,0px))]'
            : undefined
        }
        className={cn(
          'meet-dialog flex flex-col gap-0 overflow-hidden p-0 shadow-2xl',
          'sm:h-[min(90vh,720px)] sm:w-[min(760px,calc(var(--app-width,100svw)-2rem))] sm:max-w-[min(760px,calc(var(--app-width,100svw)-2rem))]',
          dockAboveMobileNav
            ? 'max-sm:fixed max-sm:left-[var(--app-offset-left,0px)] max-sm:top-[var(--app-offset-top,0px)] max-sm:h-[calc(var(--app-height,100svh)-4.5rem-env(safe-area-inset-bottom,0px))] max-sm:max-h-[calc(var(--app-height,100svh)-4.5rem-env(safe-area-inset-bottom,0px))] max-sm:w-[var(--app-width,100svw)] max-sm:max-w-[var(--app-width,100svw)] max-sm:translate-x-0 max-sm:translate-y-0 max-sm:rounded-none max-sm:border-0 max-sm:border-b max-sm:border-[var(--meet-border)]'
            : 'max-sm:fixed max-sm:left-[var(--app-offset-left,0px)] max-sm:top-[var(--app-offset-top,0px)] max-sm:h-[var(--app-height,100svh)] max-sm:max-h-[var(--app-height,100svh)] max-sm:w-[var(--app-width,100svw)] max-sm:max-w-[var(--app-width,100svw)] max-sm:translate-x-0 max-sm:translate-y-0 max-sm:rounded-none max-sm:border-0',
          'max-sm:[&>button.absolute]:hidden',
        )}
      >
        <div className="flex min-h-0 flex-1 flex-col sm:hidden">{listBody}</div>

        <div className="hidden min-h-0 flex-1 flex-col sm:flex">
          <DialogHeader className="shrink-0 border-b border-[var(--meet-border)] px-4 py-3">
            <DialogTitle className="text-[15px] font-semibold text-[var(--meet-fg)]">Settings</DialogTitle>
          </DialogHeader>

          <Tabs
            value={tab}
            onValueChange={(v) => setTab(v as TabId)}
            className="flex min-h-0 flex-1 flex-row overflow-hidden"
            orientation="vertical"
          >
            <div className="flex w-[148px] shrink-0 flex-col border-r border-[var(--meet-border)] bg-[var(--meet-surface-muted)] px-2 py-3">
              <TabsList className="flex h-auto w-full flex-col items-stretch gap-0.5 bg-transparent p-0 text-[var(--meet-fg-muted)]">
                {tabs.map(({ id, label, icon: Icon }) => (
                  <TabsTrigger
                    key={id}
                    value={id}
                    className={cn(
                      'h-auto w-full justify-start gap-2 rounded-md px-3 py-2 text-xs shadow-none ring-offset-[var(--meet-bg-panel)]',
                      settingsSidebarTabClass,
                    )}
                  >
                    <Icon className="h-3.5 w-3.5 shrink-0" />
                    {label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </div>

            <div
              className={cn(
                'min-h-0 min-w-0 flex-1 overflow-y-auto p-4',
                settingsDialogScrollClass,
                meetingPanelScopeClass,
              )}
            >
              <TabsContent value="profile" className="mt-0 outline-none">
                <ProfileSettingsPanel tone="meeting" />
              </TabsContent>
              <TabsContent value="appearance" className="mt-0 outline-none">
                <AppearanceSettingsPanel tone="meeting" />
              </TabsContent>
              {includeSecurity ? (
                <TabsContent value="security" className="mt-0 outline-none">
                  <SecuritySettingsPanel />
                </TabsContent>
              ) : null}
              <TabsContent value="audio" className="mt-0 outline-none">
                <AudioSettingsPanel tone="meeting" />
              </TabsContent>
              <TabsContent value="video" className="mt-0 outline-none">
                {inMeeting ? <MeetingVideoSettingsPanel /> : <VideoSettingsPanel tone="meeting" />}
              </TabsContent>
              <TabsContent value="experimental" className="mt-0 outline-none">
                <ExperimentalSettingsPanel tone="meeting" />
              </TabsContent>
            </div>
          </Tabs>
        </div>
      </DialogContent>
    </Dialog>
  )
}
