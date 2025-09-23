"use client"

import { useMemo } from "react"
import { useSearchParams } from "next/navigation"
import { useExploreParams } from "@/components/hooks/useExploreParams"
import { buildGlobalOptionsFromParams } from "@/lib/explore/derive"
import type { GlobalOptions } from "@/lib/domain/models"

type Opts = {
  // When true, set normalize="per_utterance" whenever metric !== "counts"
  normalizeFromMetric?: boolean
}

export function useGlobalOptions(opts?: Opts): GlobalOptions {
  const p = useExploreParams()
  const q = useSearchParams()

  // Canonical signature using only the keys that affect GlobalOptions (+ metric if normalize needed)
  const sig = useMemo(() => {
    const keys = [
      "period",
      "date",
      "start",
      "end",
      "tz",
      "repo",
      "actor",
      "lang",
      "metric",
      "series",
    ] as const
    const sp = new URLSearchParams()
    for (const k of keys) {
      const v = q.get(k)
      if (v) sp.set(k, v)
    }
    if (opts?.normalizeFromMetric) sp.set("metric", q.get("metric") ?? "counts")
    return sp.toString()
  }, [q, opts?.normalizeFromMetric])

  // Build a request object tied to the signature only -> stable identity across re-renders
  return useMemo(() => {
    const base = buildGlobalOptionsFromParams({
      period: p.period,
      date: p.date,
      start: p.start,
      end: p.end,
      tz: p.tz,
      repo: p.repo,
      actor: p.actor,
      lang: p.lang,
      metric: p.metric,
      series: p.series,
    })
    return opts?.normalizeFromMetric && p.metric !== "counts"
      ? { ...base, normalize: "per_utterance" as const }
      : base
  }, [sig])
}
