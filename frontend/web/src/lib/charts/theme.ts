"use client"

export function readColorVar(name: string, fallback = ""): string {
  if (typeof window === "undefined") return fallback
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return v || fallback
}

export function hslWithAlpha(hsl: string, alpha = 1): string {
  if (!hsl) return hsl
  const i = hsl.indexOf(")")
  return i > -1 ? `${hsl.slice(0, i)} / ${alpha})` : hsl
}

export function buildNivoTheme() {
  const text = readColorVar("--color-foreground", "hsl(0 0% 98%)")
  const grid = readColorVar("--color-border", "hsl(0 0% 25%)")
  const bgPop = readColorVar("--color-popover", "hsl(0 0% 12%)")
  const fgPop = readColorVar("--color-popover-foreground", "hsl(0 0% 98%)")
  const brdr = readColorVar("--color-border", "hsl(0 0% 25%)")

  return {
    textColor: text,
    fontSize: 13,
    axis: {
      domain: { line: { stroke: grid, strokeWidth: 1 } },
      ticks: { line: { stroke: grid, strokeWidth: 1 }, text: { fill: text, fontSize: 12 } },
      legend: { text: { fill: text, fontSize: 12, fontWeight: 500 } },
    },
    legends: {
      text: { fill: text, fontSize: 12, fontWeight: 600, opacity: 0.95 },
      title: { text: { fill: text, fontSize: 12, fontWeight: 600 } },
    },
    grid: { line: { stroke: grid, strokeWidth: 1 } },
    labels: { text: { fill: text, fontSize: 12, fontWeight: 500 } },
    tooltip: {
      container: {
        background: bgPop,
        color: fgPop,
        border: `1px solid ${brdr}`,
        borderRadius: 8,
        padding: 8,
      },
    },
    crosshair: { line: { stroke: grid, strokeWidth: 1 } },
  } as const
}

export const ROSE_11 = [
  "#fff1f2",
  "#ffe4e6",
  "#fecdd3",
  "#fda4af",
  "#fb7185",
  "#f43f5e",
  "#e11d48",
  "#be123c",
  "#9f1239",
  "#881337",
  "#4c0519",
]

export const GREEN_11 = [
  "#f0fdf4",
  "#dcfce7",
  "#bbf7d0",
  "#86efac",
  "#4ade80",
  "#22c55e",
  "#16a34a",
  "#15803d",
  "#166534",
  "#14532d",
  "#052e16",
]

export type Variant = "utterances" | "commit-crimes"
export type ScaleConfig = { steps?: number; min?: number; max?: number; reverse?: boolean }

export function getVariantColors(variant: Variant, steps = 11, reverse = false): string[] {
  const base = variant === "commit-crimes" ? ROSE_11 : GREEN_11
  const picked =
    steps === base.length
      ? base
      : steps < base.length
        ? resample(base, steps)
        : upsample(base, steps)
  return reverse ? [...picked].reverse() : picked
}

function resample(arr: string[], n: number): string[] {
  if (n <= 1) return [arr[0]]
  const out: string[] = []
  for (let i = 0; i < n; i++) {
    const t = i / (n - 1)
    const idx = Math.round(t * (arr.length - 1))
    out.push(arr[idx])
  }
  return out
}
function upsample(arr: string[], n: number): string[] {
  return resample(arr, n)
}

export function deriveMinMax(
  data: { day: string; value: number }[],
  overrides?: { min?: number; max?: number },
) {
  const vals = data.map((d) => d.value)
  const min = overrides?.min ?? 0
  const naturalMax = vals.length ? Math.max(...vals) : 0
  const max = overrides?.max ?? Math.max(1, Math.ceil(naturalMax * 1.05))
  return { min, max }
}

export type HeatmapZ = "hits" | "ratio" | "offending_utterances" | "all_utterances"
export type MetricKind = "counts" | "intensity" | "coverage" | "rarity"

export type HeatmapMode = {
  z: HeatmapZ
  metric?: MetricKind
}

export const DOW_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const

// Typical caps so sparse ratio data still has contrast
export const RATIO_CAPS: Record<Exclude<MetricKind, "counts">, number> = {
  intensity: 2.0, // hits / offending_utterances
  coverage: 0.02, // offending / all
  rarity: 0.01, // hits / all
}

export function legendForHeatmap(mode: HeatmapMode): string {
  if (mode.z === "ratio") {
    switch (mode.metric) {
      case "intensity":
        return "Intensity (hits / offending utterances)"
      case "coverage":
        return "Coverage (offending / all utterances)"
      case "rarity":
        return "Rarity (hits / all utterances)"
      default:
        return "Ratio"
    }
  }
  switch (mode.z) {
    case "offending_utterances":
      return "Offending utterances per hour"
    case "all_utterances":
      return "All utterances per hour"
    default:
      return "Hits per hour"
  }
}

export function baseColorForHeatmap(mode: HeatmapMode): string {
  if (mode.z === "ratio") {
    switch (mode.metric) {
      case "intensity":
        return readColorVar("--color-chart-intensity", "hsl(268 83% 60%)")
      case "coverage":
        return readColorVar("--color-chart-coverage", "hsl(161 84% 40%)")
      case "rarity":
        return readColorVar("--color-chart-rarity", "hsl(199 89% 48%)")
      default:
        return readColorVar("--color-chart-rarity", "hsl(199 89% 48%)")
    }
  }
  switch (mode.z) {
    case "offending_utterances":
      return readColorVar("--color-chart-offending", "hsl(252 95% 60%)")
    case "all_utterances":
      return readColorVar("--color-chart-utterances", "hsl(142 71% 45%)")
    default:
      return readColorVar("--color-chart-hits", "hsl(350 86% 54%)")
  }
}

// v is one cell; maxima supplies natural maxima discovered client-side
export function makeHeatmapColorer(
  mode: HeatmapMode,
  maxima: { maxHits: number; maxUtt: number; maxOff: number; maxRatio: number },
): {
  legend: string
  colorFor: (v: { hits: number; utterances: number; offending?: number; ratio?: number }) => string
} {
  const base = baseColorForHeatmap(mode)
  const legend = legendForHeatmap(mode)

  const denom =
    mode.z === "ratio"
      ? Math.max(
          RATIO_CAPS[(mode.metric as keyof typeof RATIO_CAPS) || "rarity"] ?? 0,
          maxima.maxRatio,
        )
      : mode.z === "all_utterances"
        ? maxima.maxUtt
        : mode.z === "offending_utterances"
          ? Math.max(1, maxima.maxOff)
          : maxima.maxHits

  const valueOf = (v: { hits: number; utterances: number; offending?: number; ratio?: number }) =>
    mode.z === "ratio"
      ? (v.ratio ?? 0)
      : mode.z === "all_utterances"
        ? v.utterances
        : mode.z === "offending_utterances"
          ? (v.offending ?? v.hits)
          : v.hits

  const colorFor = (v: {
    hits: number
    utterances: number
    offending?: number
    ratio?: number
  }) => {
    const t = Math.max(0, Math.min(1, valueOf(v) / Math.max(1e-12, denom)))
    const alpha = 0.08 + t * 0.92
    return hslWithAlpha(base, alpha)
  }

  return { legend, colorFor }
}
