import * as SheetPrimitive from '@radix-ui/react-dialog'
import { cva, type VariantProps } from 'class-variance-authority'
import { X } from 'lucide-react'
import * as React from 'react'

import { cn } from '@/lib/utils'

const Sheet = SheetPrimitive.Root

const SheetTrigger = SheetPrimitive.Trigger

const SheetClose = SheetPrimitive.Close

const SheetPortal = SheetPrimitive.Portal

const SheetOverlay = React.forwardRef<
  React.ElementRef<typeof SheetPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof SheetPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <SheetPrimitive.Overlay
    className={cn(
      // Visual viewport (not layout 100vh/100vw) — iOS Safari toolbar-safe
      'fixed z-50 bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
      'top-[var(--app-offset-top,0px)] left-[var(--app-offset-left,0px)]',
      'h-[var(--app-height,100svh)] w-[var(--app-width,100svw)]',
      className,
    )}
    {...props}
    ref={ref}
  />
))
SheetOverlay.displayName = SheetPrimitive.Overlay.displayName

const sheetVariants = cva(
  'fixed z-50 gap-4 bg-background p-6 shadow-lg transition ease-in-out data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:duration-300 data-[state=open]:duration-500',
  {
    variants: {
      side: {
        top: 'left-[var(--app-offset-left,0px)] top-[var(--app-offset-top,0px)] w-[var(--app-width,100svw)] border-b pt-[env(safe-area-inset-top,0px)] data-[state=closed]:slide-out-to-top data-[state=open]:slide-in-from-top',
        bottom:
          // Align to visual viewport bottom (Safari bar + home indicator).
          'left-[var(--app-offset-left,0px)] top-[calc(var(--app-offset-top,0px)+var(--app-height,100svh))] w-[var(--app-width,100svw)] -translate-y-full border-t pb-[max(1.5rem,env(safe-area-inset-bottom,0px))] data-[state=closed]:slide-out-to-bottom data-[state=open]:slide-in-from-bottom',
        left: 'left-[var(--app-offset-left,0px)] top-[var(--app-offset-top,0px)] h-[var(--app-height,100svh)] w-[min(75%,var(--app-width,100svw))] max-w-sm border-r pt-[env(safe-area-inset-top,0px)] pb-[env(safe-area-inset-bottom,0px)] data-[state=closed]:slide-out-to-left data-[state=open]:slide-in-from-left',
        right:
          'left-[calc(var(--app-offset-left,0px)+var(--app-width,100svw))] top-[var(--app-offset-top,0px)] h-[var(--app-height,100svh)] w-[min(75%,var(--app-width,100svw))] max-w-sm -translate-x-full border-l pt-[env(safe-area-inset-top,0px)] pb-[env(safe-area-inset-bottom,0px)] data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right',
      },
    },
    defaultVariants: {
      side: 'right',
    },
  },
)

interface SheetContentProps
  extends React.ComponentPropsWithoutRef<typeof SheetPrimitive.Content>,
    VariantProps<typeof sheetVariants> {}

const SheetContent = React.forwardRef<React.ElementRef<typeof SheetPrimitive.Content>, SheetContentProps>(
  ({ side = 'right', className, children, ...props }, ref) => (
    <SheetPortal>
      <SheetOverlay />
      <SheetPrimitive.Content ref={ref} className={cn(sheetVariants({ side }), className)} {...props}>
        {children}
        <SheetPrimitive.Close
          className={cn(
            'absolute z-10 flex h-11 w-11 items-center justify-center opacity-70 ring-offset-background transition-opacity',
            'hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
            'disabled:pointer-events-none data-[state=open]:bg-secondary',
            'top-[max(0.5rem,env(safe-area-inset-top,0px))] end-[max(0.5rem,env(safe-area-inset-right,0px))]',
          )}
        >
          <X className="h-4 w-4" />
          <span className="sr-only">Close</span>
        </SheetPrimitive.Close>
      </SheetPrimitive.Content>
    </SheetPortal>
  ),
)
SheetContent.displayName = SheetPrimitive.Content.displayName

const SheetHeader = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={cn('flex flex-col space-y-2 text-center sm:text-left', className)} {...props} />
)
SheetHeader.displayName = 'SheetHeader'

const SheetFooter = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn(
      'flex flex-col-reverse sm:flex-row sm:justify-end sm:space-x-2',
      'pb-[env(safe-area-inset-bottom,0px)] sm:pb-0',
      className,
    )}
    {...props}
  />
)
SheetFooter.displayName = 'SheetFooter'

const SheetTitle = React.forwardRef<
  React.ElementRef<typeof SheetPrimitive.Title>,
  React.ComponentPropsWithoutRef<typeof SheetPrimitive.Title>
>(({ className, ...props }, ref) => (
  <SheetPrimitive.Title ref={ref} className={cn('text-lg font-semibold text-foreground', className)} {...props} />
))
SheetTitle.displayName = SheetPrimitive.Title.displayName

const SheetDescription = React.forwardRef<
  React.ElementRef<typeof SheetPrimitive.Description>,
  React.ComponentPropsWithoutRef<typeof SheetPrimitive.Description>
>(({ className, ...props }, ref) => (
  <SheetPrimitive.Description ref={ref} className={cn('text-sm text-muted-foreground', className)} {...props} />
))
SheetDescription.displayName = SheetPrimitive.Description.displayName

export {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetOverlay,
  SheetPortal,
  SheetTitle,
  SheetTrigger,
}
