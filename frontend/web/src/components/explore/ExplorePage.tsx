"use client"
import { GlobalControls } from "@/components/explore/controls/GlobalControls"
import { ExploreTabs } from "@/components/explore/ExploreTabs"
import { OverviewPanel } from "@/components/explore/panels/OverviewPanel"
import { RepositoriesPanel } from "@/components/explore/panels/RepositoriesPanel"
import { ActorsPanel } from "@/components/explore/panels/ActorsPanel"
import { TermsPanel } from "@/components/explore/panels/TermsPanel"
import { SamplesPanel } from "@/components/explore/panels/SamplesPanel"
import { useExploreParams } from "@/components/hooks/useExploreParams"

export function ExplorePage() {
  const { tab, setTab } = useExploreParams()
  return (
    <div className="container mx-auto space-y-4 px-4 py-4">
      <h1 className="text-2xl font-semibold tracking-tight">Explore</h1>
      <GlobalControls />
      <ExploreTabs value={tab} onValueChange={setTab as any} />
      {tab === "overview" && <OverviewPanel />}
      {tab === "explore" && <RepositoriesPanel />}
      {tab === "samples" && <SamplesPanel />}
      {tab === "compare" && <ActorsPanel />}
    </div>
  )
}
