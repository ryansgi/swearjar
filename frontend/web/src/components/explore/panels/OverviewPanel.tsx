"use client"

import { useEffect, useMemo, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useExploreParams } from "@/components/hooks/useExploreParams"
import { buildGlobalOptionsFromParams } from "@/lib/explore/derive"
import { api } from "@/lib/api/client"
import { KPIStripRespDTOZ, intoKPIStripResp } from "@/lib/domain/codecs"
import type { KPIStripResp } from "@/lib/domain/models"
import { Flame, GitBranch, Users, PiggyBank } from "lucide-react"
import ActivityClock from "./ActivityClock"
import { useGlobalOptions } from "@/components/hooks/useGlobalOptions"

import dynamic from "next/dynamic"
const YearlyTrends = dynamic(() => import("./YearlyTrends"), { ssr: false })

import CategoryStack from "./CategoryStacks"
import LanguageBars from "./LanguageBars"

const fmtInt = new Intl.NumberFormat()
const fmt1 = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 })
const fmt2p = new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 2 })

async function fetchKPIStrip(opts: ReturnType<typeof buildGlobalOptionsFromParams>) {
  return api.decode.post("/swearjar/kpi", opts, KPIStripRespDTOZ, intoKPIStripResp)
}

function KPI({
  title,
  value,
  icon: Icon,
  accent,
  loading,
  ribbons,
}: {
  title: string
  value?: string
  icon: React.ComponentType<{ className?: string }>
  accent: string // Tailwind gradient classes after `bg-gradient-to-r`
  loading?: boolean
  ribbons?: {
    label: string
    value?: string
    color: "violet" | "emerald" | "sky" | "amber"
    title?: string
  }[]
}) {
  const chip = {
    violet: "bg-violet-500/15 text-violet-300 ring-violet-500/30",
    emerald: "bg-emerald-500/15 text-emerald-300 ring-emerald-500/30",
    sky: "bg-sky-500/15 text-sky-300 ring-sky-500/30",
    amber: "bg-amber-500/15 text-amber-300 ring-amber-500/30",
  } as const
  const dot = {
    violet: "bg-violet-400",
    emerald: "bg-emerald-400",
    sky: "bg-sky-400",
    amber: "bg-amber-400",
  } as const

  return (
    <Card className="relative overflow-hidden">
      {/* top accent */}
      <div
        className={`pointer-events-none absolute inset-x-0 top-0 h-1 bg-gradient-to-r ${accent}`}
      />
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-muted-foreground text-sm font-medium">{title}</CardTitle>
        <Icon className="text-muted-foreground h-4 w-4" />
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-8 w-24" />
        ) : (
          <div className="text-2xl font-semibold">{value}</div>
        )}
      </CardContent>

      {/* bottom-right ribbons (stacked) */}
      {!!ribbons?.length && (
        <div className="pointer-events-none absolute right-2 bottom-2 flex flex-col items-end gap-1">
          {ribbons.map((r, i) => (
            <span
              key={`${r.label}-${i}`}
              title={r.title}
              className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] ring-1 md:text-xs ${chip[r.color]}`}
            >
              <span className={`h-1.5 w-1.5 rounded-full ${dot[r.color]}`} />
              <span className="tracking-wide uppercase">{r.label}</span>
              <span className="font-semibold">{r.value ?? "—"}</span>
            </span>
          ))}
        </div>
      )}
    </Card>
  )
}

export function OverviewPanel() {
  const params = useExploreParams()

  const request = useGlobalOptions()

  const [data, setData] = useState<KPIStripResp | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const fmtUSD = new Intl.NumberFormat(undefined, { style: "currency", currency: "USD" })
  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    fetchKPIStrip(request)
      .then((resp) => !cancelled && setData(resp))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load KPIs"))
      .finally(() => !cancelled && setLoading(false))
    return () => {
      cancelled = true
    }
  }, [request])

  const dayLabel = data?.day ? new Date(data.day).toISOString().slice(0, 10) : ""

  return (
    <div className="grid grid-cols-12 gap-4">
      {/* KPI row */}
      <div className="col-span-12 grid grid-cols-2 gap-4 md:grid-cols-4">
        {loading ? (
          Array.from({ length: 4 }).map((_, i) => (
            <Card key={i}>
              <CardHeader>
                <CardTitle className="text-muted-foreground text-sm font-medium">&nbsp;</CardTitle>
              </CardHeader>
              <CardContent>
                <Skeleton className="h-8 w-24" />
              </CardContent>
            </Card>
          ))
        ) : error ? (
          <Card className="col-span-2 md:col-span-4">
            <CardHeader>
              <CardTitle className="text-destructive text-sm font-medium">Error</CardTitle>
            </CardHeader>
            <CardContent className="text-muted-foreground text-sm">{error}</CardContent>
          </Card>
        ) : (
          <>
            <KPI
              title="Swearjar"
              value={data ? fmtUSD.format((data.offending_utterances ?? 0) * 0.25) : undefined}
              icon={PiggyBank}
              accent="from-emerald-500/50 via-teal-500/30 to-emerald-600/40"
              ribbons={[
                {
                  label: "All Events",
                  value:
                    data?.all_utterances !== undefined ? fmtInt.format(data.all_utterances) : "—",
                  color: "amber",
                },
              ]}
            />

            <KPI
              title="Repositories"
              value={fmtInt.format(data!.repos)}
              icon={GitBranch}
              accent="from-amber-500/50 via-orange-500/30 to-amber-600/40"
              ribbons={[
                {
                  label: "Rarity",
                  value: data?.rarity !== undefined ? fmt2p.format(data.rarity) : "—",
                  color: "sky",
                  title: "hits / all_utterances",
                },
              ]}
            />

            <KPI
              title="Actors"
              value={fmtInt.format(data!.actors)}
              icon={Users}
              accent="from-green-500/50 via-emerald-500/30 to-teal-500/40"
            />

            <KPI
              title="Hits"
              value={fmtInt.format(data!.hits)}
              icon={Flame}
              accent="from-sky-500/50 via-cyan-500/30 to-blue-500/40"
              ribbons={[
                {
                  label: "Intensity",
                  value: data?.intensity !== undefined ? fmt1.format(data.intensity) : "—",
                  color: "violet",
                  title: "hits / offending_utterances",
                },
                {
                  label: "Coverage",
                  value: data?.coverage !== undefined ? fmt2p.format(data.coverage) : "—",
                  color: "emerald",
                  title: "offending_utterances / all_utterances",
                },
              ]}
            />
          </>
        )}
      </div>{" "}
      {/* Calendar heatmap
      <Card className="col-span-12">
        <CardHeader>
          <CardTitle className="text-base">Calendar Heatmap</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-40 w-full" />
        </CardContent>
      </Card> */}
      {/* Activity clock and trend line */}
      <Card className="col-span-12 lg:col-span-6">
        <CardHeader>
          <CardTitle className="text-base">Activity Clock</CardTitle>
        </CardHeader>
        <CardContent>
          <ActivityClock />
        </CardContent>
      </Card>
      <Card className="col-span-12 lg:col-span-6">
        <CardHeader>
          <CardTitle className="text-base">Yearly Trend</CardTitle>
        </CardHeader>
        <CardContent>
          <YearlyTrends />
        </CardContent>
      </Card>
      {/* Category stack & language bars */}
      <CategoryStack />
      <LanguageBars />
    </div>
  )
}
