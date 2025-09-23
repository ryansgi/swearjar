"use client"

import { useEffect, useMemo, useState } from "react"
import { Skeleton } from "@/components/ui/skeleton"
import { useExploreParams } from "@/components/hooks/useExploreParams"
import { buildGlobalOptionsFromParams } from "@/lib/explore/derive"
import { api } from "@/lib/api/client"
import { HeatmapWeeklyRespDTOZ, intoHeatmapWeeklyResp } from "@/lib/domain/codecs"
import type { HeatmapCell, HeatmapWeeklyResp } from "@/lib/domain/models"
import { useGlobalOptions } from "@/components/hooks/useGlobalOptions"
import { makeHeatmapPainter, makeScaleKey } from "@/lib/charts/heatmapScale"

const DOW = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"]

async function fetchHeatmapWeekly(opts: ReturnType<typeof buildGlobalOptionsFromParams>) {
  return api.decode.post(
    "/swearjar/heatmap/weekly",
    opts,
    HeatmapWeeklyRespDTOZ,
    intoHeatmapWeeklyResp,
  )
}

export default function ActivityClock() {
  const params = useExploreParams()
  const request = useGlobalOptions({ normalizeFromMetric: true })

  const [data, setData] = useState<HeatmapWeeklyResp | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchHeatmapWeekly(request)
      .then((resp) => !cancelled && setData(resp))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load activity clock"))
      .finally(() => !cancelled && setLoading(false))
    return () => {
      cancelled = true
    }
  }, [request])

  // Dense 7x24 for layout (even if backend ever sparsifies)
  const dense: HeatmapCell[][] = useMemo(() => {
    const grid: HeatmapCell[][] = Array.from({ length: 7 }, (_, d) =>
      Array.from(
        { length: 24 },
        (_, h) =>
          ({
            dow: d,
            hour: h,
            hits: 0,
            offending_utterances: 0,
            utterances: 0,
            ratio: 0,
          }) as HeatmapCell,
      ),
    )
    if (data) for (const c of data.grid) grid[c.dow][c.hour] = c
    return grid
  }, [data])

  // Backend announces what Z to color by. Treat anything not "ratio" as counts bucket.
  const zMode = (data?.z as "hits" | "offending_utterances" | "all_utterances" | "ratio") ?? "hits"

  // Build painter once per response + controls (unconditional)
  const { paint, legend, domain } = useMemo(
    () => makeHeatmapPainter(data?.grid ?? [], params.metric, params.series, zMode),
    [data?.grid, params.metric, params.series, zMode],
  )

  // Force remount on scale/palette changes to avoid color bleed
  const scaleKey = useMemo(
    () => makeScaleKey(data?.z, params.metric, params.series),
    [data?.z, params.metric, params.series],
  )

  if (loading) return <Skeleton className="h-64 w-full" />
  if (error) return <div className="text-destructive text-sm">{error}</div>

  // Helper to extract the numeric value that corresponds to the chosen Z
  const valueFor = (c: HeatmapCell): number => {
    switch (zMode) {
      case "ratio":
        return c.ratio ?? 0
      case "all_utterances":
        return c.utterances
      case "offending_utterances":
        return c.offending_utterances ?? 0
      default:
        return c.hits
    }
  }

  const unitForZ =
    zMode === "ratio"
      ? params.metric === "intensity"
        ? "intensity"
        : params.metric === "coverage"
          ? "coverage"
          : "rarity"
      : zMode === "offending_utterances"
        ? "offending utterances"
        : zMode === "all_utterances"
          ? "utterances"
          : "hits"

  const titleFor = (c: HeatmapCell, label: string) => {
    const hh = c.hour.toString().padStart(2, "0")
    if (zMode === "ratio") {
      return `${label} ${hh}:00 – ${(c.ratio ?? 0).toLocaleString(undefined, {
        style: "percent",
        maximumFractionDigits: 2,
      })} (${c.hits.toLocaleString()} hits)`
    }
    return `${label} ${hh}:00 – ${valueFor(c).toLocaleString()} ${unitForZ}`
  }

  const gradLeft = paint(domain.min)
  const gradRight = paint(domain.max)

  return (
    <div className="w-full" key={scaleKey}>
      <div className="grid gap-1" style={{ gridTemplateColumns: `28px repeat(24, minmax(0,1fr))` }}>
        {DOW.map((label, dow) => (
          <div key={`${label}-${scaleKey}`} className="contents">
            <div className="text-muted-foreground flex items-center justify-end pr-1 text-[10px]">
              {label}
            </div>
            {dense[dow].map((cell) => {
              const title =
                zMode === "ratio"
                  ? `${label} ${cell.hour.toString().padStart(2, "0")}:00 – ` +
                    `${(cell.ratio ?? 0).toLocaleString(undefined, {
                      style: "percent",
                      maximumFractionDigits: 2,
                    })} (${cell.hits.toLocaleString()} hits)`
                  : `${label} ${cell.hour.toString().padStart(2, "0")}:00 – ${cell.hits.toLocaleString()} hits`
              return (
                <div
                  key={`${cell.dow}-${cell.hour}-${scaleKey}`}
                  title={titleFor(cell, label)}
                  className="border-border/30 h-6 rounded-sm border transition-colors"
                  style={{ backgroundColor: paint(valueFor(cell)) }}
                />
              )
            })}
          </div>
        ))}
      </div>

      <div className="mt-3 flex items-center gap-3" key={`legend-${scaleKey}`}>
        <span className="text-muted-foreground text-xs">Low</span>
        <div
          className="h-2 flex-1 rounded-full"
          style={{
            background: `linear-gradient(90deg, ${gradLeft}, ${gradRight})`,
          }}
        />
        <span className="text-muted-foreground text-xs">High</span>
        <span className="text-muted-foreground ml-2 rounded border px-2 py-0.5 text-xs">
          {legend}
        </span>
      </div>
    </div>
  )
}
