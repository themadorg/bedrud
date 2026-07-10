/**
 * Custom TanStack Start client entry.
 *
 * Default entry wraps the app in React.StrictMode, which double-mounts effects
 * in development. That disconnects LiveKitRoom after ~1–2s (CLIENT_REQUEST_LEAVE
 * on the SFU) and leaves publishData stuck on "PC manager is closed".
 *
 * We intentionally omit StrictMode so WebRTC sessions stay stable.
 */

import { StartClient } from '@tanstack/react-start/client'
import { startTransition } from 'react'
import { hydrateRoot } from 'react-dom/client'

startTransition(() => {
  hydrateRoot(document, <StartClient />)
})
