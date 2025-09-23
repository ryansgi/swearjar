"use client"

import { useEffect, useMemo, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useExploreParams } from "@/components/hooks/useExploreParams"
import { useGlobalOptions } from "@/components/hooks/useGlobalOptions"
import { api } from "@/lib/api/client"
import { LangBarsRespDTOZ, intoLangBarsResp } from "@/lib/domain/codecs"
import type { LangBarsResp } from "@/lib/domain/models"
import { hslWithAlpha, readColorVar } from "@/lib/charts/theme"

async function fetchLangBars(opts: ReturnType<typeof useGlobalOptions>) {
  const body = { ...opts, page: { limit: 7 } }
  return api.decode.post("/swearjar/bars/nl-lang", body, LangBarsRespDTOZ, intoLangBarsResp)
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

export default function LanguageBars() {
  const params = useExploreParams()
  const request = useGlobalOptions({ normalizeFromMetric: true })

  const [data, setData] = useState<LangBarsResp | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchLangBars(request)
      .then((resp) => !cancelled && setData(resp))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load language bars"))
      .finally(() => !cancelled && setLoading(false))
    return () => {
      cancelled = true
    }
  }, [request])

  if (loading) {
    return (
      <Card className="col-span-12 lg:col-span-5">
        <CardHeader>
          <CardTitle className="text-base">Natural Language</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-64 w-full" />
        </CardContent>
      </Card>
    )
  }
  if (error) {
    return (
      <Card className="col-span-12 lg:col-span-5">
        <CardHeader>
          <CardTitle className="text-base">Natural Language</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-destructive text-sm">{error}</div>
        </CardContent>
      </Card>
    )
  }
  if (!data) return null

  // Colors: keep consistent with other charts
  const baseColor =
    params.metric === "counts"
      ? readColorVar("--color-chart-hits", "hsl(350 86% 54%)")
      : params.metric === "coverage"
        ? readColorVar("--color-chart-coverage", "hsl(161 84% 40%)")
        : params.metric === "rarity"
          ? readColorVar("--color-chart-rarity", "hsl(199 89% 48%)")
          : readColorVar("--color-chart-intensity", "hsl(268 83% 60%)")

  // Sorting: counts by hits; rates by ratio (fallback to hits if ratio missing)
  const items = [...(data.items ?? [])].sort((a, b) => {
    if (params.metric === "counts") return (b.hits ?? 0) - (a.hits ?? 0)
    const ra = a.ratio ?? 0
    const rb = b.ratio ?? 0
    return rb - ra || (b.hits ?? 0) - (a.hits ?? 0)
  })

  const totalHits = Math.max(1, data.total_hits || items.reduce((s, i) => s + (i.hits || 0), 0))

  return (
    <Card className="col-span-12 lg:col-span-5">
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base">Natural Language</CardTitle>
      </CardHeader>
      <CardContent>
        {!items.length ? (
          <div className="text-muted-foreground text-sm">No data for the selected scope.</div>
        ) : (
          <div className="flex flex-col gap-2">
            {items.map((row) => {
              const lang = row.lang || "—"
              const hits = row.hits || 0
              const ratio = row.ratio ?? (row.utterances ? hits / row.utterances : 0)

              // Bar width: counts → share of total hits; rates → 0..1 ratio
              const frac =
                params.metric === "counts" ? Math.max(0, hits / totalHits) : Math.max(0, ratio)

              return (
                <div key={lang} className="grid grid-cols-12 items-center gap-2">
                  <div className="col-span-3 truncate text-sm" title={lang}>
                    {lang}
                  </div>
                  <div className="col-span-7">
                    <div className="bg-border/40 h-5 w-full overflow-hidden rounded">
                      <div
                        className="h-5"
                        style={{
                          width: `${Math.min(1, frac) * 100}%`,
                          backgroundColor: hslWithAlpha(baseColor, 0.85),
                        }}
                        title={`${lang} – ${formatInt(hits)} hits${
                          params.metric === "counts"
                            ? ""
                            : ` · ${formatPct(ratio)} (${formatInt(row.utterances || 0)} utterances)`
                        }`}
                      />
                    </div>
                  </div>
                  <div className="col-span-2 text-right text-sm tabular-nums">
                    {params.metric === "counts" ? formatInt(hits) : formatPct(ratio)}
                  </div>
                </div>
              )
            })}
          </div>
        )}
        <div className="text-muted-foreground mt-3 text-xs tabular-nums">
          Total hits: <span className="font-medium">{formatInt(totalHits)}</span>
        </div>
      </CardContent>
    </Card>
  )
}
