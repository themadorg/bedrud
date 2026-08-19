export type SettingsPanelTone = 'default' | 'meeting'

export function isMeetingTone(tone: SettingsPanelTone) {
  return tone === 'meeting'
}

export function panelSurfaceClass(tone: SettingsPanelTone) {
  return tone === 'meeting'
    ? 'border border-[var(--meet-border)] bg-[var(--meet-surface-muted)] text-[var(--meet-fg)]'
    : 'border bg-card/50'
}

export const meetingSliderClass =
  '[&>span.relative]:bg-[var(--meet-slider-track)] [&>span.relative>span]:bg-[var(--meet-accent)] [&_[role=slider]]:border-[color-mix(in_oklab,var(--meet-accent)_50%,transparent)] [&_[role=slider]]:bg-[var(--meet-slider-thumb)]'

export const meetingPanelScopeClass =
  '[&_.bg-card]:border-[var(--meet-border)] [&_.bg-card]:bg-[var(--meet-surface-muted)] [&_.bg-card]:text-[var(--meet-fg)] [&_.text-muted-foreground]:text-[var(--meet-fg-muted)] [&_.border-b]:border-[var(--meet-border)] [&_.divide-y>:not([hidden])~:not([hidden])]:border-[var(--meet-border)] [&_button[role=combobox]]:border-[var(--meet-border)] [&_button[role=combobox]]:bg-[var(--meet-control)] [&_button[role=combobox]]:text-[var(--meet-fg)]'

export const settingsDialogScrollClass = 'meet-scroll'

export const appScrollClass = 'app-scroll'

export const appScrollYClass = 'app-scroll-y'

export const settingsSidebarTabClass =
  'text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control)] hover:text-[var(--meet-fg-strong)] data-[state=active]:!bg-[var(--meet-btn-muted-bg)] data-[state=active]:!text-[var(--meet-btn-muted-fg)] data-[state=active]:!shadow-none'
