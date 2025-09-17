// 11-step palettes from shadcn / Tailwind colors (light to dark)
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

// Variant to default scale config (you can override per chart)
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

// If we ever want fewer/more than 11 swatches
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
  // simple nearest-neighbor upsample
  return resample(arr, n)
}

export function deriveMinMax(
  data: { day: string; value: number }[],
  overrides?: { min?: number; max?: number },
) {
  const vals = data.map((d) => d.value)
  const min = overrides?.min ?? 0
  const naturalMax = vals.length ? Math.max(...vals) : 0
  const max =
    overrides?.max ??
    // give a little space so highest cell isn't always the darkest
    Math.max(1, Math.ceil(naturalMax * 1.05))
  return { min, max }
}
