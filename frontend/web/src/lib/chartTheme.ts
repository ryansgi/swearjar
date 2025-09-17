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
    fontSize: 13, // base size
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
