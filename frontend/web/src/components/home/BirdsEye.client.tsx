"use client"

import * as React from "react"
import ExploreCalendar from "@/components/home/ExploreCalendar.client"
import TimeseriesPanel from "@/components/home/ExplorePanel.client"
import { Card } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import type { Variant } from "@/lib/chartScales"

import { api } from "@/lib/api/client"
import {
  TimeseriesHitsRespDTOZ,
  intoTimeseriesHitsResp,
  toGlobalOptionsDTO,
} from "@/lib/domain/codecs"
import type { TimeseriesHitsResp, TimeseriesHitsInput, TimeseriesPoint } from "@/lib/domain/models"

async function fetchTimeseries(year: number): Promise<TimeseriesHitsResp> {
  return api.decode.post(
    "/swearjar/timeseries/hits",
    {
      // request model -> DTO
      ...toGlobalOptionsDTO({
        range: { start: `${year}-01-01`, end: `${year}-12-31` },
        interval: "day",
        tz: "UTC",
      } as TimeseriesHitsInput),
    },
    TimeseriesHitsRespDTOZ, // validate DTO response
    intoTimeseriesHitsResp, // scan DTO to model
  )
}

export default function BirdsEye({
  initialYear,
  initialVariant = "commit-crimes",
  className,
}: {
  initialYear?: number
  initialVariant?: Variant
  className?: string
}) {
  const [year, setYear] = React.useState(initialYear ?? 2012)
  const [variant, setVariant] = React.useState<Variant>(initialVariant)
  const [rows, setRows] = React.useState<TimeseriesPoint[] | null>(null)
  const [error, setError] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(false)

  // Load whenever year changes
  React.useEffect(() => {
    let alive = true
    setLoading(true)
    setError(null)
    fetchTimeseries(year)
      .then((resp) => {
        if (alive) setRows(resp.series)
      })
      .catch((e) => alive && setError(e?.message ?? "Failed to load data"))
      .finally(() => alive && setLoading(false))
    return () => {
      alive = false
    }
  }, [year])

  const ytd = React.useMemo(() => {
    if (!rows?.length) return null
    const sumHits = rows.reduce((acc, r) => acc + (r.hits ?? 0), 0)
    const hasIntensity = rows.some((r) => typeof r.intensity === "number")
    const avgIntensity = hasIntensity
      ? Number(
          (
            rows.reduce((acc, r) => acc + (r.intensity ?? 0), 0) /
            rows.filter((r) => typeof r.intensity === "number").length
          ).toFixed(3),
        )
      : null
    const hasCoverage = rows.some((r) => typeof r.coverage === "number")
    const maxCoverage = hasCoverage
      ? rows.reduce(
          (best, r) => (r.coverage! > best.coverage ? { t: r.t, coverage: r.coverage! } : best),
          { t: rows[0].t, coverage: rows[0].coverage ?? 0 },
        )
      : null
    return { sumHits, avgIntensity, maxCoverage }
  }, [rows])

  return (
    <section className={cn("space-y-4", className)}>
      <Card className="card-surface p-3 md:p-4">
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm">
          <div className="text-muted-foreground">YTD ({year})</div>
          <div className="font-medium">
            Hits: <span className="tabular-nums">{ytd?.sumHits ?? "-"}</span>
          </div>
          <div className="text-muted-foreground">
            Avg intensity:{" "}
            <span className="tabular-nums">
              {ytd?.avgIntensity != null ? ytd.avgIntensity : "-"}
            </span>
          </div>
          <div className="text-muted-foreground">
            Max coverage day:{" "}
            {ytd?.maxCoverage ? (
              <>
                <span className="tabular-nums">{ytd.maxCoverage.t}</span> (
                <span className="tabular-nums">{ytd.maxCoverage.coverage.toFixed(3)}</span>)
              </>
            ) : (
              "—"
            )}
          </div>
          {loading && <div className="text-muted-foreground">Loading…</div>}
          {error && <div className="text-destructive">Error: {error}</div>}
        </div>
      </Card>

      <ExploreCalendar
        year={year}
        onYearChange={setYear}
        variant={variant}
        onVariantChange={setVariant}
        initialVariant={initialVariant}
        points={rows ?? []}
      />

      <TimeseriesPanel data={rows ?? []} year={year} />
    </section>
  )
}
