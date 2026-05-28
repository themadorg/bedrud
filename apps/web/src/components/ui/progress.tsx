import * as React from 'react'

import { cn } from '@/lib/utils'

interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  value?: number
  indicatorClassName?: string
}

function Progress({ className, value, indicatorClassName, ...props }: ProgressProps) {
  return (
    <div className={cn('relative h-2 w-full overflow-hidden rounded-full bg-primary/20', className)} {...props}>
      <div
        className={cn('h-full w-full flex-1 transition-all', indicatorClassName ?? 'bg-primary')}
        style={{ transform: `translateX(-${100 - (value ?? 0)}%)` }}
      />
    </div>
  )
}

export { Progress }
