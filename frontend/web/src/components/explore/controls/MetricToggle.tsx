import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useExploreParams } from "@/components/hooks/useExploreParams"
export function MetricToggle() {
  const { metric, setMetric } = useExploreParams()
  // Two-level control: Counts or Rates (choose which rate via secondary tabs)
  const isRates = metric !== "counts"
  return (
    <div className="flex items-center gap-2">
      <Tabs
        value={isRates ? "rates" : "counts"}
        onValueChange={(v) => setMetric(v === "counts" ? "counts" : "intensity")}
      >
        <TabsList>
          <TabsTrigger value="counts">Counts</TabsTrigger>
          <TabsTrigger value="rates">Rates</TabsTrigger>
        </TabsList>
      </Tabs>
      {isRates && (
        <Tabs value={metric} onValueChange={(v) => setMetric(v as any)}>
          <TabsList>
            <TabsTrigger value="intensity">Intensity</TabsTrigger>
            <TabsTrigger value="coverage">Coverage</TabsTrigger>
            <TabsTrigger value="rarity">Rarity</TabsTrigger>
          </TabsList>
        </Tabs>
      )}
    </div>
  )
}
