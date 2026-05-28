import { ChevronLeft, ChevronRight } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

interface DataTablePaginationProps {
  total: number
  page: number
  limit: number
  onPageChange: (page: number) => void
  onLimitChange: (limit: number) => void
}

export function DataTablePagination({ total, page, limit, onPageChange, onLimitChange }: DataTablePaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / limit))
  const maxPage = Math.max(1, totalPages)

  return (
    <div className="flex items-center justify-between border bg-muted/30 px-4 py-3.5">
      <p className="text-[11px] text-muted-foreground">
        {total === 0 ? 'No results' : `Page ${Math.min(page, maxPage)} of ${maxPage}`}
      </p>

      <div className="flex items-center gap-2">
        <Select value={String(limit)} onValueChange={(v) => onLimitChange(+v)}>
          <SelectTrigger className="h-8 w-[72px] text-[11px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="25">25</SelectItem>
            <SelectItem value="50">50</SelectItem>
            <SelectItem value="100">100</SelectItem>
          </SelectContent>
        </Select>

        <Button
          variant="outline"
          size="icon"
          type="button"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          className="h-8 w-8"
          aria-label="Previous page"
        >
          <ChevronLeft className="h-3.5 w-3.5" />
        </Button>

        <Button
          variant="outline"
          size="icon"
          type="button"
          disabled={page >= maxPage}
          onClick={() => onPageChange(page + 1)}
          className="h-8 w-8"
          aria-label="Next page"
        >
          <ChevronRight className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  )
}
