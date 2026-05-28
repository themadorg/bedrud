import { Check, ChevronDown } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'

interface FacetOption {
  label: string
  value: string
}

interface DataTableFacetedFilterProps {
  label: string
  options: FacetOption[]
  values: string[]
  onChange: (values: string[]) => void
}

export function DataTableFacetedFilter({ label, options, values, onChange }: DataTableFacetedFilterProps) {
  const selectedValues = new Set(values)

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" size="sm" className="h-8 gap-1 text-xs font-normal justify-between w-full sm:w-auto">
          <span>{label}</span>
          <span className="flex items-center gap-1">
            {selectedValues?.size > 0 && (
              <Badge variant="secondary" className="h-4 px-1 text-[10px] font-normal">
                {selectedValues.size}
              </Badge>
            )}
            <ChevronDown className="h-3 w-3 opacity-50" />
          </span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[200px] p-0" align="start">
        <Command>
          <CommandInput placeholder={label} />
          <CommandList>
            <CommandEmpty>No results found.</CommandEmpty>
            <CommandGroup>
              {options.map((option) => {
                const isSelected = selectedValues.has(option.value)
                return (
                  <CommandItem
                    key={option.value}
                    onSelect={() => {
                      if (isSelected) {
                        selectedValues.delete(option.value)
                      } else {
                        selectedValues.add(option.value)
                      }
                      onChange(Array.from(selectedValues))
                    }}
                  >
                    <div
                      className={cn(
                        'mr-2 flex h-4 w-4 items-center justify-center border',
                        isSelected
                          ? 'bg-primary text-primary-foreground border-primary'
                          : 'opacity-50 [&_svg]:invisible',
                      )}
                    >
                      <Check className="h-3 w-3" />
                    </div>
                    <span className="text-xs">{option.label}</span>
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
