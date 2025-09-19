"use client"
import { Button } from "@/components/ui/button"
import { useExploreParams } from "@/components/hooks/useExploreParams"
export function ResetButton() {
  const { resetAll, isDefault } = useExploreParams()
  return (
    <Button variant="ghost" size="sm" onClick={resetAll} disabled={isDefault}>
      Reset
    </Button>
  )
}
