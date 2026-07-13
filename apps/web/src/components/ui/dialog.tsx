import * as DialogPrimitive from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import * as React from 'react'

import { cn } from '@/lib/utils'

const Dialog = DialogPrimitive.Root

const DialogTrigger = DialogPrimitive.Trigger

const DialogPortal = DialogPrimitive.Portal

const DialogClose = DialogPrimitive.Close

/**
 * iOS Safari: size against visual viewport CSS vars, not layout 100vh/100vw.
 * See `#/lib/visual-viewport`.
 */
const DialogOverlay = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Overlay
    ref={ref}
    className={cn(
      'fixed z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
      'top-[var(--app-offset-top,0px)] left-[var(--app-offset-left,0px)]',
      'h-[var(--app-height,100svh)] w-[var(--app-width,100svw)]',
      className,
    )}
    {...props}
  />
))
DialogOverlay.displayName = DialogPrimitive.Overlay.displayName

const DialogContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & {
    /** Stack above expanded WebXDC (z-200) and similar overlays. */
    elevated?: boolean
  }
>(({ className, children, elevated = false, ...props }, ref) => (
  <DialogPortal>
    <DialogOverlay className={elevated ? 'z-[219]' : undefined} />
    <DialogPrimitive.Content
      ref={ref}
      className={cn(
        // Center inside the *visual* viewport (toolbar-safe on iPhone Safari).
        'fixed z-50 grid gap-4 border bg-background p-6 shadow-lg duration-200',
        elevated && 'z-[220]',
        'left-[calc(var(--app-offset-left,0px)+var(--app-width,100svw)/2)]',
        'top-[calc(var(--app-offset-top,0px)+var(--app-height,100svh)/2)]',
        '-translate-x-1/2 -translate-y-1/2',
        // Width: never use bare 100% / 100vw of the layout viewport.
        'w-[min(32rem,calc(var(--app-width,100svw)-2rem))] max-w-[calc(var(--app-width,100svw)-2rem)]',
        'max-h-[calc(var(--app-height,100svh)-1.5rem)] overflow-y-auto overflow-x-hidden',
        'data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
        className,
      )}
      {...props}
    >
      {children}
      {/* 44×44pt minimum touch target (Apple HIG) */}
      <DialogPrimitive.Close
        className={cn(
          'absolute end-2 top-2 z-10 flex h-11 w-11 items-center justify-center opacity-70 ring-offset-background transition-opacity',
          'hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
          'disabled:pointer-events-none data-[state=open]:bg-accent data-[state=open]:text-muted-foreground',
        )}
      >
        <X className="h-4 w-4" />
        <span className="sr-only">Close</span>
      </DialogPrimitive.Close>
    </DialogPrimitive.Content>
  </DialogPortal>
))
DialogContent.displayName = DialogPrimitive.Content.displayName

const DialogHeader = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn('flex flex-col space-y-1.5 pe-10 text-center sm:text-left', className)} {...props} />
)
DialogHeader.displayName = 'DialogHeader'

const DialogFooter = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn(
      'flex flex-col-reverse sm:flex-row sm:justify-end sm:space-x-2',
      'max-sm:pb-[max(0px,env(safe-area-inset-bottom,0px))]',
      className,
    )}
    {...props}
  />
)
DialogFooter.displayName = 'DialogFooter'

const DialogTitle = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Title>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Title
    ref={ref}
    className={cn('text-lg font-semibold leading-none tracking-tight', className)}
    {...props}
  />
))
DialogTitle.displayName = DialogPrimitive.Title.displayName

const DialogDescription = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Description>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Description ref={ref} className={cn('text-sm text-muted-foreground', className)} {...props} />
))
DialogDescription.displayName = DialogPrimitive.Description.displayName

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
}
