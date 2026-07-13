import { ExternalLink, Scale } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const KRISP_LICENSE_URL = 'https://krisp.ai/developers/'
const LIVEKIT_KRISP_DOCS_URL = 'https://docs.livekit.io/transport/media/noise-cancellation/'

interface KrispLicenseDialogProps {
  open: boolean
  onConfirm: () => void
  onCancel: () => void
  /** Meeting-room dark chrome */
  meeting?: boolean
}

/**
 * Shown whenever the user tries to enable Krisp. Krisp is proprietary —
 * Bedrud only wires the optional LiveKit package; operators must verify
 * their own license (e.g. LiveKit Cloud or a commercial Krisp license).
 */
export function KrispLicenseDialog({ open, onConfirm, onCancel, meeting }: KrispLicenseDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onCancel()
      }}
    >
      <DialogContent
        className={
          meeting
            ? 'meet-dialog sm:max-w-md border-[var(--meet-border)] bg-[var(--meet-surface)] text-[var(--meet-fg-strong)]'
            : 'sm:max-w-md'
        }
        onInteractOutside={(e) => e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle className={meeting ? 'flex items-center gap-2 text-white' : 'flex items-center gap-2'}>
            <Scale className="h-4 w-4 shrink-0 text-amber-500" aria-hidden />
            Krisp license required
          </DialogTitle>
          <DialogDescription className={meeting ? 'space-y-3 text-left text-white/55' : 'space-y-3 text-left'}>
            <span className="block">
              Krisp is a proprietary noise-cancellation product. Bedrud does not include a Krisp license and does not
              enable Krisp by default.
            </span>
            <span className="block">
              Before turning it on, you must verify that you (or your deployment) have a valid right to use Krisp — for
              example via LiveKit Cloud, or a commercial license from Krisp. Check licensing yourself on the Krisp
              developer site.
            </span>
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-2 text-sm">
          <a
            href={KRISP_LICENSE_URL}
            target="_blank"
            rel="noopener noreferrer"
            className={
              meeting
                ? 'inline-flex items-center gap-1.5 text-teal-400 hover:underline'
                : 'inline-flex items-center gap-1.5 text-primary hover:underline'
            }
          >
            Krisp developer licensing
            <ExternalLink className="h-3.5 w-3.5 shrink-0" aria-hidden />
          </a>
          <a
            href={LIVEKIT_KRISP_DOCS_URL}
            target="_blank"
            rel="noopener noreferrer"
            className={
              meeting
                ? 'inline-flex items-center gap-1.5 text-teal-400/80 hover:underline'
                : 'inline-flex items-center gap-1.5 text-muted-foreground hover:underline'
            }
          >
            LiveKit noise cancellation docs
            <ExternalLink className="h-3.5 w-3.5 shrink-0" aria-hidden />
          </a>
        </div>

        <DialogFooter className="flex-col gap-2 sm:flex-col">
          <Button
            className={meeting ? 'w-full border-none bg-primary text-white hover:bg-primary/90' : 'w-full'}
            onClick={onConfirm}
          >
            I have checked licensing — enable Krisp
          </Button>
          <Button
            variant="ghost"
            className={meeting ? 'w-full text-white/50 hover:text-white/70' : 'w-full'}
            onClick={onCancel}
          >
            Cancel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
