import { toast } from 'sonner'
import { useExperimentalPreferencesStore } from '#/lib/experimental-preferences.store'
import { WhiteboardExperimentalDialog } from '@/components/meeting/whiteboard/WhiteboardExperimentalDialog'
import { useWhiteboardWatch } from '@/components/meeting/whiteboard/whiteboard-watch-context'

export function WhiteboardExperimentalGate() {
  const acknowledgeDisclaimer = useExperimentalPreferencesStore((s) => s.acknowledgeWhiteboardDisclaimer)
  const {
    pendingOpen,
    confirmStartWhiteboard,
    cancelStartWhiteboard,
    acceptWhiteboard,
    declineWhiteboard,
    isHost,
    stopWhiteboard,
  } = useWhiteboardWatch()

  const handleContinue = () => {
    acknowledgeDisclaimer()
    if (pendingOpen) {
      const err = confirmStartWhiteboard()
      if (err) toast.error(err)
      return
    }
    acceptWhiteboard()
  }

  const handleCancel = () => {
    if (pendingOpen) {
      cancelStartWhiteboard()
      return
    }
    if (isHost) {
      stopWhiteboard()
      return
    }
    declineWhiteboard()
  }

  return <WhiteboardExperimentalDialog open onContinue={handleContinue} onCancel={handleCancel} />
}
