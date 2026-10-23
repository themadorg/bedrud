import { act, cleanup, render, screen } from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, describe, expect, it } from 'vitest'

import { Toaster } from './sonner'

async function show(kind: 'success' | 'info' | 'warning' | 'error', message: string) {
  render(<Toaster />)
  await act(async () => {
    toast[kind](message)
  })
  return screen.findByText(message)
}

describe('Toaster', () => {
  afterEach(() => {
    toast.dismiss()
    cleanup()
  })

  it.each([
    ['success', 'Success'],
    ['info', 'Information'],
    ['warning', 'Warning'],
    ['error', 'Error'],
  ] as const)('should lead a %s toast with an icon named %s', async (kind, name) => {
    await show(kind, `a ${kind} message`)

    expect(screen.getByRole('img', { name })).toBeTruthy()
  })

  // Rich colours tinted each kind's whole bar; Android #283 keeps one inverse bar for every kind and
  // lets the icon carry the kind.
  it('should draw every kind on one bar rather than tinting it', async () => {
    const message = await show('error', 'tinted?')

    const toastElement = message.closest('[data-sonner-toast]')

    expect(toastElement).toBeTruthy()
    expect(toastElement?.getAttribute('data-rich-colors')).not.toBe('true')
  })
})
