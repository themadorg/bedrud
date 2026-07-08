import { FlaskConical } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface WhiteboardExperimentalDialogProps {
  open: boolean
  onContinue: () => void
  onCancel: () => void
}

export function WhiteboardExperimentalDialog({ open, onContinue, onCancel }: WhiteboardExperimentalDialogProps) {
  return (
    <Dialog open={open} onOpenChange={() => {}}>
      <DialogContent
        className="meet-dialog sm:max-w-sm [&>button]:hidden"
        onInteractOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-white">
            <FlaskConical className="h-4 w-4 shrink-0 text-amber-400" aria-hidden />
            Experimental whiteboard
          </DialogTitle>
          <DialogDescription className="text-white/55">
            This whiteboard is experimental. Bugs may occur — please report anything odd you notice.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className="flex-col gap-2 sm:flex-col">
          <Button className="w-full border-none bg-primary text-white hover:bg-primary/90" onClick={onContinue}>
            Continue
          </Button>
          <Button variant="ghost" className="w-full text-white/50 hover:text-white/70" onClick={onCancel}>
            Cancel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
