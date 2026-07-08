import type { OrderedExcalidrawElement } from '@excalidraw/excalidraw/element/types'
import * as Y from 'yjs'

export const WHITEBOARD_LOCKS_ORIGIN = Symbol('whiteboard-element-lock')

export type ElementLock = {
  identity: string
  username: string
  ts: number
}

export type ElementLockSnapshot = ReadonlyMap<string, ElementLock>

export function getYLocks(doc: Y.Doc): Y.Map<ElementLock> {
  return doc.getMap<ElementLock>('locks')
}

export function readLockSnapshot(doc: Y.Doc): ElementLockSnapshot {
  const yLocks = getYLocks(doc)
  const snapshot = new Map<string, ElementLock>()
  yLocks.forEach((lock, id) => {
    if (lock?.identity) snapshot.set(id, lock)
  })
  return snapshot
}

export function acquireElementLocks(
  doc: Y.Doc,
  identity: string,
  username: string,
  elementIds: Iterable<string>,
): void {
  const yLocks = getYLocks(doc)
  const ts = Date.now()
  doc.transact(() => {
    for (const id of elementIds) {
      if (!id) continue
      yLocks.set(id, { identity, username, ts })
    }
  }, WHITEBOARD_LOCKS_ORIGIN)
}

export function releaseElementLocks(doc: Y.Doc, identity: string, elementIds: Iterable<string>): void {
  const yLocks = getYLocks(doc)
  doc.transact(() => {
    for (const id of elementIds) {
      if (!id) continue
      const current = yLocks.get(id)
      if (current?.identity === identity) yLocks.delete(id)
    }
  }, WHITEBOARD_LOCKS_ORIGIN)
}

export function releaseAllLocksForIdentity(doc: Y.Doc, identity: string): void {
  const yLocks = getYLocks(doc)
  const toDelete: string[] = []
  yLocks.forEach((lock, id) => {
    if (lock?.identity === identity) toDelete.push(id)
  })
  if (toDelete.length === 0) return
  doc.transact(() => {
    for (const id of toDelete) yLocks.delete(id)
  }, WHITEBOARD_LOCKS_ORIGIN)
}

/** True when `identity` may edit this element (unlocked or self-locked). */
export function canEditElement(elementId: string, identity: string, locks: ElementLockSnapshot): boolean {
  const lock = locks.get(elementId)
  return !lock || lock.identity === identity
}

/** Strip local edits to elements locked by other participants before Yjs write. */
export function filterElementsForLocalSync(
  elements: readonly OrderedExcalidrawElement[],
  locks: ElementLockSnapshot,
  localIdentity: string,
  yElements: Y.Map<OrderedExcalidrawElement>,
): OrderedExcalidrawElement[] {
  return elements.map((el) => {
    if (canEditElement(el.id, localIdentity, locks)) return el
    const remote = yElements.get(el.id)
    return remote ?? el
  })
}

export function mergeElementsWithLocks(
  local: readonly OrderedExcalidrawElement[],
  remote: readonly OrderedExcalidrawElement[],
  locks: ElementLockSnapshot,
  localIdentity: string,
  pickNewer: (a: OrderedExcalidrawElement, b: OrderedExcalidrawElement) => OrderedExcalidrawElement,
): OrderedExcalidrawElement[] {
  const localById = new Map(local.map((el) => [el.id, el]))
  const remoteById = new Map(remote.map((el) => [el.id, el]))
  const order: string[] = []

  for (const el of remote) {
    if (!order.includes(el.id)) order.push(el.id)
  }
  for (const el of local) {
    if (!remoteById.has(el.id) && !el.isDeleted && !order.includes(el.id)) {
      order.push(el.id)
    }
  }

  const merged: OrderedExcalidrawElement[] = []
  for (const id of order) {
    const localEl = localById.get(id)
    const remoteEl = remoteById.get(id)
    const lock = locks.get(id)

    if (lock?.identity === localIdentity && localEl) {
      merged.push(localEl)
      continue
    }
    if (lock && lock.identity !== localIdentity && remoteEl) {
      merged.push(remoteEl)
      continue
    }
    if (localEl && remoteEl) {
      merged.push(pickNewer(localEl, remoteEl))
    } else if (remoteEl) {
      merged.push(remoteEl)
    } else if (localEl && !localEl.isDeleted) {
      merged.push(localEl)
    }
  }
  return merged
}

export function selectedElementIds(appState: { selectedElementIds: Record<string, boolean> }): string[] {
  return Object.entries(appState.selectedElementIds)
    .filter(([, selected]) => selected)
    .map(([id]) => id)
}

export function heldLockElementIds(
  appState: {
    selectedElementIds: Record<string, boolean>
    editingTextElement: { id: string } | null
  },
  drawingElementIds: ReadonlySet<string>,
): string[] {
  const held = new Set<string>(selectedElementIds(appState))
  if (appState.editingTextElement?.id) held.add(appState.editingTextElement.id)
  for (const id of drawingElementIds) held.add(id)
  return [...held]
}
