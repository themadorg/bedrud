// @vitest-environment node
//
// This file reads a route module from disk, so it runs in Node rather than jsdom: jsdom's URL
// resolves `import.meta.url` against http://localhost instead of the file system.
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const rootRouteSource = readFileSync(new URL('./__root.tsx', import.meta.url), 'utf8')

describe('the root route', () => {
  it('should mount the app toaster rather than sonner directly', () => {
    expect(rootRouteSource).toContain("import { Toaster } from '@/components/ui/sonner'")
  })

  it('should not tint toasts by kind', () => {
    expect(rootRouteSource).not.toContain('richColors')
  })
})
