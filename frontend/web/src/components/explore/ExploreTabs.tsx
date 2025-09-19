"use client"
import * as React from "react"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import type { ExploreTab } from "@/components/hooks/useExploreState"

export function ExploreTabs({
  value,
  onValueChange,
}: {
  value: ExploreTab
  onValueChange: (v: ExploreTab) => void
}) {
  return (
    <Tabs value={value} onValueChange={(v) => onValueChange(v as ExploreTab)}>
      <TabsList className="grid w-full grid-cols-5">
        <TabsTrigger value="overview">Overview</TabsTrigger>
        <TabsTrigger value="repositories">Repositories</TabsTrigger>
        <TabsTrigger value="actors">Actors</TabsTrigger>
        <TabsTrigger value="terms">Terms</TabsTrigger>
        <TabsTrigger value="samples">Samples</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}
