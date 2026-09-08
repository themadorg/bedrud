// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { APP_NAME, documentLinks, documentMeta, LIGHT_THEME_COLOR } from './document-head'

function metaContent(name: string): string | undefined {
  const tag = documentMeta.find((meta) => meta.name === name)
  return typeof tag?.content === 'string' ? tag.content : undefined
}

describe('documentMeta', () => {
  it('should let the viewport reach under the system bars', () => {
    expect(metaContent('viewport')).toBe('width=device-width, initial-scale=1, viewport-fit=cover')
  })

  it('should leave the theme-color meta to the pre-paint script', () => {
    expect(documentMeta.some((meta) => meta.name === 'theme-color')).toBe(false)
  })

  it('should keep the theme colour equal to the light background token', () => {
    const themeCss = readFileSync(new URL('../theme.css', import.meta.url), 'utf8')
    expect(/--background:\s*([^;]+);/.exec(themeCss)?.[1].trim()).toBe(LIGHT_THEME_COLOR)
  })

  it('should declare the page installable on Android and iOS', () => {
    expect(metaContent('mobile-web-app-capable')).toBe('yes')
    expect(metaContent('apple-mobile-web-app-title')).toBe(APP_NAME)
    expect(metaContent('apple-mobile-web-app-status-bar-style')).toBe('default')
  })
})

describe('documentLinks', () => {
  it('should put the stylesheet first and link the manifest and Apple touch icon', () => {
    const links = documentLinks('/assets/styles.css')
    expect(links[0]).toEqual({ rel: 'stylesheet', href: '/assets/styles.css' })
    expect(links).toContainEqual({ rel: 'manifest', href: '/manifest.json' })
    expect(links).toContainEqual({ rel: 'apple-touch-icon', href: '/apple-touch-icon.png' })
  })
})
