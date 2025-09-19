"use client"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useExploreParams } from "@/components/hooks/useExploreParams"
export function BucketToggle() {
  const { bucket, setBucket } = useExploreParams()
  return (
    <Tabs value={bucket} onValueChange={(v) => setBucket(v as any)}>
      <TabsList>
        <TabsTrigger value="day">Day</TabsTrigger>
        <TabsTrigger value="week">Week</TabsTrigger>
        <TabsTrigger value="hour">Hour</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}
