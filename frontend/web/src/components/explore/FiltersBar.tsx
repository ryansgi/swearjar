"use client"
import { CalendarIcon, Filter } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"

export function FiltersBar() {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" className="gap-2">
          <CalendarIcon className="h-4 w-4" /> Year
        </Button>
        <Button variant="outline" size="sm" className="gap-2">
          <Filter className="h-4 w-4" /> Filters
        </Button>
      </div>
      <div className="ml-auto flex items-center gap-2">
        <Input placeholder="Search…" className="h-8 w-[220px]" />
      </div>
    </div>
  )
}
