"use client"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useExploreParams } from "@/components/hooks/useExploreParams"
export function SeriesToggle() {
  const { metric, series, setSeries } = useExploreParams()
  if (metric !== "counts") return null
  return (
    <Tabs value={series} onValueChange={(v) => setSeries(v as any)}>
      <TabsList>
        <TabsTrigger value="hits">Hits</TabsTrigger>
        <TabsTrigger value="offending_utterances">Offending</TabsTrigger>
        <TabsTrigger value="all_utterances">All</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}
