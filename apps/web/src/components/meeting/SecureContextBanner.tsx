import { AlertTriangle, Shield, X } from 'lucide-react'
import { useState } from 'react'

export function SecureContextBanner() {
  const [dismissed, setDismissed] = useState(false)

  if (dismissed) return null
  if (typeof window === 'undefined' || window.isSecureContext) return null

  const isLocalhost = /^(localhost|127\.0\.0\.1|::1)(:\d+)?$/.test(window.location.host)

  return (
    <div
      role="alert"
      style={{
        position: 'fixed',
        top: 12,
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 60,
        background: 'rgba(15,15,30,0.95)',
        border: '1px solid rgba(234,179,8,0.25)',
        borderRadius: 12,
        padding: '10px 16px',
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
        backdropFilter: 'blur(16px)',
        maxWidth: 'min(480px, calc(100vw - 32px))',
      }}
    >
      <div
        style={{
          width: 28,
          height: 28,
          borderRadius: 7,
          background: 'rgba(234,179,8,0.12)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
        }}
      >
        {isLocalhost ? (
          <AlertTriangle size={14} style={{ color: '#eab308' }} />
        ) : (
          <Shield size={14} style={{ color: '#eab308' }} />
        )}
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <p style={{ color: 'rgba(255,255,255,0.85)', fontSize: 12, fontWeight: 500, margin: 0 }}>
          {isLocalhost ? 'Media access limited' : 'HTTPS required for media'}
        </p>
        <p style={{ color: 'rgba(255,255,255,0.45)', fontSize: 11, margin: '2px 0 0' }}>
          {isLocalhost
            ? 'Camera and microphone may not work over HTTP. Use HTTPS or localhost for full support.'
            : 'Camera and microphone require a secure connection. Enable TLS or access via localhost.'}
        </p>
      </div>
      <button
        type="button"
        onClick={() => setDismissed(true)}
        style={{
          background: 'none',
          border: 'none',
          padding: 4,
          cursor: 'pointer',
          color: 'rgba(255,255,255,0.3)',
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
        }}
        aria-label="Dismiss"
      >
        <X size={14} />
      </button>
    </div>
  )
}
