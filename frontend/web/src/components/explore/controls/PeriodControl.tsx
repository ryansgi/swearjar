"use client"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useExploreParams } from "@/components/hooks/useExploreParams"
export function PeriodControl() {
  const { period, setPeriod } = useExploreParams()
  return (
    <Tabs value={period} onValueChange={(v) => setPeriod(v as any)}>
      <TabsList>
        <TabsTrigger value="year">Year</TabsTrigger>
        <TabsTrigger value="month">Month</TabsTrigger>
        <TabsTrigger value="day">Day</TabsTrigger>
        <TabsTrigger value="custom">Custom</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}
