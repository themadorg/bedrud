import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { MASKED_SECRET, TextInput } from './shared'

afterEach(cleanup)

function renderSecret(value = 'super-secret-value') {
  render(<TextInput type="password" value={value} onChange={() => {}} placeholder="secret" />)
  return screen.getByPlaceholderText('secret') as HTMLInputElement
}

describe('TextInput', () => {
  describe('type="password"', () => {
    it('should render the value masked', () => {
      expect(renderSecret().type).toBe('password')
    })

    it('should not offer saved site passwords', () => {
      expect(renderSecret().autocomplete).toBe('new-password')
    })

    it('should reveal the value when the show toggle is pressed and mask it again when pressed twice', () => {
      const input = renderSecret()

      fireEvent.click(screen.getByRole('button', { name: 'Show secret' }))
      expect(input.type).toBe('text')

      fireEvent.click(screen.getByRole('button', { name: 'Hide secret' }))
      expect(input.type).toBe('password')
    })

    it('should keep the masked placeholder value unchanged', () => {
      expect(renderSecret(MASKED_SECRET).value).toBe(MASKED_SECRET)
    })
  })

  describe('type="text"', () => {
    it('should render no show toggle', () => {
      render(<TextInput value="plain" onChange={() => {}} />)
      expect(screen.queryByRole('button')).toBeNull()
    })
  })
})
