import { useEffect, useRef } from 'react'

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

function getFocusable(el: HTMLElement): HTMLElement[] {
  return Array.from(el.querySelectorAll<HTMLElement>(FOCUSABLE))
}

interface UseFocusTrapOptions {
  enabled: boolean
  onClose: () => void
}

/**
 * Traps Tab/Shift+Tab inside the container, focuses first element on mount,
 * closes on Escape. Only active when `enabled` is true.
 */
export function useFocusTrap(options: UseFocusTrapOptions) {
  const ref = useRef<HTMLElement | null>(null)
  const prevFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!options.enabled) return

    const el = ref.current
    if (!el) return

    // Save previously focused element
    prevFocusRef.current = document.activeElement as HTMLElement

    // Focus first focusable inside
    const focusable = getFocusable(el)
    if (focusable.length > 0) {
      focusable[0].focus()
    }

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        // Stop expanded WebXDC (window listener) from also collapsing on the same Escape.
        e.stopPropagation()
        options.onClose()
        return
      }

      if (e.key !== 'Tab') return

      const current = getFocusable(el)
      if (current.length === 0) {
        e.preventDefault()
        return
      }

      const first = current[0]
      const last = current[current.length - 1]

      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault()
          last.focus()
        }
      } else {
        if (document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }

    el.addEventListener('keydown', onKeyDown)

    return () => {
      el.removeEventListener('keydown', onKeyDown)
      // Restore focus to trigger element
      prevFocusRef.current?.focus()
    }
  }, [options.enabled, options.onClose])

  return ref
}
