"use client"
import { Input } from "@/components/ui/input"
import { useExploreParams } from "@/components/hooks/useExploreParams"
export function TimezoneField() {
  const { tz, setTz } = useExploreParams()
  return (
    <Input
      className="h-8 w-[120px]"
      placeholder="UTC"
      value={tz}
      onChange={(e) => setTz(e.target.value)}
    />
  )
}
