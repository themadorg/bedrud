import { AlertTriangle, Shield, X } from 'lucide-react'
import { useEffect, useState } from 'react'

export function SecureContextBanner() {
  const [dismissed, setDismissed] = useState(false)
  const [show, setShow] = useState(false)

  useEffect(() => {
    if (!window.isSecureContext) setShow(true)
  }, [])

  if (dismissed) return null
  if (!show) return null

  const isLocalhost = /^(localhost|127\.0\.0\.1|::1)(:\d+)?$/.test(window.location.host)

  return (
    <div
      role="alert"
      className="fixed top-3 left-1/2 -translate-x-1/2 z-60 flex items-center gap-2.5 bg-[#0f0f1e]/95 border border-yellow-500/25 rounded-xl px-4 py-2.5 shadow-[0_8px_32px_rgba(0,0,0,0.4)] backdrop-blur-lg max-w-[min(480px,calc(100vw-32px))]"
    >
      <div className="w-7 h-7 rounded-[7px] bg-yellow-500/12 flex items-center justify-center shrink-0">
        {isLocalhost ? (
          <AlertTriangle size={14} className="text-yellow-500" />
        ) : (
          <Shield size={14} className="text-yellow-500" />
        )}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-white/85 text-xs font-medium m-0">
          {isLocalhost ? 'Media access limited' : 'HTTPS required for media'}
        </p>
        <p className="text-white/50 text-[11px] mt-0.5 m-0">
          {isLocalhost
            ? 'Camera and microphone may not work over HTTP. Use HTTPS or localhost for full support.'
            : 'Camera and microphone require a secure connection. Enable TLS or access via localhost.'}
        </p>
      </div>
      <button
        type="button"
        onClick={() => setDismissed(true)}
        className="bg-none border-none p-1 cursor-pointer text-white/50 shrink-0 flex items-center"
        aria-label="Dismiss"
      >
        <X size={14} />
      </button>
    </div>
  )
}
