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

function SettingsPanelBody({
  tab,
  inMeeting,
  variant,
}: {
  tab: TabId
  inMeeting: boolean
  variant: 'meeting' | 'app'
}) {
  const tone = variant === 'meeting' ? 'meeting' : 'default'
  switch (tab) {
    case 'profile':
      return <ProfileSettingsPanel tone={tone} />
    case 'appearance':
      return <AppearanceSettingsPanel tone={tone} />
    case 'security':
      return <SecuritySettingsPanel />
    case 'audio':
      return <AudioSettingsPanel tone={tone} />
    case 'video':
      return inMeeting ? <MeetingVideoSettingsPanel /> : <VideoSettingsPanel tone={tone} />
    case 'experimental':
      return <ExperimentalSettingsPanel tone={tone} />
    default:
      return null
  }
}

function SettingsListNav({
  page,
  navDir,
  tabs,
  inMeeting,
  showHeader,
  variant,
  onOpenSubPage,
  onBack,
  onClose,
}: {
  page: TabId | null
  navDir: 'forward' | 'back'
  tabs: readonly (typeof TABS)[number][]
  inMeeting: boolean
  showHeader: boolean
  variant: 'meeting' | 'app'
  onOpenSubPage: (id: TabId) => void
  onBack: () => void
  onClose: () => void
}) {
  const activeTabMeta = page ? tabs.find((t) => t.id === page) : null
  const pageAnimClass =
    navDir === 'forward'
      ? 'animate-in fade-in-0 slide-in-from-right duration-200 ease-out'
      : 'animate-in fade-in-0 slide-in-from-left duration-200 ease-out'
  const isApp = variant === 'app'
  const listSurface = isApp
    ? 'border border-border bg-muted/40'
    : 'border border-[var(--meet-border)] bg-[var(--meet-surface-muted)]'
  const listItemBorder = isApp ? 'border-border' : 'border-[var(--meet-border)]'
  const listHover = isApp
    ? 'active:bg-muted hover:bg-muted'
    : 'active:bg-[var(--meet-control)] hover:bg-[var(--meet-control)]'
  const iconChip = isApp
    ? 'bg-primary/10 text-primary'
    : 'bg-[var(--meet-btn-muted-bg)] text-[var(--meet-btn-muted-fg)]'
  const titleClass = isApp ? 'text-foreground' : 'text-[var(--meet-fg-strong)]'
  const descClass = isApp ? 'text-muted-foreground' : 'text-[var(--meet-fg-muted)]'
  const chevronClass = isApp ? 'text-muted-foreground' : 'text-[var(--meet-fg-subtle)]'
  const backClass = isApp ? 'text-primary' : 'text-[var(--meet-accent)]'
  const scopeClass = isApp ? undefined : meetingPanelScopeClass

  return (
    <>
      {showHeader ? (
        <MeetingElevatedPanelHeader
          title="Settings"
          onClose={onClose}
          closeLabel="Close settings"
          leading={
            page ? (
              <button
                type="button"
                onClick={onBack}
                className={cn(
                  'flex h-11 min-w-0 flex-1 items-center gap-0.5 border-none bg-transparent px-1',
                  backClass,
                )}
                aria-label="Back to settings"
              >
                <ChevronLeft size={22} className="shrink-0" />
                <span className="truncate text-[15px]">Settings</span>
              </button>
            ) : undefined
          }
        />
      ) : page ? (
        <div
          className={cn(
            'flex shrink-0 items-center gap-1 border-b',
            isApp ? 'border-border' : 'border-[var(--meet-border)]',
          )}
        >
          <button
            type="button"
            onClick={onBack}
            className={cn('flex h-11 min-w-0 flex-1 items-center gap-0.5 border-none bg-transparent px-3', backClass)}
            aria-label="Back to settings"
          >
            <ChevronLeft size={22} className="shrink-0" />
            <span className="truncate text-[15px]">Settings</span>
          </button>
        </div>
      ) : null}

      {page && activeTabMeta && showHeader && (
        <MeetingElevatedPanelSectionSubheader title={activeTabMeta.label} className={pageAnimClass} />
      )}
      {page && activeTabMeta && !showHeader && (
        <div
          className={cn(
            'shrink-0 border-b px-4 py-2',
            isApp ? 'border-border' : 'border-[var(--meet-border)]',
            pageAnimClass,
          )}
        >
          <h2 className={cn('text-[15px] font-semibold', titleClass)}>{activeTabMeta.label}</h2>
        </div>
      )}

      <div
        className={cn(
          'relative min-h-0 flex-1 overflow-hidden',
          showHeader && 'pb-[max(0.75rem,env(safe-area-inset-bottom,0px))]',
          scopeClass,
        )}
      >
        <div
          key={page ?? 'root'}
          className={cn('meet-scroll absolute inset-0 overflow-y-auto', settingsDialogScrollClass, pageAnimClass)}
        >
          {page === null ? (
            <nav className="p-3" aria-label="Settings categories">
              <ul className={cn('m-0 list-none overflow-hidden p-0', listSurface)}>
                {tabs.map(({ id, label, icon: Icon, description }, index) => (
                  <li key={id} className={cn(index > 0 && 'border-t', listItemBorder)}>
                    <button
                      type="button"
                      onClick={() => onOpenSubPage(id)}
                      className={cn(
                        'flex w-full items-center gap-3 border-none bg-transparent px-3.5 py-3 text-start transition-colors',
                        listHover,
                      )}
                    >
                      <span className={cn('flex h-8 w-8 shrink-0 items-center justify-center', iconChip)}>
                        <Icon size={16} />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className={cn('block text-[15px] font-medium', titleClass)}>{label}</span>
                        <span className={cn('block truncate text-[12px]', descClass)}>{description}</span>
                      </span>
                      <ChevronRight size={18} className={cn('shrink-0', chevronClass)} />
                    </button>
                  </li>
                ))}
              </ul>
            </nav>
          ) : (
            <div className="p-4">
              <SettingsPanelBody tab={page} inMeeting={inMeeting} variant={variant} />
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
  /** Mobile top chrome (title + close). Default true. */
  showMobileHeader?: boolean
  /** Visual surface: meeting (meet tokens) or app (theme background). */
  variant?: 'meeting' | 'app'
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
  showMobileHeader = true,
  variant = 'meeting',
}: Props) {
  const tabs = includeSecurity ? TABS : TABS.filter((t) => t.id !== 'security')
  const [tab, setTab] = useState<TabId>(initialTab ?? (inMeeting ? 'audio' : 'profile'))
  const [mobilePage, setMobilePage] = useState<TabId | null>(null)
  const [navDir, setNavDir] = useState<'forward' | 'back'>('forward')
  const panelTone = variant === 'meeting' ? 'meeting' : 'default'

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
      showHeader={showMobileHeader}
      variant={variant}
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
        onPointerDownOutside={dockAboveMobileNav ? (e) => e.preventDefault() : undefined}
        onInteractOutside={dockAboveMobileNav ? (e) => e.preventDefault() : undefined}
        onFocusOutside={dockAboveMobileNav ? (e) => e.preventDefault() : undefined}
        className={cn(
          'flex flex-col gap-0 overflow-hidden p-0 shadow-2xl',
          variant === 'meeting' ? 'meet-dialog' : 'bg-background',
          'sm:h-[min(90vh,720px)] sm:w-[min(760px,calc(var(--app-width,100svw)-2rem))] sm:max-w-[min(760px,calc(var(--app-width,100svw)-2rem))]',
          dockAboveMobileNav
            ? cn(
                'max-sm:fixed max-sm:left-[var(--app-offset-left,0px)] max-sm:top-[var(--app-offset-top,0px)] max-sm:h-[calc(var(--app-height,100svh)-4.5rem-env(safe-area-inset-bottom,0px))] max-sm:max-h-[calc(var(--app-height,100svh)-4.5rem-env(safe-area-inset-bottom,0px))] max-sm:w-[var(--app-width,100svw)] max-sm:max-w-[var(--app-width,100svw)] max-sm:translate-x-0 max-sm:translate-y-0 max-sm:rounded-none max-sm:border-0 max-sm:border-b',
                variant === 'meeting' ? 'max-sm:border-[var(--meet-border)]' : 'max-sm:border-border',
              )
            : 'max-sm:fixed max-sm:left-[var(--app-offset-left,0px)] max-sm:top-[var(--app-offset-top,0px)] max-sm:h-[var(--app-height,100svh)] max-sm:max-h-[var(--app-height,100svh)] max-sm:w-[var(--app-width,100svw)] max-sm:max-w-[var(--app-width,100svw)] max-sm:translate-x-0 max-sm:translate-y-0 max-sm:rounded-none max-sm:border-0',
          'max-sm:[&>button.absolute]:hidden',
        )}
      >
        <div className="flex min-h-0 flex-1 flex-col sm:hidden">{listBody}</div>

        <div className="hidden min-h-0 flex-1 flex-col sm:flex">
          <DialogHeader
            className={cn(
              'shrink-0 border-b px-4 py-3',
              variant === 'meeting' ? 'border-[var(--meet-border)]' : 'border-border',
            )}
          >
            <DialogTitle
              className={cn(
                'text-[15px] font-semibold',
                variant === 'meeting' ? 'text-[var(--meet-fg)]' : 'text-foreground',
              )}
            >
              Settings
            </DialogTitle>
          </DialogHeader>

          <Tabs
            value={tab}
            onValueChange={(v) => setTab(v as TabId)}
            className="flex min-h-0 flex-1 flex-row overflow-hidden"
            orientation="vertical"
          >
            <div
              className={cn(
                'flex w-[148px] shrink-0 flex-col border-r px-2 py-3',
                variant === 'meeting'
                  ? 'border-[var(--meet-border)] bg-[var(--meet-surface-muted)]'
                  : 'border-border bg-muted/40',
              )}
            >
              <TabsList
                className={cn(
                  'flex h-auto w-full flex-col items-stretch gap-0.5 bg-transparent p-0',
                  variant === 'meeting' ? 'text-[var(--meet-fg-muted)]' : 'text-muted-foreground',
                )}
              >
                {tabs.map(({ id, label, icon: Icon }) => (
                  <TabsTrigger
                    key={id}
                    value={id}
                    className={cn(
                      'h-auto w-full justify-start gap-2 rounded-md px-3 py-2 text-xs shadow-none',
                      variant === 'meeting'
                        ? cn('ring-offset-[var(--meet-bg-panel)]', settingsSidebarTabClass)
                        : 'data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm',
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
                variant === 'meeting' && meetingPanelScopeClass,
              )}
            >
              <TabsContent value="profile" className="mt-0 outline-none">
                <ProfileSettingsPanel tone={panelTone} />
              </TabsContent>
              <TabsContent value="appearance" className="mt-0 outline-none">
                <AppearanceSettingsPanel tone={panelTone} />
              </TabsContent>
              {includeSecurity ? (
                <TabsContent value="security" className="mt-0 outline-none">
                  <SecuritySettingsPanel />
                </TabsContent>
              ) : null}
              <TabsContent value="audio" className="mt-0 outline-none">
                <AudioSettingsPanel tone={panelTone} />
              </TabsContent>
              <TabsContent value="video" className="mt-0 outline-none">
                {inMeeting ? <MeetingVideoSettingsPanel /> : <VideoSettingsPanel tone={panelTone} />}
              </TabsContent>
              <TabsContent value="experimental" className="mt-0 outline-none">
                <ExperimentalSettingsPanel tone={panelTone} />
              </TabsContent>
            </div>
          </Tabs>
        </div>
      </DialogContent>
    </Dialog>
  )
}
