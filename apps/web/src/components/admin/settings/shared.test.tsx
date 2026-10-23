import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Toggle } from './shared'

afterEach(() => {
  cleanup()
})

describe('Toggle', () => {
  it('should report a change when its label is clicked', async () => {
    const onChange = vi.fn()
    render(<Toggle checked={false} onChange={onChange} label="Allow registration" />)

    await userEvent.click(screen.getByText('Allow registration'))

    expect(onChange).toHaveBeenCalledWith(true)
  })

  it('should name its switch by its label', () => {
    render(<Toggle checked={false} onChange={vi.fn()} label="Allow registration" />)

    expect(screen.getByRole('switch', { name: 'Allow registration' })).toBeTruthy()
  })
})
