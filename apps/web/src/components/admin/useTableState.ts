import { useCallback, useMemo, useState } from 'react'

export interface TableFilters {
  q: string
  visibility: string[]
  status: string[]
  capacity: string | null
  created: string | null
}

export interface UserTableFilters {
  q: string
  provider: string[]
  role: string[]
  status: string[]
  created: string | null
}

interface ActiveFilter {
  key: string
  label: string
  value: string
}

const FILTER_LABELS: Record<string, string> = {
  q: 'Search',
  visibility: 'Visibility',
  status: 'Status',
  capacity: 'Capacity',
  owner: 'Owner',
  tags: 'Tags',
  created: 'Created',
  dateFrom: 'From',
  dateTo: 'To',
  provider: 'Provider',
  role: 'Role',
  // TODO oncoming feature
  recordingType: 'Type',
  downloadStatus: 'Download',
}

export function useTableState<T extends { id: string }>(opts: {
  items: T[]
  defaultSort?: { key: string; order: 'asc' | 'desc' }
}) {
  const [filters, setFilters] = useState<Record<string, any>>({})
  const [sortKey, setSortKey] = useState(opts.defaultSort?.key ?? 'createdAt')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>(opts.defaultSort?.order ?? 'desc')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(50)

  const setFilter = useCallback((key: string, value: any) => {
    setFilters((prev) => {
      if (value === null || value === '' || (Array.isArray(value) && value.length === 0)) {
        const next = { ...prev }
        delete next[key]
        return next
      }
      return { ...prev, [key]: value }
    })
    setPage(1)
  }, [])

  const clearFilter = useCallback((key: string) => {
    setFilters((prev) => {
      const next = { ...prev }
      delete next[key]
      return next
    })
    setPage(1)
  }, [])

  const resetFilters = useCallback(() => {
    setFilters({})
    setPage(1)
  }, [])

  const toggleSort = useCallback(
    (key: string) => {
      if (sortKey === key) {
        setSortOrder((prev) => (prev === 'asc' ? 'desc' : 'asc'))
      } else {
        setSortKey(key)
        setSortOrder('asc')
      }
    },
    [sortKey],
  )

  const selectOne = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const selectPage = useCallback(() => {
    setSelectedIds((prev) => {
      if (prev.size === 0 && opts.items.length === 0) return prev
      const allSelected = opts.items.every((item) => prev.has(item.id))
      if (allSelected) {
        const next = new Set(prev)
        for (const item of opts.items) next.delete(item.id)
        return next
      }
      const next = new Set(prev)
      for (const item of opts.items) next.add(item.id)
      return next
    })
  }, [opts.items])

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set())
  }, [])

  const filtered = useMemo(() => {
    let result = opts.items

    if (filters.q) {
      const q = (filters.q as string).toLowerCase()
      result = result.filter((item: any) => {
        const name = (item.name ?? '').toLowerCase()
        const email = (item.email ?? '').toLowerCase()
        const roomName = (item.roomName ?? '').toLowerCase()
        const createdBy = (item.createdBy ?? '').toLowerCase()
        const recordingType = (item.recordingType ?? '').toLowerCase()
        return (
          name.includes(q) ||
          email.includes(q) ||
          roomName.includes(q) ||
          createdBy.includes(q) ||
          recordingType.includes(q)
        )
      })
    }

    if (filters.visibility && (filters.visibility as string[]).length > 0) {
      const visValues = filters.visibility as string[]
      result = result.filter((item: any) => {
        const isPublic = visValues.includes('public')
        const isPrivate = visValues.includes('private')
        if (isPublic && isPrivate) return true
        if (isPublic) return item.isPublic === true
        if (isPrivate) return item.isPublic === false
        return true
      })
    }

    if (filters.status && (filters.status as string[]).length > 0) {
      const statusValues = filters.status as string[]
      const hasActive = statusValues.includes('active')
      const hasSuspended = statusValues.includes('suspended')
      const hasArchived = statusValues.includes('archived')
      result = result.filter((item: any) => {
        if ('isActive' in item && typeof item.isActive === 'boolean') {
          const isArchived = !!item.deletedAt
          const matchActive = hasActive && item.isActive === true && !isArchived
          const matchSuspended = hasSuspended && item.isActive === false && !isArchived
          const matchArchived = hasArchived && isArchived
          if (hasActive && hasSuspended && hasArchived) return true
          return matchActive || matchSuspended || matchArchived
        }
        return statusValues.includes(item.status)
      })
    }

    // TODO oncoming feature
    if (filters.recordingType && (filters.recordingType as string[]).length > 0) {
      const typeValues = filters.recordingType as string[]
      result = result.filter((item: any) => typeValues.includes(item.recordingType))
    }

    // TODO oncoming feature
    if (filters.downloadStatus && (filters.downloadStatus as string[]).length > 0) {
      const dlValues = filters.downloadStatus as string[]
      result = result.filter((item: any) => dlValues.includes(item.downloadStatus))
    }

    if (filters.provider && (filters.provider as string[]).length > 0) {
      const provValues = filters.provider as string[]
      result = result.filter((item: any) => provValues.includes(item.provider))
    }

    if (filters.role && (filters.role as string[]).length > 0) {
      const roleValues = filters.role as string[]
      result = result.filter((item: any) => {
        const accesses: string[] = item.accesses ?? []
        return roleValues.some((r) => accesses.includes(r))
      })
    }

    return result
  }, [opts.items, filters])

  const sorted = useMemo(() => {
    const arr = [...filtered]
    arr.sort((a: any, b: any) => {
      let cmp = 0
      if (
        sortKey === 'name' ||
        sortKey === 'email' ||
        sortKey === 'provider' ||
        sortKey === 'mode' ||
        sortKey === 'roomName' ||
        // TODO oncoming feature
        sortKey === 'recordingType' ||
        sortKey === 'status'
      ) {
        cmp = String(a[sortKey] ?? '').localeCompare(String(b[sortKey] ?? ''))
      } else if (
        sortKey === 'maxParticipants' ||
        sortKey === 'participantsCount' ||
        // TODO oncoming feature
        sortKey === 'durationMs' ||
        // TODO oncoming feature
        sortKey === 'fileSize'
      ) {
        cmp = (a[sortKey] ?? 0) - (b[sortKey] ?? 0)
      } else if (sortKey === 'createdAt' || sortKey === 'lastActivityAt') {
        cmp = new Date(a[sortKey] ?? 0).getTime() - new Date(b[sortKey] ?? 0).getTime()
      } else if (sortKey === 'isActive') {
        cmp = a.isActive === b.isActive ? 0 : a.isActive ? -1 : 1
      } else if (sortKey === 'createdBy') {
        cmp = String(a.ownerName ?? a.createdBy ?? '').localeCompare(String(b.ownerName ?? b.createdBy ?? ''))
      }
      return sortOrder === 'asc' ? cmp : -cmp
    })
    return arr
  }, [filtered, sortKey, sortOrder])

  const total = sorted.length
  const totalPages = Math.max(1, Math.ceil(total / limit))

  const paginated = useMemo(() => {
    const start = (page - 1) * limit
    return sorted.slice(start, start + limit)
  }, [sorted, page, limit])

  const isAllSelected = paginated.length > 0 && paginated.every((item) => selectedIds.has(item.id))
  const isIndeterminate = paginated.some((item) => selectedIds.has(item.id)) && !isAllSelected

  const activeFilterKeys = useMemo<ActiveFilter[]>(() => {
    const result: ActiveFilter[] = []
    for (const [key, value] of Object.entries(filters)) {
      if (key === 'q') continue
      if (Array.isArray(value)) {
        for (const v of value) {
          result.push({ key, label: FILTER_LABELS[key] ?? key, value: v })
        }
      } else if (value !== null && value !== '') {
        result.push({ key, label: FILTER_LABELS[key] ?? key, value: String(value) })
      }
    }
    return result
  }, [filters])

  const hasActiveFilters = activeFilterKeys.length > 0

  return {
    filters,
    setFilter,
    clearFilter,
    resetFilters,
    activeFilterKeys,
    hasActiveFilters,
    sortKey,
    sortOrder,
    toggleSort,
    selectedIds,
    selectOne,
    selectPage,
    clearSelection,
    isAllSelected,
    isIndeterminate,
    page,
    limit,
    setPage: (n: number) => {
      setSelectedIds(new Set())
      setPage(n)
    },
    setLimit: (n: number) => {
      setSelectedIds(new Set())
      setLimit(n)
      setPage(1)
    },
    total,
    totalPages,
    filtered,
    paginated,
  }
}
