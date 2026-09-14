// @vitest-environment node
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { LIGHT_THEME_COLOR } from './lib/document-head'

interface ManifestIcon {
  src: string
  sizes: string
  type: string
  purpose?: string
}

const publicDir = new URL('../public/', import.meta.url)
const manifest = JSON.parse(readFileSync(new URL('manifest.json', publicDir), 'utf8'))
const pngIcons: ManifestIcon[] = manifest.icons.filter((icon: ManifestIcon) => icon.type === 'image/png')

/** Reads width and height from a PNG's IHDR chunk, which starts at byte 16. */
function pngSize(src: string): string {
  const bytes = readFileSync(new URL(src.replace(/^\//, ''), publicDir))
  return `${bytes.readUInt32BE(16)}x${bytes.readUInt32BE(20)}`
}

describe('manifest.json', () => {
  it('should open on the rooms list like the Android app', () => {
    expect(manifest.start_url).toBe('/dashboard')
    expect(manifest.scope).toBe('/')
    expect(manifest.display).toBe('standalone')
  })

  it('should paint the splash and system bar in the light surface colour', () => {
    expect(manifest.theme_color).toBe(LIGHT_THEME_COLOR)
    expect(manifest.background_color).toBe(LIGHT_THEME_COLOR)
  })

  it('should ship a maskable 512px icon for Android home screens', () => {
    const maskable = pngIcons.find((icon) => icon.purpose === 'maskable')
    expect(maskable).toEqual({
      src: '/icon-maskable-512.png',
      sizes: '512x512',
      type: 'image/png',
      purpose: 'maskable',
    })
  })

  it('should declare every PNG icon at its real size', () => {
    for (const icon of pngIcons) {
      expect(pngSize(icon.src), icon.src).toBe(icon.sizes)
    }
  })
})

describe('apple-touch-icon.png', () => {
  it('should be 180px square', () => {
    expect(pngSize('/apple-touch-icon.png')).toBe('180x180')
  })
})
