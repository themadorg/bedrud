import { BedrudSettingsDialog } from '#/components/settings/BedrudSettingsDialog'

type SettingsTab = 'profile' | 'appearance' | 'audio' | 'video' | 'security' | 'experimental'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  includeSecurity?: boolean
  initialTab?: SettingsTab
  initialMobilePage?: SettingsTab | null
  dockAboveMobileNav?: boolean
}

export function HomeSettingsDialog({
  open,
  onOpenChange,
  includeSecurity = false,
  initialTab,
  initialMobilePage = null,
  dockAboveMobileNav = false,
}: Props) {
  return (
    <BedrudSettingsDialog
      open={open}
      onOpenChange={onOpenChange}
      inMeeting={false}
      includeSecurity={includeSecurity}
      initialTab={initialTab}
      initialMobilePage={initialMobilePage}
      dockAboveMobileNav={dockAboveMobileNav}
    />
  )
}
