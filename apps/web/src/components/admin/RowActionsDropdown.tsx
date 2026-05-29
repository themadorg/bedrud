import { Copy, DoorOpen, Eye, Pencil, Power, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { AdminRoom } from '#/types/admin'

// Re-export for consumers that import the type from this module (back-compat)
export type { AdminRoom }

interface RowActionsDropdownProps {
  room: AdminRoom
  onView: () => void
  onEditCapacity: () => void
  onCopyId: () => void
  onSuspend: () => void
  onUnsuspend?: () => void
  onClose: () => void
  onDelete: () => void
  isReadOnly?: boolean
}

export function RowActionsDropdown({
  room,
  onView,
  onEditCapacity,
  onCopyId,
  onSuspend,
  onUnsuspend,
  onClose,
  onDelete,
  isReadOnly,
}: RowActionsDropdownProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="h-7 w-7" aria-label="Row actions">
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="currentColor"
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
          >
            <title>Row actions</title>
            <circle cx="8" cy="3" r="1.5" />
            <circle cx="8" cy="8" r="1.5" />
            <circle cx="8" cy="13" r="1.5" />
          </svg>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuItem onClick={onView}>
          <Eye className="me-2 h-3.5 w-3.5" />
          View room
        </DropdownMenuItem>
        {!isReadOnly && (
          <DropdownMenuItem onClick={onEditCapacity}>
            <Pencil className="me-2 h-3.5 w-3.5" />
            Edit capacity
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={onCopyId}>
          <Copy className="me-2 h-3.5 w-3.5" />
          Copy room ID
        </DropdownMenuItem>
        {!isReadOnly && (
          <>
            <DropdownMenuSeparator />
            {room.isActive ? (
              <DropdownMenuItem onClick={onSuspend}>
                <Power className="me-2 h-3.5 w-3.5" />
                Suspend
              </DropdownMenuItem>
            ) : (
              onUnsuspend && (
                <DropdownMenuItem onClick={onUnsuspend}>
                  <Power className="me-2 h-3.5 w-3.5" />
                  Unsuspend
                </DropdownMenuItem>
              )
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onClose} className="text-destructive focus:text-destructive">
              <DoorOpen className="me-2 h-3.5 w-3.5" />
              Close room
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onDelete} className="text-destructive focus:text-destructive">
              <Trash2 className="me-2 h-3.5 w-3.5" />
              Delete room
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
