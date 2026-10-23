import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { OpenSourceLicensesLink } from './OpenSourceLicensesLink'

afterEach(() => {
  cleanup()
})

describe('OpenSourceLicensesLink', () => {
  it('should point at the licence report the build writes', () => {
    render(<OpenSourceLicensesLink />)

    expect(screen.getByRole('link', { name: /Open-source licenses/ }).getAttribute('href')).toBe(
      '/third-party-licenses.txt',
    )
  })

  it('should open the report in a new tab without handing it this page', () => {
    render(<OpenSourceLicensesLink />)
    const link = screen.getByRole('link', { name: /Open-source licenses/ })

    expect(link.getAttribute('target')).toBe('_blank')
    expect(link.getAttribute('rel')).toBe('noopener noreferrer')
  })
})
