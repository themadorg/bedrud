import { Search } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { Input } from '@/components/ui/input'

interface DataTableSearchProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

export function DataTableSearch({ value, onChange, placeholder = 'Search…' }: DataTableSearchProps) {
  const [local, setLocal] = useState(value)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    setLocal(value)
  }, [value])

  function handleChange(v: string) {
    setLocal(v)
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => {
      onChange(v)
    }, 300)
  }

  return (
    <div className="flex items-center gap-2 border bg-background px-3 h-8 w-full sm:w-56 focus-within:ring-2 focus-within:ring-ring">
      <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      <Input
        value={local}
        onChange={(e) => handleChange(e.target.value)}
        placeholder={placeholder}
        className="flex-1 px-0 text-xs border-none focus-visible:border-none focus-visible:ring-0"
      />
    </div>
  )
}
