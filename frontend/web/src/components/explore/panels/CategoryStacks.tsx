"use client"

import { useEffect, useMemo, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useExploreParams } from "@/components/hooks/useExploreParams"
import { useGlobalOptions } from "@/components/hooks/useGlobalOptions"
import { api } from "@/lib/api/client"
import { CategoriesStackRespDTOZ } from "@/lib/domain/codecs"
import { intoCategoriesStackResp } from "@/lib/domain/codecs"
import type { CategoriesStackResp, CategoryStackItem } from "@/lib/domain/models"
import { readColorVar, hslWithAlpha } from "@/lib/charts/theme"

async function fetchCategoriesStack(opts: ReturnType<typeof useGlobalOptions>) {
  const body = {
    ...opts,
    top_n: 7,
    sort_by: "hits" as const,
    include_other: true,
    as_share: true,
    severities: ["mild", "strong", "slur_masked"] as const,
  }
  return api.decode.post(
    "/swearjar/stacked/categories",
    body,
    CategoriesStackRespDTOZ,
    intoCategoriesStackResp,
  )
}

const SEVERITY_COLOR_VARS: Record<string, string> = {
  mild: readColorVar("--color-severity-mild", "hsl(32 95% 60%)"),
  strong: readColorVar("--color-severity-strong", "hsl(0 86% 54%)"),
  slur_masked: readColorVar("--color-severity-slur", "hsl(280 83% 67%)"),
}

function totalOf(row: CategoryStackItem) {
  return row?.total ?? (row?.mild || 0) + (row?.strong || 0) + (row?.slur_masked || 0)
}

function formatPct(v: number) {
  return (isFinite(v) ? v : 0).toLocaleString(undefined, {
    style: "percent",
    maximumFractionDigits: 1,
  })
}

function formatInt(v: number) {
  return (v || 0).toLocaleString()
}

type BarMode = "counts" | "share"

export default function CategoryStacks() {
  const params = useExploreParams()
  const request = useGlobalOptions({ normalizeFromMetric: true })

  const [data, setData] = useState<CategoriesStackResp | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchCategoriesStack(request)
      .then((resp) => !cancelled && setData(resp))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load category stacks"))
      .finally(() => !cancelled && setLoading(false))
    return () => {
      cancelled = true
    }
  }, [request])

  if (loading) {
    return (
      <Card className="col-span-12 lg:col-span-7">
        <CardHeader>
          <CardTitle className="text-base">Category Stack</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-64 w-full" />
        </CardContent>
      </Card>
    )
  }

  if (error) {
    return (
      <Card className="col-span-12 lg:col-span-7">
        <CardHeader>
          <CardTitle className="text-base">Category Stack</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-destructive text-sm">{error}</div>
        </CardContent>
      </Card>
    )
  }

  if (!data) return null

  const severityOrder = (
    data.severity_keys?.length ? data.severity_keys : (["mild", "strong", "slur_masked"] as const)
  ).slice()

  const rows: CategoryStackItem[] = data.stack ?? []
  const grandTotal = Math.max(1, data.total_hits || rows.reduce((s, r) => s + totalOf(r), 0))

  const barMode: BarMode = params.metric === "counts" ? "counts" : "share"

  return (
    <Card className="col-span-12 lg:col-span-7">
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base">Category Stack</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-2">
          <div className="mb-1 flex flex-wrap items-center gap-3">
            {severityOrder.map((sev) => (
              <div key={`legend-${sev}`} className="flex items-center gap-2">
                <span
                  className="inline-block h-3 w-3 rounded-sm"
                  style={{ backgroundColor: hslWithAlpha(SEVERITY_COLOR_VARS[sev] ?? "", 0.9) }}
                />
                <span className="text-muted-foreground text-xs">
                  {sev === "slur_masked" ? "slur (masked)" : sev}
                </span>
              </div>
            ))}
            <span className="text-muted-foreground ml-auto text-xs">
              {barMode === "counts" ? "Absolute counts" : "Share of total"}
            </span>
          </div>

          {/* bars */}
          <div className="flex flex-col gap-2">
            {rows.map((row) => {
              const total = Math.max(0, totalOf(row))
              // shares per row (for 100% bars). If server sent shares, use those; else compute
              const shares =
                row.shares ??
                (row.total
                  ? {
                      mild: (row.mild || 0) / row.total,
                      strong: (row.strong || 0) / row.total,
                      slur_masked: (row.slur_masked || 0) / row.total,
                    }
                  : { mild: 0, strong: 0, slur_masked: 0 })

              // width base depends on mode: relative to grand total (counts) or 100% (shares)
              const base = barMode === "counts" ? Math.max(0, total / grandTotal) : 1

              return (
                <div key={row.key} className="grid grid-cols-12 items-center gap-2">
                  {/* label */}
                  <div className="col-span-3 truncate text-sm" title={row.label || row.key}>
                    {row.label || row.key}
                  </div>

                  {/* bar */}
                  <div className="col-span-7">
                    <div className="bg-border/40 relative h-5 w-full overflow-hidden rounded">
                      <div
                        className="absolute inset-y-0 left-0"
                        style={{ width: `${base * 100}%` }}
                      >
                        <div className="flex h-full w-full">
                          {severityOrder.map((sev) => {
                            const count = (row as any)[sev] as number
                            const share = (shares as any)[sev] as number
                            const frac = barMode === "counts" ? (total ? count / total : 0) : share
                            return (
                              <div
                                key={`${row.key}-${sev}`}
                                className="h-full"
                                style={{
                                  width: `${Math.max(0, Math.min(1, frac)) * 100}%`,
                                  backgroundColor: hslWithAlpha(
                                    SEVERITY_COLOR_VARS[sev] ?? "",
                                    0.85,
                                  ),
                                }}
                                title={
                                  barMode === "counts"
                                    ? `${row.label}: ${sev} – ${formatInt(count)} (${formatPct(
                                        share,
                                      )})`
                                    : `${row.label}: ${sev} – ${formatPct(share)} (${formatInt(
                                        count,
                                      )})`
                                }
                              />
                            )
                          })}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="col-span-2 text-right text-sm tabular-nums">
                    {barMode === "counts" ? formatInt(total) : formatPct(total / grandTotal)}
                  </div>
                </div>
              )
            })}
          </div>

          <div className="mt-3 flex items-center justify-between">
            <div className="text-muted-foreground text-xs">
              Sorted by <span className="font-medium">{data.sorted_by ?? "hits"}</span>
            </div>
            <div className="text-muted-foreground text-xs tabular-nums">
              Total hits: <span className="font-medium">{formatInt(data.total_hits || 0)}</span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
