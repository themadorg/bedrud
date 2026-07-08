import * as Y from 'yjs'
import { describe, expect, it } from 'vitest'
import {
  acquireElementLocks,
  canEditElement,
  filterElementsForLocalSync,
  mergeElementsWithLocks,
  readLockSnapshot,
  releaseAllLocksForIdentity,
  releaseElementLocks,
} from './whiteboardElementLocks'

describe('whiteboardElementLocks', () => {
  it('acquires and releases locks for an identity', () => {
    const doc = new Y.Doc()
    acquireElementLocks(doc, 'alice', 'Alice', ['el-1'])
    expect(readLockSnapshot(doc).get('el-1')).toMatchObject({ identity: 'alice' })

    releaseElementLocks(doc, 'alice', ['el-1'])
    expect(readLockSnapshot(doc).has('el-1')).toBe(false)
  })

  it('does not release locks owned by another identity', () => {
    const doc = new Y.Doc()
    acquireElementLocks(doc, 'alice', 'Alice', ['el-1'])
    releaseElementLocks(doc, 'bob', ['el-1'])
    expect(readLockSnapshot(doc).get('el-1')?.identity).toBe('alice')
  })

  it('releaseAllLocksForIdentity clears only matching locks', () => {
    const doc = new Y.Doc()
    acquireElementLocks(doc, 'alice', 'Alice', ['el-1', 'el-2'])
    acquireElementLocks(doc, 'bob', 'Bob', ['el-3'])
    releaseAllLocksForIdentity(doc, 'alice')
    const locks = readLockSnapshot(doc)
    expect(locks.has('el-1')).toBe(false)
    expect(locks.has('el-2')).toBe(false)
    expect(locks.get('el-3')?.identity).toBe('bob')
  })

  it('canEditElement respects lock ownership', () => {
    const locks = new Map([['el-1', { identity: 'alice', username: 'Alice', ts: 1 }]])
    expect(canEditElement('el-1', 'alice', locks)).toBe(true)
    expect(canEditElement('el-1', 'bob', locks)).toBe(false)
    expect(canEditElement('el-2', 'bob', locks)).toBe(true)
  })

  it('filterElementsForLocalSync keeps remote copy for foreign locks', () => {
    const doc = new Y.Doc()
    const yElements = doc.getMap<{ id: string; version: number }>('elements')
    yElements.set('el-1', { id: 'el-1', version: 1 } as never)

    const locks = new Map([['el-1', { identity: 'alice', username: 'Alice', ts: 1 }]])
    const local = [{ id: 'el-1', version: 2, isDeleted: false }] as never[]

    const filtered = filterElementsForLocalSync(local, locks, 'bob', yElements as never)
    expect(filtered[0]).toMatchObject({ version: 1 })
  })

  it('mergeElementsWithLocks prefers lock owner copies', () => {
    const locks = new Map([['el-1', { identity: 'alice', username: 'Alice', ts: 1 }]])
    const local = [{ id: 'el-1', version: 5, isDeleted: false }] as never[]
    const remote = [{ id: 'el-1', version: 2, isDeleted: false }] as never[]

    const mergedAlice = mergeElementsWithLocks(local, remote, locks, 'alice', (_, b) => b)
    expect(mergedAlice[0]).toMatchObject({ version: 5 })

    const mergedBob = mergeElementsWithLocks(local, remote, locks, 'bob', (a) => a)
    expect(mergedBob[0]).toMatchObject({ version: 2 })
  })
})