"use client"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useExploreParams } from "@/components/hooks/useExploreParams"
export function EventToggle() {
  const { event, setEvent } = useExploreParams()
  return (
    <Tabs value={event} onValueChange={(v) => setEvent(v as any)}>
      <TabsList>
        <TabsTrigger value="commit">Commit crimes</TabsTrigger>
        <TabsTrigger value="events">Events</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}
