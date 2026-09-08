#!/usr/bin/env node
/**
 * Renders the PWA icons from scripts/app-icon.svg into public/. Run `bun run pwa-icons`
 * after editing the SVG; the PNGs are generated files and are never edited by hand.
 */
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import sharp from 'sharp'

const scriptsDirectory = path.dirname(fileURLToPath(import.meta.url))
const sourceSvg = path.join(scriptsDirectory, 'app-icon.svg')
const publicDirectory = path.join(scriptsDirectory, '..', 'public')

// Android masks the 512px icon with the launcher's shape; iOS rounds the 180px one itself.
const outputs = [
  { fileName: 'icon-maskable-512.png', sizePx: 512 },
  { fileName: 'apple-touch-icon.png', sizePx: 180 },
]

for (const { fileName, sizePx } of outputs) {
  await sharp(sourceSvg).resize(sizePx, sizePx).png().toFile(path.join(publicDirectory, fileName))
  console.log(`wrote public/${fileName} (${sizePx}x${sizePx})`)
}
