import type { ExcalidrawTextElement, OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import type { AppState, BinaryFileData, BinaryFiles, ExcalidrawImperativeAPI } from '@excalidraw/excalidraw/types'
import * as Y from 'yjs'
import {
  normalizeRemoteScene,
  sceneElementsSignature,
  type WhiteboardScenePayload,
} from './excalidrawSceneUtils'
import {
  filterElementsForLocalSync,
  mergeElementsWithLocks,
  type ElementLockSnapshot,
} from './whiteboardElementLocks'
import {
  pickSyncableSettings,
  settingsSignature,
  WHITEBOARD_SYNC_SETTINGS_KEYS,
  type WhiteboardSyncSettings,
} from './whiteboardSyncSettings'

export const EXCALIDRAW_YJS_ORIGIN = Symbol('excalidraw-yjs-binding')

const LOCAL_SYNC_DEBOUNCE_MS = 60

function readSettingsFromDoc(doc: Y.Doc): Partial<WhiteboardSyncSettings> | null {
  const ySettings = doc.getMap<WhiteboardSyncSettings[keyof WhiteboardSyncSettings]>('settings')
  const settings: Partial<WhiteboardSyncSettings> = {}
  let hasAny = false

  for (const key of WHITEBOARD_SYNC_SETTINGS_KEYS) {
    const value = ySettings.get(key)
    if (key === 'viewBackgroundColor') {
      if (typeof value !== 'string') continue
      settings.viewBackgroundColor = value
      hasAny = true
      continue
    }
    if (key === 'gridModeEnabled') {
      if (typeof value !== 'boolean') continue
      settings.gridModeEnabled = value
      hasAny = true
    }
  }

  return hasAny ? settings : null
}

function syncSettingsToDoc(doc: Y.Doc, settings: WhiteboardSyncSettings) {
  const ySettings = doc.getMap<WhiteboardSyncSettings[keyof WhiteboardSyncSettings]>('settings')
  doc.transact(() => {
    for (const key of WHITEBOARD_SYNC_SETTINGS_KEYS) {
      const value = settings[key]
      if (value == null) {
        if (ySettings.has(key)) ySettings.delete(key)
        continue
      }
      if (ySettings.get(key) !== value) ySettings.set(key, value)
    }
  }, EXCALIDRAW_YJS_ORIGIN)
}

type PointList = readonly (readonly [number, number])[]

function cloneElement(el: OrderedExcalidrawElement): OrderedExcalidrawElement {
  return structuredClone(el)
}

function pointsSignature(points: PointList | undefined): string {
  if (!points || points.length === 0) return '0'
  const last = points[points.length - 1]
  return `${points.length}:${last[0]},${last[1]}`
}

function pointCount(el: OrderedExcalidrawElement): number {
  if (!('points' in el) || !Array.isArray(el.points)) return 0
  return (el.points as PointList).length
}

function pickNewerElement(
  local: OrderedExcalidrawElement,
  remote: OrderedExcalidrawElement,
): OrderedExcalidrawElement {
  if (remote.version > local.version) return remote
  if (local.version > remote.version) return local

  const localPoints = pointCount(local)
  const remotePoints = pointCount(remote)
  if (remotePoints > localPoints) return remote
  if (localPoints > remotePoints) return local

  if (elementChanged(local, remote)) return remote
  return local
}

export type BindExcalidrawToYDocOptions = {
  localIdentity: string
  getLocks: () => ElementLockSnapshot
}

function elementChanged(prev: OrderedExcalidrawElement | undefined, next: OrderedExcalidrawElement): boolean {
  if (!prev) return true
  if (prev.version !== next.version) return true

  if (prev.type === 'text' && next.type === 'text') {
    return (
      (prev as ExcalidrawTextElement).originalText !== (next as ExcalidrawTextElement).originalText
    )
  }

  const prevPoints = 'points' in prev ? (prev.points as PointList | undefined) : undefined
  const nextPoints = 'points' in next ? (next.points as PointList | undefined) : undefined
  if (pointsSignature(prevPoints) !== pointsSignature(nextPoints)) return true

  if (prev.x !== next.x || prev.y !== next.y || prev.width !== next.width || prev.height !== next.height) {
    return true
  }

  return false
}

function readSceneFromDoc(doc: Y.Doc): WhiteboardScenePayload {
  const yElements = doc.getMap<OrderedExcalidrawElement>('elements')
  const yOrder = doc.getArray<string>('order')
  const yFiles = doc.getMap<BinaryFileData>('files')

  const elements = yOrder
    .toArray()
    .map((id) => yElements.get(id))
    .filter((el): el is OrderedExcalidrawElement => el != null)

  const files: BinaryFiles = {}
  yFiles.forEach((file, id) => {
    files[id] = file
  })

  return normalizeRemoteScene({ elements, files })
}

function syncSceneToDoc(doc: Y.Doc, scene: WhiteboardScenePayload) {
  const yElements = doc.getMap<OrderedExcalidrawElement>('elements')
  const yOrder = doc.getArray<string>('order')
  const yFiles = doc.getMap<BinaryFileData>('files')

  doc.transact(() => {
    const nextIds = scene.elements.map((el) => el.id)
    const prevIds = yOrder.toArray()

    if (nextIds.join('|') !== prevIds.join('|')) {
      yOrder.delete(0, yOrder.length)
      yOrder.push(nextIds)
    }

    const nextIdSet = new Set(nextIds)
    for (const id of prevIds) {
      if (!nextIdSet.has(id)) yElements.delete(id)
    }

    for (const el of scene.elements) {
      const prev = yElements.get(el.id)
      if (elementChanged(prev, el)) {
        yElements.set(el.id, cloneElement(el))
      }
    }

    const nextFiles = scene.files ?? {}
    const nextFileIds = new Set(Object.keys(nextFiles))
    yFiles.forEach((_, id) => {
      if (!nextFileIds.has(id)) yFiles.delete(id)
    })
    for (const [id, file] of Object.entries(nextFiles)) {
      yFiles.set(id, structuredClone(file))
    }
  }, EXCALIDRAW_YJS_ORIGIN)
}

export interface ExcalidrawYjsBinding {
  onExcalidrawChange: (
    elements: readonly OrderedExcalidrawElement[],
    appState: AppState,
    files: BinaryFiles,
  ) => void
  flush: () => void
  destroy: () => void
}

export function bindExcalidrawToYDoc(
  api: ExcalidrawImperativeAPI,
  doc: Y.Doc,
  options: BindExcalidrawToYDocOptions,
): ExcalidrawYjsBinding {
  const { localIdentity, getLocks } = options
  const yElements = doc.getMap<OrderedExcalidrawElement>('elements')
  const yOrder = doc.getArray<string>('order')
  const yFiles = doc.getMap<BinaryFileData>('files')

  const ySettings = doc.getMap<WhiteboardSyncSettings[keyof WhiteboardSyncSettings]>('settings')

  let applyingRemote = false
  let trackedSignature = ''
  let trackedSettingsSignature = ''
  let debounceTimer: number | null = null
  let pendingScene: WhiteboardScenePayload | null = null

  const pushLocalScene = (scene: WhiteboardScenePayload) => {
    const locks = getLocks()
    const filteredElements = filterElementsForLocalSync(scene.elements, locks, localIdentity, yElements)
    const filteredScene = { ...scene, elements: filteredElements }
    const signature = sceneElementsSignature(filteredScene.elements)
    if (signature === trackedSignature) return
    syncSceneToDoc(doc, filteredScene)
    trackedSignature = signature
  }

  const flush = () => {
    if (debounceTimer != null) {
      window.clearTimeout(debounceTimer)
      debounceTimer = null
    }
    if (pendingScene) {
      pushLocalScene(pendingScene)
      pendingScene = null
    }
  }

  const reapplyDocToExcalidraw = () => {
    if (applyingRemote) return
    applyingRemote = true

    const remoteScene = readSceneFromDoc(doc)
    const remoteSettings = readSettingsFromDoc(doc)
    const localElements = api.getSceneElementsIncludingDeleted()
    const mergedElements = mergeElementsWithLocks(
      localElements,
      remoteScene.elements,
      getLocks(),
      localIdentity,
      pickNewerElement,
    )
    const scene = { ...remoteScene, elements: mergedElements }
    trackedSignature = sceneElementsSignature(scene.elements)

    api.updateScene({
      elements: scene.elements,
      appState: remoteSettings as Pick<AppState, 'viewBackgroundColor' | 'gridModeEnabled'> | undefined,
      captureUpdate: 'NEVER',
    })

    if (remoteSettings) {
      trackedSettingsSignature = settingsSignature(remoteSettings)
    }

    if (scene.files && Object.keys(scene.files).length > 0) {
      api.addFiles(Object.values(scene.files))
    }

    queueMicrotask(() => {
      applyingRemote = false
    })
  }

  const applyDocToExcalidraw = (_events: unknown, transaction: Y.Transaction) => {
    if (transaction.origin === EXCALIDRAW_YJS_ORIGIN || applyingRemote) return
    reapplyDocToExcalidraw()
  }

  const yLocks = doc.getMap('locks')
  const applyLocksChange = () => reapplyDocToExcalidraw()

  yElements.observe(applyDocToExcalidraw)
  yOrder.observe(applyDocToExcalidraw)
  yFiles.observe(applyDocToExcalidraw)
  ySettings.observe(applyDocToExcalidraw)
  yLocks.observe(applyLocksChange)

  if (yOrder.length > 0 || ySettings.size > 0) {
    applyingRemote = true
    const scene = readSceneFromDoc(doc)
    const initialSettings = readSettingsFromDoc(doc)
    trackedSignature = sceneElementsSignature(scene.elements)
    api.updateScene({
      elements: scene.elements,
      appState: initialSettings as Pick<AppState, 'viewBackgroundColor' | 'gridModeEnabled'> | undefined,
      captureUpdate: 'NEVER',
    })
    if (initialSettings) {
      trackedSettingsSignature = settingsSignature(initialSettings)
    }
    if (scene.files && Object.keys(scene.files).length > 0) {
      api.addFiles(Object.values(scene.files))
    }
    applyingRemote = false
  }

  const onExcalidrawChange = (
    elements: readonly OrderedExcalidrawElement[],
    appState: AppState,
    files: BinaryFiles,
  ) => {
    if (applyingRemote) return

    const settings = pickSyncableSettings(appState)
    const nextSettingsSignature = settingsSignature(settings)
    if (nextSettingsSignature !== trackedSettingsSignature) {
      syncSettingsToDoc(doc, settings)
      trackedSettingsSignature = nextSettingsSignature
    }

    pendingScene = { elements, files }
    if (debounceTimer != null) return

    debounceTimer = window.setTimeout(() => {
      debounceTimer = null
      if (!pendingScene) return
      pushLocalScene(pendingScene)
      pendingScene = null
    }, LOCAL_SYNC_DEBOUNCE_MS)
  }

  return {
    onExcalidrawChange,
    flush,
    destroy: () => {
      flush()
      if (debounceTimer != null) window.clearTimeout(debounceTimer)
      yElements.unobserve(applyDocToExcalidraw)
      yOrder.unobserve(applyDocToExcalidraw)
      yFiles.unobserve(applyDocToExcalidraw)
      ySettings.unobserve(applyDocToExcalidraw)
      yLocks.unobserve(applyLocksChange)
    },
  }
}