"use client"

import { hslWithAlpha, readColorVar } from "@/lib/charts/theme"
import type { HeatmapCell } from "@/lib/domain/models"
import type { Metric, Series } from "@/components/hooks/useExploreParams"

type ZMode = "hits" | "offending_utterances" | "all_utterances" | "ratio"

const clamp01 = (x: number) => Math.max(0, Math.min(1, x))
const pct = (arr: number[], p: number) => {
  if (!arr.length) return 0
  const a = [...arr].sort((x, y) => x - y)
  const i = clamp01(p) * (a.length - 1)
  const lo = Math.floor(i),
    hi = Math.ceil(i)
  if (lo === hi) return a[lo]
  const t = i - lo
  return a[lo] * (1 - t) + a[hi] * t
}

function pickBaseColor(metric: Metric, z: ZMode) {
  // map to our CSS palette
  // hits/offending -> crimson/purple, all/coverage -> green, rarity -> cyan
  if (z === "hits") return readColorVar("--color-chart-1", "hsl(0 84% 60%)") // crimson
  if (z === "offending_utterances") return readColorVar("--color-chart-2", "hsl(280 83% 67%)") // purple
  if (z === "all_utterances") return readColorVar("--color-chart-3", "hsl(160 84% 45%)") // green

  // z === "ratio"
  switch (metric) {
    case "intensity":
      return readColorVar("--color-chart-2", "hsl(280 83% 67%)") // purple
    case "coverage":
      return readColorVar("--color-chart-3", "hsl(160 84% 45%)") // green
    case "rarity":
      return readColorVar("--color-chart-5", "hsl(190 95% 54%)") // cyan
    default:
      return readColorVar("--color-chart-1", "hsl(0 84% 60%)")
  }
}

function extractZValues(grid: HeatmapCell[], z: ZMode, metric: Metric) {
  if (z === "hits") return grid.map((c) => c.hits)
  if (z === "offending_utterances") return grid.map((c) => c.offending_utterances ?? 0)
  if (z === "all_utterances") return grid.map((c) => c.utterances)
  // z === "ratio" -> metric decides what ratio means
  return grid.map((c) => c.ratio ?? 0)
}

export function makeScaleKey(z?: string, metric?: string, series?: string) {
  return `${z ?? "hits"}|${metric ?? "counts"}|${series ?? "hits"}`
}

export function makeHeatmapPainter(grid: HeatmapCell[], metric: Metric, series: Series, z: ZMode) {
  const base = pickBaseColor(metric, z)

  // pick values for the selected Z
  const vals = extractZValues(grid, z, metric)

  // defaults
  let qLow = 0.02
  let qHigh = 0.98

  let lo = pct(vals, qLow)
  let hi = pct(vals, qHigh)

  // ensure some spread
  if (hi <= lo) hi = lo + 1

  // curve selection
  let curve = (x: number) => x

  if (z === "ratio") {
    if (metric === "intensity") {
      // anchor at 1 and boost contrast near 1
      const nz = vals.filter((v) => v > 0 && Number.isFinite(v))
      const loQ = nz.length ? pct(nz, 0.05) : 1
      const hiQ = nz.length ? pct(nz, 0.97) : 1
      lo = Math.max(1, loQ)
      hi = Math.min(2.0, Math.max(hiQ, lo * 1.1)) // soft cap + min spread
      const k = 4
      const denom = Math.log1p(k * Math.max(0, hi - 1)) || 1e-9
      curve = (x) => Math.log1p(k * Math.max(0, x - 1)) / denom
    } else {
      // coverage / rarity: very small floats -> log10
      const eps = 1e-9
      const nz = vals.filter((v) => v > 0 && Number.isFinite(v))
      const loQ = nz.length ? pct(nz, 0.05) : eps
      const hiQ = nz.length ? pct(nz, 0.97) : 1
      lo = Math.max(loQ, eps)
      hi = Math.max(hiQ, lo * 1.1)
      const y0 = Math.log10(lo),
        y1 = Math.log10(hi)
      curve = (x) => {
        const y = Math.log10(Math.max(eps, x))
        return (y - y0) / Math.max(1e-9, y1 - y0)
      }
    }
  } else {
    if (z === "offending_utterances") {
      // small counts -> log1p stretch
      const k = 1
      const span = Math.max(1e-9, hi - lo)
      curve = (x) => Math.log1p(k * Math.max(0, x - lo)) / Math.log1p(k * span)
      hi = Math.max(hi, lo + 1)
    } else {
      // hits / all_utterances: sqrt is fine
      curve = (x) => Math.sqrt((x - lo) / Math.max(1e-9, hi - lo))
    }
  }

  const legend =
    z === "ratio"
      ? metric === "intensity"
        ? "Intensity (hits / offending utterances)"
        : metric === "coverage"
          ? "Coverage (offending / all)"
          : "Rarity (hits / all)"
      : z === "hits"
        ? "Hits per hour"
        : z === "offending_utterances"
          ? "Offending utterances per hour"
          : "All utterances per hour"

  return {
    legend,
    paint: (value: number) => {
      const t = Math.max(0, Math.min(1, curve(value)))
      const alpha = 0.06 + t * 0.94
      return hslWithAlpha(base, alpha)
    },
    domain: { min: lo, max: hi },
  }
}
