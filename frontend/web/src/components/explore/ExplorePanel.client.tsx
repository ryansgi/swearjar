"use client"

import { useEffect, useMemo, useState } from "react"
import { ResponsiveLine } from "@nivo/line"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { Card } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import type { TimeseriesPoint } from "@/lib/domain/models"
import { buildNivoTheme, readColorVar } from "@/lib/chartTheme"
import { utcFormat } from "d3-time-format"

type Props = {
  data: TimeseriesPoint[]
  year: number
}

const COUNT_KEYS = ["hits", "offendingUtterances", "allUtterances"] as const
type CountKey = (typeof COUNT_KEYS)[number]
const COUNT_SERIES: { key: CountKey; label: string; colorVar: string }[] = [
  { key: "hits", label: "Hits", colorVar: "--color-chart-1" },
  { key: "offendingUtterances", label: "Offending", colorVar: "--color-chart-2" },
  { key: "allUtterances", label: "All utterances", colorVar: "--color-chart-5" },
]

const RATE_KEYS = ["intensity", "coverage", "rarity"] as const
type RateKey = (typeof RATE_KEYS)[number]
const RATE_SERIES: { key: RateKey; label: string; colorVar: string }[] = [
  { key: "intensity", label: "Intensity", colorVar: "--color-chart-3" },
  { key: "coverage", label: "Coverage", colorVar: "--color-chart-4" },
  { key: "rarity", label: "Rarity", colorVar: "--color-chart-2" },
]

type LinePoint = { x: Date; y: number | null }
type LineSerie = { id: string; color: string; data: LinePoint[] }

