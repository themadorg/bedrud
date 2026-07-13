import { ExternalLink, Scale } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Field, Section, Toggle } from './shared'
import type { SystemSettings } from './types'

const KRISP_LICENSE_URL = 'https://krisp.ai/developers/'
const LIVEKIT_KRISP_DOCS_URL = 'https://docs.livekit.io/transport/media/noise-cancellation/'

export function AudioTab({
  settings,
  setSettings,
}: {
  settings: SystemSettings
  setSettings: (s: SystemSettings) => void
}) {
  const [licenseOpen, setLicenseOpen] = useState(false)

  function onToggleRNNoise(next: boolean) {
    setSettings({ ...settings, rnnoiseEnabled: next })
  }

  function onToggleKrisp(next: boolean) {
    if (next && !settings.krispEnabled) {
      setLicenseOpen(true)
      return
    }
    setSettings({ ...settings, krispEnabled: next })
  }

  function confirmEnableKrisp() {
    setSettings({ ...settings, krispEnabled: true })
    setLicenseOpen(false)
  }

  return (
    <div className="space-y-4">
      <Section
        title="Noise cancellation"
        description="Optional processors users may select. Both are off by default and not downloaded until enabled and chosen."
      >
        <Field
          label="RNNoise"
          hint="Open-source WASM noise suppression (~1.9 MB when a user selects it). Off by default so clients never download it unless you enable this."
        >
          <Toggle
            checked={settings.rnnoiseEnabled}
            onChange={onToggleRNNoise}
            label={
              settings.rnnoiseEnabled
                ? 'RNNoise allowed for this instance'
                : 'RNNoise disabled (default) — not downloaded'
            }
          />
        </Field>

        <Field
          label="Krisp"
          hint="Proprietary AI filter. Off by default. Requires your own license (e.g. LiveKit Cloud or Krisp commercial). Not downloaded unless enabled and selected."
        >
          <Toggle
            checked={settings.krispEnabled}
            onChange={onToggleKrisp}
            label={
              settings.krispEnabled ? 'Krisp allowed for this instance' : 'Krisp disabled (default) — not downloaded'
            }
          />
        </Field>

        {settings.krispEnabled && (
          <p className="text-[10px] text-amber-700 dark:text-amber-400">
            You enabled Krisp for this deployment. Ensure licensing (LiveKit Cloud or a commercial Krisp license)
            remains valid.
          </p>
        )}

        <div className="flex flex-col gap-1.5 pt-1 text-xs">
          <a
            href={KRISP_LICENSE_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 text-primary hover:underline"
          >
            Krisp developer licensing
            <ExternalLink className="h-3.5 w-3.5 shrink-0" aria-hidden />
          </a>
          <a
            href={LIVEKIT_KRISP_DOCS_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 text-muted-foreground hover:underline"
          >
            LiveKit noise cancellation docs
            <ExternalLink className="h-3.5 w-3.5 shrink-0" aria-hidden />
          </a>
        </div>
      </Section>

      <Section title="Always available" description="No large download; uses the browser">
        <p className="text-xs text-muted-foreground">
          <strong>Off</strong> and <strong>Browser</strong> noise suppression stay available to all users and do not
          load RNNoise or Krisp packages.
        </p>
      </Section>

      <Dialog open={licenseOpen} onOpenChange={(open) => !open && setLicenseOpen(false)}>
        <DialogContent className="sm:max-w-md" onInteractOutside={(e) => e.preventDefault()}>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Scale className="h-4 w-4 shrink-0 text-amber-500" aria-hidden />
              Krisp license required
            </DialogTitle>
            <DialogDescription className="space-y-3 text-left">
              <span className="block">
                Krisp is proprietary. Bedrud only ships optional integration code and does not provide a Krisp license.
              </span>
              <span className="block">
                Before enabling Krisp on this instance, verify that you have a valid right to use it (for example
                LiveKit Cloud, or a commercial license from Krisp).
              </span>
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2 text-sm">
            <a
              href={KRISP_LICENSE_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-primary hover:underline"
            >
              Krisp developer licensing
              <ExternalLink className="h-3.5 w-3.5 shrink-0" aria-hidden />
            </a>
          </div>
          <DialogFooter className="flex-col gap-2 sm:flex-col">
            <Button className="w-full" onClick={confirmEnableKrisp}>
              I have checked licensing — enable Krisp
            </Button>
            <Button variant="ghost" className="w-full" onClick={() => setLicenseOpen(false)}>
              Cancel
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
