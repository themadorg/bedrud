import { useEffect, useState } from 'react'
import type { ParticipantPalette } from '#/lib/participant-palette'

const cache = new Map<string, ParticipantPalette>()

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function toHex(r: number, g: number, b: number) {
  return `#${[r, g, b]
    .map((channel) => clamp(Math.round(channel), 0, 255).toString(16).padStart(2, '0'))
    .join('')}`
}

function loadImage(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.referrerPolicy = 'no-referrer'
    img.onload = () => resolve(img)
    img.onerror = () => reject(new Error('avatar load failed'))
    img.src = url
  })
}

export async function extractAvatarPhotoPalette(url: string): Promise<ParticipantPalette | null> {
  const cached = cache.get(url)
  if (cached) return cached

  try {
    const img = await loadImage(url)
    const canvas = document.createElement('canvas')
    const size = 32
    canvas.width = size
    canvas.height = size
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    if (!ctx) return null

    ctx.drawImage(img, 0, 0, size, size)
    const { data } = ctx.getImageData(0, 0, size, size)

    let rSum = 0
    let gSum = 0
    let bSum = 0
    let count = 0
    let accentR = 128
    let accentG = 128
    let accentB = 128
    let bestScore = -1

    for (let i = 0; i < data.length; i += 4) {
      const alpha = data[i + 3]!
      if (alpha < 48) continue
      const r = data[i]!
      const g = data[i + 1]!
      const b = data[i + 2]!
      rSum += r
      gSum += g
      bSum += b
      count += 1

      const max = Math.max(r, g, b)
      const min = Math.min(r, g, b)
      const saturation = max === 0 ? 0 : (max - min) / max
      const score = saturation * 0.75 + (max / 255) * 0.25
      if (score > bestScore) {
        bestScore = score
        accentR = r
        accentG = g
        accentB = b
      }
    }

    if (count === 0) return null

    const avgR = rSum / count
    const avgG = gSum / count
    const avgB = bSum / count
    const shadeR = accentR * 0.72
    const shadeG = accentG * 0.72
    const shadeB = accentB * 0.72

    const palette: ParticipantPalette = {
      tile: `rgba(${Math.round(avgR)},${Math.round(avgG)},${Math.round(avgB)},0.18)`,
      avatar: `linear-gradient(135deg,${toHex(accentR, accentG, accentB)},${toHex(shadeR, shadeG, shadeB)})`,
      glow: `rgba(${Math.round(avgR)},${Math.round(avgG)},${Math.round(avgB)},0.45)`,
    }

    cache.set(url, palette)
    return palette
  } catch {
    return null
  }
}

export function useAvatarPhotoPalette(resolvedUrl?: string): ParticipantPalette | null {
  const [palette, setPalette] = useState<ParticipantPalette | null>(() =>
    resolvedUrl ? (cache.get(resolvedUrl) ?? null) : null,
  )

  useEffect(() => {
    if (!resolvedUrl) {
      setPalette(null)
      return
    }

    const cached = cache.get(resolvedUrl)
    if (cached) {
      setPalette(cached)
      return
    }

    let cancelled = false
    void extractAvatarPhotoPalette(resolvedUrl).then((next) => {
      if (!cancelled) setPalette(next)
    })

    return () => {
      cancelled = true
    }
  }, [resolvedUrl])

  return palette
}
