import { Fingerprint, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { dismissPasskeyOffer, passkeyErrorMessage, registerPasskey } from '#/lib/passkey'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

interface Props {
  userId: string
  /** Leaves the auth page, whether or not a passkey was added. */
  onDone: () => void
}

/**
 * One-time offer, shown after a password sign-in, to add a passkey for next time. It is the only
 * place outside Settings where a passkey is created, so the sign-in and register forms never have
 * to explain passkeys while the user is still filling them in.
 */
export function PasskeyOffer({ userId, onDone }: Props) {
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState('')

  async function handleAdd() {
    setAdding(true)
    setError('')
    try {
      await registerPasskey()
      onDone()
    } catch (err) {
      setError(passkeyErrorMessage(err, 'Could not add a passkey'))
      setAdding(false)
    }
  }

  function handleNotNow() {
    dismissPasskeyOffer(userId)
    onDone()
  }

  return (
    <div className="space-y-7">
      <div className="space-y-2">
        <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-accent text-accent-foreground">
          <Fingerprint className="h-7 w-7 text-primary" aria-hidden />
        </div>
        <h1 className="text-2xl font-semibold tracking-tight">Sign in faster next time</h1>
        <p className="text-sm text-muted-foreground">
          Add a passkey and sign in with your fingerprint, face, or screen lock instead of typing your password. Your
          password keeps working.
        </p>
      </div>

      {error ? <Alert type="error" message={error} /> : null}

      <div className="space-y-3">
        <Button className="w-full gap-2" onClick={() => void handleAdd()} disabled={adding}>
          {adding ? <Loader2 className="h-4 w-4 animate-spin" /> : <Fingerprint className="h-4 w-4" />}
          {adding ? 'Adding passkey…' : 'Add a passkey'}
        </Button>
        <Button variant="ghost" className="w-full" onClick={handleNotNow} disabled={adding}>
          Not now
        </Button>
      </div>

      <p className="text-xs text-muted-foreground">You can add or review passkeys any time in Settings → Security.</p>
    </div>
  )
}
