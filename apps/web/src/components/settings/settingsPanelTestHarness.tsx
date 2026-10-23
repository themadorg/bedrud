import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'
import { vi } from 'vitest'

function Providers({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>
}

/**
 * Renders a settings panel in jsdom, which has no `matchMedia`, `ResizeObserver` or media devices,
 * and whose `fetch` cannot reach the relative API paths the panels sync with. All are stubbed to
 * report nothing: no query matches, nothing resizes, there are no cameras or microphones to list,
 * and the server answers every request with an empty object.
 *
 * The stubs stay for the rest of the test file. A panel syncs a changed setting on a timer that can
 * fire after its test ends, and unstubbing would hand that late request the real `fetch`.
 */
export function renderSettingsPanel(panel: ReactElement) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => new Response('{}', { status: 200 })),
  )
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  )
  vi.stubGlobal(
    'matchMedia',
    vi.fn((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
    })),
  )
  Object.defineProperty(navigator, 'mediaDevices', {
    configurable: true,
    value: {
      enumerateDevices: vi.fn(async () => []),
      getUserMedia: vi.fn(async () => {
        throw new DOMException('No device in tests', 'NotFoundError')
      }),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    },
  })
  return render(panel, { wrapper: Providers })
}

/** Returns whether the switch named by its row's text is on, read from the switch the label points at. */
export function switchState(name: RegExp): string | null {
  return screen.getByRole('switch', { name }).getAttribute('aria-checked')
}