export default function TimeseriesPanel({ data, year }: Props) {
  const hasRates = useMemo(
    () => data.some((d) => d.intensity != null || d.coverage != null || d.rarity != null),
    [data],
  )

  const [tab, setTab] = useState<"counts" | "rates">(hasRates ? "rates" : "counts")
  const [countKeys, setCountKeys] = useState<CountKey[]>(["hits", "offendingUtterances"])
  const [rateKeys, setRateKeys] = useState<RateKey[]>(
    hasRates ? ["intensity", "coverage", "rarity"] : [],
  )

  // if a year has no rate metrics, bounce back to Counts to avoid placeholder-only view
  useEffect(() => {
    if (tab === "rates" && !hasRates) setTab("counts")
  }, [tab, hasRates])

  // Filter by selected year
  const filtered = useMemo(() => {
    const prefix = String(year) + "-"
    return data.filter((d) => d.t.startsWith(prefix))
  }, [data, year])

  // Convert "YYYY-MM-DD" to a Date at 00:00:00Z
  const toUTCDate = (isoDay: string) => new Date(`${isoDay}T00:00:00Z`)

  // Build series (x as Date)
  const countsSeries: LineSerie[] = useMemo(() => {
    return COUNT_SERIES.filter((s) => countKeys.includes(s.key)).map((s) => ({
      id: s.label,
      color: readColorVar(s.colorVar, "hsl(0 0% 50%)"),
      data: filtered.map((d) => ({ x: toUTCDate(d.t), y: d[s.key] as number })),
    }))
  }, [filtered, countKeys])

  const ratesSeries: LineSerie[] = useMemo(() => {
    if (!hasRates) return []
    return RATE_SERIES.filter(
      (s) => rateKeys.includes(s.key) && filtered.some((d) => d[s.key] != null),
    ).map((s) => ({
      id: s.label,
      color: readColorVar(s.colorVar, "hsl(0 0% 50%)"),
      data: filtered.map((d) => ({
        x: toUTCDate(d.t),
        y: (d[s.key] as number | undefined) ?? null,
      })),
    }))
  }, [filtered, rateKeys, hasRates])

  // Final dataset + guard: never pass an empty array to Nivo
  const lineData: LineSerie[] = useMemo(() => {
    const chosen = tab === "counts" ? countsSeries : ratesSeries
    if (chosen.length > 0) return chosen
    return [
      {
        id: "placeholder",
        color: "transparent",
        data: [{ x: new Date(Date.UTC(year, 0, 1)), y: 0 }],
      },
    ]
  }, [tab, countsSeries, ratesSeries, year])

  const theme = useMemo(() => buildNivoTheme(), [])

  const isCounts = tab === "counts"

  // Domain + ticks as Dates in UTC (max = Jan 1 of next year so Dec is last)
  const minDate = new Date(Date.UTC(year, 0, 1))
  const maxDate = new Date(Date.UTC(year + 1, 0, 1))
  const monthTickDates = useMemo(
    () => Array.from({ length: 12 }, (_, m) => new Date(Date.UTC(year, m, 1))),
    [year],
  )
  const fmtMonth = utcFormat("%b")

  return (
    <Card className="card-surface p-4 md:p-6">
      <div className="mb-3 flex items-center justify-between gap-4">
        <h3 className="text-muted-foreground text-sm font-medium">{year} trends</h3>
        <Tabs value={tab} onValueChange={(v) => setTab(v as any)} className="shrink-0">
          <TabsList>
            <TabsTrigger value="counts">Counts</TabsTrigger>
            {hasRates && <TabsTrigger value="rates">Rates</TabsTrigger>}
          </TabsList>
        </Tabs>
      </div>

      {/* Series toggles */}
      {isCounts ? (
        <ToggleGroup
          type="multiple"
          value={countKeys}
          onValueChange={(v) => setCountKeys((v.length ? v : ["hits"]) as CountKey[])}
          className="mb-3 flex flex-wrap gap-2"
        >
          {COUNT_SERIES.map((s) => (
            <ToggleGroupItem
              key={s.key}
              value={s.key}
              className={cn(
                "h-8 rounded-md border px-3 text-sm",
                "data-[state=on]:bg-primary/15 data-[state=on]:border-primary/40",
              )}
            >
              <span
                className="mr-2 inline-block h-2 w-2 rounded-full"
                style={{ background: `hsl(var(${s.colorVar}))` }}
              />
              {s.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      ) : (
        hasRates && (
          <ToggleGroup
            type="multiple"
            value={rateKeys}
            onValueChange={(v) => setRateKeys((v.length ? v : ["intensity"]) as RateKey[])}
            className="mb-3 flex flex-wrap gap-2"
          >
            {RATE_SERIES.map((s) => (
              <ToggleGroupItem
                key={s.key}
                value={s.key}
                className={cn(
                  "h-8 rounded-md border px-3 text-sm",
                  "data-[state=on]:bg-primary/15 data-[state=on]:border-primary/40",
                )}
              >
                <span
                  className="mr-2 inline-block h-2 w-2 rounded-full"
                  style={{ background: `hsl(var(${s.colorVar}))` }}
                />
                {s.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        )
      )}

      <div className="h-[300px] w-full">
        <ResponsiveLine
          data={lineData}
          theme={theme}
          layers={[
            "grid",
            "markers",
            "areas",
            "axes",
            "lines",
            "slices",
            "points",
            "mesh",
            "legends",
          ]}
          margin={{ top: 10, right: 18, bottom: 40, left: 48 }}
          xScale={{
            type: "time",
            useUTC: true,
            min: minDate,
            max: maxDate,
            nice: false,
          }}
          xFormat="time:%Y-%m-%d"
          axisBottom={{
            format: (v) => fmtMonth(v as Date),
            tickValues: monthTickDates,
            tickPadding: 6,
            tickSize: 0,
            legend: "",
          }}
          axisLeft={{ tickPadding: 6, tickSize: 0 }}
          enablePoints={false}
          enableArea={!isCounts}
          areaBaselineValue={0}
          areaOpacity={0.08}
          useMesh
          curve="monotoneX"
          colors={lineData.map((s) => s.color)}
          gridYValues={5}
          animate={false}
          tooltip={({ point }) => (
            <div className="border-border bg-popover text-popover-foreground rounded-md border px-2 py-1 text-xs shadow">
              <div className="opacity-70">{point.data.xFormatted}</div>
              <div className="mt-1">
                <span
                  className="mr-1 inline-block h-2 w-2 rounded-full align-middle"
                  style={{ background: point.seriesColor as string }}
                />
                {String(point.seriesId)}:{" "}
                <span className="font-medium">{point.data.yFormatted}</span>
              </div>
            </div>
          )}
        />
      </div>
    </Card>
  )
}
