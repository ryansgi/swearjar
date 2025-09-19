"use client"

import { useCallback, useMemo } from "react"
import { usePathname, useRouter, useSearchParams } from "next/navigation"

export type ExploreTab =
  | "overview"
  | "repositories"
  | "actors"
  | "terms"
  | "samples"
  | "explore"
  | "compare"

export type Period = "year" | "month" | "day" | "custom"
export type Bucket = "day" | "week" | "hour"
export type Metric = "counts" | "intensity" | "coverage" | "rarity"
export type Series = "hits" | "offending_utterances" | "all_utterances"
export type EventKind = "commit" | "events"

const TAB_OPTS = [
  "overview",
  "repositories",
  "actors",
  "terms",
  "samples",
  "explore",
  "compare",
] as const satisfies readonly ExploreTab[]

const PERIOD_OPTS = ["year", "month", "day", "custom"] as const satisfies readonly Period[]
const BUCKET_OPTS = ["day", "week", "hour"] as const satisfies readonly Bucket[]
const METRIC_OPTS = [
  "counts",
  "intensity",
  "coverage",
  "rarity",
] as const satisfies readonly Metric[]
const SERIES_OPTS = [
  "hits",
  "offending_utterances",
  "all_utterances",
] as const satisfies readonly Series[]
const EVENT_OPTS = ["commit", "events"] as const satisfies readonly EventKind[]

const readMulti = (q: URLSearchParams, key: string) => {
  const multi = q.getAll(key).filter(Boolean)
  const csv = (q.get(key) ?? "").split(",").filter(Boolean)
  return Array.from(new Set([...multi, ...csv])) // de-dupe, preserve compat
}
const writeMulti = (sp: URLSearchParams, key: string, values: string[]) => {
  sp.delete(key)
  for (const v of values) sp.append(key, v) // proper encoding for symbols/commas
}

function setOrDel(sp: URLSearchParams, key: string, val: string | null | undefined) {
  if (val === undefined || val === null || val === "") sp.delete(key)
  else sp.set(key, val)
}

function pickOrDefault<T extends string>(raw: string | null, opts: readonly T[], def: T): T {
  return (opts as readonly string[]).includes(raw ?? "") ? (raw as T) : def
}

export function useExploreParams() {
  const router = useRouter()
  const pathname = usePathname()
  const q = useSearchParams()

  // Read + STRICT normalize (typos fall back to defaults; no fuzzy, no synonyms)
  const params = useMemo(() => {
    const period = pickOrDefault<Period>(q.get("period"), PERIOD_OPTS, "year")

    // bucket validity depends on period (no 'hour' for year/month)
    const rawBucket = pickOrDefault<Bucket>(q.get("bucket"), BUCKET_OPTS, "day")
    const bucket: Bucket =
      (period === "year" || period === "month") && rawBucket === "hour" ? "day" : rawBucket

    const metric = pickOrDefault<Metric>(q.get("metric"), METRIC_OPTS, "counts")
    const series =
      metric === "counts" ? pickOrDefault<Series>(q.get("series"), SERIES_OPTS, "hits") : "hits"

    return {
      tab: pickOrDefault<ExploreTab>(q.get("tab"), TAB_OPTS, "overview"),
      period,
      date: q.get("date") ?? "",
      start: q.get("start") ?? "",
      end: q.get("end") ?? "",
      bucket,
      metric,
      series,
      event: pickOrDefault<EventKind>(q.get("event"), EVENT_OPTS, "commit"),
      tz: q.get("tz") ?? "UTC",
      repo: q.get("repo") ?? "",
      actor: q.get("actor") ?? "",
      lang: q.get("lang") ?? "",
      aType: q.get("aType") ?? "",
      aId: q.get("aId") ?? "",
      bType: q.get("bType") ?? "",
      bId: q.get("bId") ?? "",
    }
  }, [q])

  const isDefault = useMemo(() => {
    return (
      params.period === "year" &&
      params.bucket === "day" &&
      params.metric === "counts" &&
      params.series === "hits" &&
      params.event === "commit" &&
      params.tz === "UTC" &&
      !params.date &&
      !params.start &&
      !params.end &&
      !params.repo &&
      !params.actor &&
      !params.lang
    )
  }, [params])

  // Update helpers
  const push = useCallback(
    (next: Partial<typeof params>) => {
      const sp = new URLSearchParams(q.toString())
      Object.entries(next).forEach(([k, v]) => setOrDel(sp, k, v as any))
      router.replace(sp.size ? `${pathname}?${sp}` : pathname, { scroll: false })
    },
    [router, pathname, q],
  )

  const setters = useMemo(
    () => ({
      setTab: (v: ExploreTab) => {
        const sp = new URLSearchParams(q.toString())
        setOrDel(sp, "tab", pickOrDefault<ExploreTab>(v, TAB_OPTS, "overview"))
        router.push(sp.size ? `${pathname}?${sp}` : pathname)
      },

      setPeriod: (v: Period) =>
        push({
          period: pickOrDefault<Period>(v, PERIOD_OPTS, "year"),
          ...(v === "custom" ? { date: "" } : { start: "", end: "" }),
          ...(v === "year" || v === "month" ? { bucket: "day" } : null),
        }),

      setDate: (v: string) => push({ date: v }),
      setRange: (s: string, e: string) => push({ start: s, end: e }),

      setBucket: (v: Bucket) => {
        const clean = pickOrDefault<Bucket>(v, BUCKET_OPTS, "day")
        const safe =
          (params.period === "year" || params.period === "month") && clean === "hour"
            ? "day"
            : clean
        push({ bucket: safe })
      },

      setMetric: (v: Metric) => push({ metric: pickOrDefault<Metric>(v, METRIC_OPTS, "counts") }),

      setSeries: (v: Series) => push({ series: pickOrDefault<Series>(v, SERIES_OPTS, "hits") }),

      setEvent: (v: EventKind) =>
        push({ event: pickOrDefault<EventKind>(v, EVENT_OPTS, "commit") }),

      setTz: (v: string) => push({ tz: v }),

      addScope: (k: "repo" | "actor" | "lang", v: string) => {
        const sp = new URLSearchParams(q.toString())
        const values = readMulti(sp, k)
        if (!v || values.includes(v)) return
        values.push(v)
        writeMulti(sp, k, values)
        router.replace(sp.size ? `${pathname}?${sp}` : pathname, { scroll: false })
      },

      removeScope: (k: "repo" | "actor" | "lang", v: string) => {
        const sp = new URLSearchParams(q.toString())
        const values = readMulti(sp, k).filter((x) => x !== v)
        writeMulti(sp, k, values)
        router.replace(sp.size ? `${pathname}?${sp}` : pathname, { scroll: false })
      },

      // Clear only global-control keys; keep unrelated (e.g., tab, compare)
      resetAll: () => {
        const sp = new URLSearchParams(q.toString())
        const RESET_KEYS = [
          "period",
          "date",
          "start",
          "end",
          "bucket",
          "metric",
          "series",
          "event",
          "tz",
          "repo",
          "actor",
          "lang",
        ]
        RESET_KEYS.forEach((k) => sp.delete(k))
        router.replace(sp.size ? `${pathname}?${sp}` : pathname, { scroll: false })
      },
    }),
    [push, q, router, pathname, params.period],
  )

  return { ...params, ...setters, isDefault }
}
