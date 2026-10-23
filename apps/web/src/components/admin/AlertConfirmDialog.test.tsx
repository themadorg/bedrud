import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { AlertConfirmDialog } from './AlertConfirmDialog'

function renderDialog(variant?: 'destructive' | 'secondary') {
  render(
    <AlertConfirmDialog
      open
      onOpenChange={() => {}}
      title="Ban Someone?"
      description="They are removed at once."
      confirmLabel="Ban"
      onConfirm={() => {}}
      variant={variant}
    />,
  )
}

describe('AlertConfirmDialog', () => {
  afterEach(cleanup)

  // Cancel was an outlined button beside a filled confirm, so the two disagreed on weight; Android
  // #204 made every dialog's cancel the ghost button.
  it('should draw cancel as a ghost button', () => {
    renderDialog()

    const cancel = screen.getByRole('button', { name: 'Cancel' })

    expect(cancel.className).not.toContain('border-input')
    expect(cancel.className).toContain('hover:bg-accent')
  })

  it('should fill the confirm with the destructive colour by default', () => {
    renderDialog()

    expect(screen.getByRole('button', { name: 'Ban' }).className).toContain('bg-destructive')
  })

  it('should draw a confirm that loses nothing as a secondary button', () => {
    renderDialog('secondary')

    const confirm = screen.getByRole('button', { name: 'Ban' })

    expect(confirm.className).toContain('bg-secondary')
    expect(confirm.className).not.toContain('bg-destructive')
  })
})
