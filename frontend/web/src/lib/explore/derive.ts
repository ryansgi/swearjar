import { config } from "@/lib/config/config"
import type { GlobalOptions, TimeRange } from "@/lib/domain/models"

// Keep everything as YYYY-MM-DD (server can parse; if config is ISO, slice to date)
const isoDateOnly = (iso?: string) => (iso && iso.length >= 10 ? iso.slice(0, 10) : iso || "")

const MIN_DAY = isoDateOnly(config.exploreMinDate) || "2011-02-12"
const DEFAULT_YEAR = String((config as any).exploreDefaultYear ?? new Date().getUTCFullYear())

export type PeriodKind = "year" | "month" | "day" | "custom"

const pad2 = (n: number) => (n < 10 ? `0${n}` : String(n))
const todayUTC = (): string => {
  const now = new Date()
  return `${now.getUTCFullYear()}-${pad2(now.getUTCMonth() + 1)}-${pad2(now.getUTCDate())}`
}
const lastDayOfMonth = (yyyyMM: string): string => {
  const y = Number(yyyyMM.slice(0, 4))
  const m = Number(yyyyMM.slice(5, 7))
  const d = new Date(y, m, 0).getDate()
  return `${yyyyMM}-${pad2(d)}`
}
const clampDay = (d: string, minDay: string, maxDay: string) => {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(d)) return minDay
  return d < minDay ? minDay : d > maxDay ? maxDay : d
}

export type ExploreUrlParamsLike = {
  period: PeriodKind
  date: string
  start: string
  end: string
  tz?: string
  repo?: string
  actor?: string
  lang?: string
  metric?: "intensity" | "coverage" | "rarity" | "counts"
  series?: "hits" | "offending_utterances" | "all_utterances"
}

// Derive the effective range from URL params with config-backed defaults (no URL writes here).
export function deriveRangeFromParams(p: ExploreUrlParamsLike, maxDay = todayUTC()): TimeRange {
  const minDay = MIN_DAY

  if (p.period === "year") {
    const y = p.date || DEFAULT_YEAR
    return { start: `${y}-01-01`, end: clampDay(`${y}-12-31`, minDay, maxDay) }
  }

  if (p.period === "month") {
    const ym = p.date || `${DEFAULT_YEAR}-01`
    return { start: `${ym}-01`, end: clampDay(lastDayOfMonth(ym), minDay, maxDay) }
  }

  if (p.period === "day") {
    const d = clampDay(p.date || `${DEFAULT_YEAR}-01-01`, minDay, maxDay)
    return { start: d, end: d }
  }

  // custom
  let s = clampDay(p.start || `${DEFAULT_YEAR}-01-01`, minDay, maxDay)
  let e = clampDay(p.end || `${DEFAULT_YEAR}-12-31`, minDay, maxDay)
  if (s > e) [s, e] = [e, s]
  return { start: s, end: e }
}

// Optional period/bucket guard if we need it elsewhere
export function normalizeBucket(period: PeriodKind, bucket: "day" | "week" | "hour") {
  return (period === "year" || period === "month") && bucket === "hour" ? "day" : bucket
}

const splitCSV = (s?: string) =>
  s
    ? s
        .split(",")
        .map((x) => x.trim())
        .filter(Boolean)
    : undefined

export function buildGlobalOptionsFromParams(p: ExploreUrlParamsLike): GlobalOptions {
  const range = deriveRangeFromParams(p)
  return {
    range,
    interval: "auto",
    tz: p.tz || "UTC",
    metric: p.metric || "counts",
    series: p.series || "hits",
    normalize: p.metric && p.metric !== "counts" ? "per_utterance" : "none",
    repo_hids: splitCSV(p.repo),
    actor_hids: splitCSV(p.actor),
    nl_langs: splitCSV(p.lang),
  }
}

export const ExploreDateBounds = {
  MIN_DAY,
  MAX_DAY_UTC: todayUTC, // call to get "today" each time
  DEFAULT_YEAR,
}
