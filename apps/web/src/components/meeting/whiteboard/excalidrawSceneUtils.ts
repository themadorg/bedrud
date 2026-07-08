import { restoreElements } from '@excalidraw/excalidraw'
import type { ExcalidrawTextElement, OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import type { AppState, BinaryFiles } from '@excalidraw/excalidraw/types'
import { alignRtlTextElements } from '@/components/meeting/whiteboard/whiteboardTextDirection'

export interface WhiteboardScenePayload {
  elements: readonly OrderedExcalidrawElement[]
  appState?: Partial<AppState>
  files?: BinaryFiles
}

export function pickReferencedFiles(elements: readonly OrderedExcalidrawElement[], files: BinaryFiles): BinaryFiles {
  const fileIds = new Set<string>()
  for (const el of elements) {
    if (el.type === 'image' && 'fileId' in el && typeof el.fileId === 'string' && el.fileId) {
      fileIds.add(el.fileId)
    }
  }
  const picked: BinaryFiles = {}
  for (const id of fileIds) {
    const file = files[id]
    if (file?.dataURL) picked[id] = file
  }
  return picked
}

function markSavedImageElements(elements: OrderedExcalidrawElement[], files?: BinaryFiles): OrderedExcalidrawElement[] {
  if (!files) return elements
  return elements.map((el) => {
    if (el.type !== 'image' || !('fileId' in el) || !el.fileId || !files[el.fileId]?.dataURL) return el
    if (el.status === 'saved') return el
    return { ...el, status: 'saved' as const }
  })
}

function drawPointsSignature(el: OrderedExcalidrawElement): string | null {
  if (!('points' in el) || !Array.isArray(el.points)) return null
  const points = el.points as readonly (readonly [number, number])[]
  if (points.length === 0) return '0'
  const last = points[points.length - 1]
  return `${points.length}:${last[0]},${last[1]}`
}

export function sceneElementsSignature(elements: readonly OrderedExcalidrawElement[]): string {
  const parts: string[] = []
  for (const el of elements) {
    if (el.isDeleted) continue
    if (el.type === 'text') {
      const textEl = el as ExcalidrawTextElement
      parts.push(`${el.id}:${el.version}:${textEl.originalText ?? ''}`)
      continue
    }
    const points = drawPointsSignature(el)
    if (points) {
      parts.push(`${el.id}:${el.version}:${points}`)
      continue
    }
    parts.push(`${el.id}:${el.version}:${el.x},${el.y},${el.width},${el.height}`)
  }
  return parts.join('|')
}

export function textElementsSignature(elements: readonly OrderedExcalidrawElement[]): string {
  const parts: string[] = []
  for (const el of elements) {
    if (el.isDeleted || el.type !== 'text') continue
    const textEl = el as ExcalidrawTextElement
    parts.push(`${el.id}:${el.version}:${textEl.originalText ?? ''}`)
  }
  return parts.join('|')
}

export function restoreSceneElements(elements: readonly OrderedExcalidrawElement[]): OrderedExcalidrawElement[] {
  return restoreElements(elements, null) as OrderedExcalidrawElement[]
}

function normalizeSceneElements(
  elements: readonly OrderedExcalidrawElement[],
  files?: BinaryFiles,
): OrderedExcalidrawElement[] {
  const restored = markSavedImageElements(restoreSceneElements(elements), files)
  return alignRtlTextElements(restored) ?? restored
}

export function normalizeSceneForWire(scene: WhiteboardScenePayload): WhiteboardScenePayload {
  const files = scene.files ? pickReferencedFiles(scene.elements, scene.files) : undefined
  const elements = normalizeSceneElements(scene.elements, files)
  return {
    ...scene,
    elements,
    files: files && Object.keys(files).length > 0 ? files : undefined,
  }
}

export function normalizeRemoteScene(scene: WhiteboardScenePayload): WhiteboardScenePayload {
  const files = scene.files ? pickReferencedFiles(scene.elements, scene.files) : undefined
  const elements = normalizeSceneElements(scene.elements, files)
  return {
    ...scene,
    elements,
    files: files && Object.keys(files).length > 0 ? files : undefined,
  }
}
