import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { applyTheme, useThemeStore } from './theme.store'

/** Makes `--background` resolve to `value` without loading the real stylesheet into jsdom. */
function installBackgroundToken(value: string) {
  vi.spyOn(window, 'getComputedStyle').mockReturnValue({
    getPropertyValue: () => ` ${value} `,
  } as unknown as CSSStyleDeclaration)
}

function themeColorMeta(): string | null | undefined {
  return document.querySelector('meta[name="theme-color"]')?.getAttribute('content')
}

describe('applyTheme', () => {
  beforeEach(() => {
    document.head.innerHTML = '<meta name="theme-color" content="#fafafa">'
    document.documentElement.classList.remove('dark')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should toggle the dark class on the root element', () => {
    installBackgroundToken('#0C0A09')
    applyTheme('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    applyTheme('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('should copy the resolved background token into the theme-color meta', () => {
    installBackgroundToken('#0C0A09')
    applyTheme('dark')
    expect(themeColorMeta()).toBe('#0C0A09')
  })

  it('should leave the meta alone when the token does not resolve', () => {
    installBackgroundToken('')
    applyTheme('dark')
    expect(themeColorMeta()).toBe('#fafafa')
  })

  it('should create the meta when the document has none', () => {
    document.head.innerHTML = ''
    installBackgroundToken('#0C0A09')
    applyTheme('dark')
    expect(document.head.querySelectorAll('meta[name="theme-color"]')).toHaveLength(1)
    expect(themeColorMeta()).toBe('#0C0A09')
  })
})

describe('setTheme', () => {
  beforeEach(() => {
    document.head.innerHTML = '<meta name="theme-color" content="#fafafa">'
    document.documentElement.classList.remove('dark')
    useThemeStore.setState({ theme: 'light' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should apply the class and the theme colour when the class is stale', () => {
    installBackgroundToken('#0C0A09')
    useThemeStore.getState().setTheme('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(themeColorMeta()).toBe('#0C0A09')
  })

  it('should refresh the theme colour when a view transition already toggled the class', () => {
    installBackgroundToken('#0C0A09')
    document.documentElement.classList.add('dark')
    useThemeStore.getState().setTheme('dark')
    expect(themeColorMeta()).toBe('#0C0A09')
  })
})
