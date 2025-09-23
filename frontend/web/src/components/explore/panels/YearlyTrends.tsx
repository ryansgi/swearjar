"use client"

import { useMemo, useEffect, useRef, useLayoutEffect, useState } from "react"
import Link from "next/link"
import { Skeleton } from "@/components/ui/skeleton"
import { api } from "@/lib/api/client"
import { YearlyTrendsRespDTOZ, intoYearlyTrendsResp } from "@/lib/domain/codecs"
import type { YearlyTrendsResp, YearlyTrendsInput } from "@/lib/domain/models"
import { useGlobalOptions } from "@/components/hooks/useGlobalOptions"
import { hslWithAlpha, readColorVar } from "@/lib/charts/theme"
import { config } from "@/lib/config"
import { rangeFromYears } from "@/lib/charts/range"

const fmtInt = new Intl.NumberFormat()
const fmtPct2 = new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 2 })
const fmt12 = new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 })

async function fetchYearlyTrends(payload: YearlyTrendsInput) {
  return api.decode.post(
    "/swearjar/yearly/trends",
    payload,
    YearlyTrendsRespDTOZ,
    intoYearlyTrendsResp,
  )
}

function sum(a: number[]) {
  return a.reduce((s, v) => s + (Number.isFinite(v) ? v : 0), 0)
}
function safeDiv(n: number, d: number) {
  return d ? n / d : 0
}
function arrow(delta: number) {
  return delta > 0 ? "▲" : delta < 0 ? "▼" : "•"
}
function classForDelta(delta: number) {
  return delta > 0 ? "text-emerald-400" : delta < 0 ? "text-rose-400" : "text-muted-foreground"
}

/* -------------------------- responsive long spark -------------------------- */

type LineName = "hits" | "rate" | "severity"
type Line = { color: string; data: number[]; name: LineName }

function pathFromSeries(xs: (i: number) => number, y: (v: number) => number, data: number[]) {
  let d = ""
  let penDown = false
  const n = data.length
  for (let i = 0; i < n; i++) {
    const v = data[i]
    if (!Number.isFinite(v)) {
      penDown = false
      continue
    }
    d += `${penDown ? "L" : "M"} ${xs(i)},${y(v)} `
    penDown = true
  }
  return d.trim()
}

const pct = (arr: number[], p: number) => {
  if (!arr.length) return 0
  const a = [...arr].sort((x, y) => x - y)
  const i = Math.max(0, Math.min(a.length - 1, (a.length - 1) * p))
  const lo = Math.floor(i),
    hi = Math.ceil(i)
  if (lo === hi) return a[lo]
  const t = i - lo
  return a[lo] * (1 - t) + a[hi] * t
}

function LongLinesChart({
  years,
  lines,
  markers,
  height = 160,
  mode, // "yoy" | "abs"
  perSeriesScale, // only used for "abs"
}: {
  years: number[]
  lines: Line[]
  markers: { date: string; version: number }[]
  height?: number
  mode: "yoy" | "abs"
  perSeriesScale: boolean
}) {
  // --- measure container width so the SVG can fill it ---
  const containerRef = useRef<HTMLDivElement>(null)
  const [cw, setCw] = useState(0)

  useLayoutEffect(() => {
    const el = containerRef.current
    if (!el) return
    const ro = new ResizeObserver((entries) => setCw(entries[0]?.contentRect?.width || 0))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  // --- crop X domain to actual finite data (no gutters) ---
  const { firstIdx, lastIdx } = useMemo(() => {
    let first = Number.POSITIVE_INFINITY
    let last = -1
    for (const l of lines) {
      l.data.forEach((v, i) => {
        if (Number.isFinite(v)) {
          if (i < first) first = i
          if (i > last) last = i
        }
      })
    }
    if (!Number.isFinite(first) || last < 0) return { firstIdx: 0, lastIdx: 0 }
    return { firstIdx: first, lastIdx: last }
  }, [lines])

  const monthsInDomain = Math.max(1, lastIdx - firstIdx + 1)
  const pxPerMonth = 10
  const W = Math.max(cw || 0, monthsInDomain * pxPerMonth, 1)
  const H = height

  const padX = 10
  const padY = 8
  const innerW = Math.max(1, W - padX * 2)
  const innerH = Math.max(1, H - padY * 2)

  const xOf = (i: number) => padX + (innerW * (i - firstIdx)) / Math.max(1, lastIdx - firstIdx)

  // --- Y scale ---
  const { y, clamp, scaledLines } = useMemo(() => {
    // Optionally normalize each line by its own p98 (Absolute only).
    let prepared: Line[] = lines
    if (mode === "abs" && perSeriesScale) {
      prepared = lines.map((l) => {
        const vals = l.data.filter(Number.isFinite) as number[]
        const cap = Math.max(1e-9, pct(vals, 0.98))
        return { ...l, data: l.data.map((v) => (Number.isFinite(v) ? (v as number) / cap : v)) }
      })
    }

    const vals = prepared.flatMap((l) => l.data.filter(Number.isFinite)) as number[]
    let lo: number, hi: number

    if (mode === "yoy") {
      // symmetric around 0 with p98 cap
      const abs = vals.map((v) => Math.abs(v)).sort((a, b) => a - b)
      const maxAbs = abs[Math.floor(abs.length * 0.98)] ?? 0.05
      const m = Math.max(0.05, maxAbs) * 1.1
      lo = -m
      hi = m
    } else {
      // absolute levels: floor at 0; cap at p98 across visible values (or 1 if per-series)
      const cap = Math.max(1, pct(vals, 0.98))
      lo = 0
      hi = perSeriesScale ? 1.1 : Math.max(1, cap * 1.05)
    }

    const y = (v: number) => padY + innerH - ((v - lo) / Math.max(1e-9, hi - lo)) * innerH
    const clamp = (v: number) => Math.max(lo, Math.min(hi, v))
    return { y, clamp, scaledLines: prepared }
  }, [lines, innerH, mode, perSeriesScale])

  // axis markers
  const idxOf = (iso: string) => {
    const d = new Date(iso + "T00:00:00Z")
    const y0 = years[0]
    return (d.getUTCFullYear() - y0) * 12 + d.getUTCMonth()
  }

  const janTicks = useMemo(() => {
    return years
      .map((label, yi) => {
        const idx = yi * 12
        if (idx < firstIdx || idx > lastIdx) return null
        return { x: xOf(idx), label }
      })
      .filter(Boolean) as { x: number; label: number }[]
  }, [years, firstIdx, lastIdx])

  const markerColor = hslWithAlpha(readColorVar("--color-border", "hsl(0 0% 40%)"), 0.9)
  const detverXs = useMemo(
    () =>
      markers
        .map((m) => idxOf(m.date))
        .filter((i) => i >= firstIdx && i <= lastIdx)
        .map((i) => xOf(i)),
    [markers, firstIdx, lastIdx],
  )

  const zeroY = y(0)
  const paths = useMemo(
    () =>
      scaledLines.map((l) =>
        pathFromSeries(
          (i) => xOf(i),
          (v) => y(clamp(v)),
          l.data,
        ),
      ),
    [scaledLines, xOf, y, clamp],
  )

  return (
    <div ref={containerRef} className="w-full overflow-x-auto">
      <svg width={W} height={H} role="img">
        {/* baseline */}
        <line
          x1={padX}
          x2={W - padX}
          y1={zeroY}
          y2={zeroY}
          stroke={hslWithAlpha(readColorVar("--color-border", "hsl(0 0% 40%)"), 0.7)}
          strokeDasharray="3 3"
        />
        {/* years */}
        {janTicks.map(({ x, label }, i) => (
          <g key={`jan-${i}`}>
            <line
              x1={x}
              x2={x}
              y1={padY}
              y2={H - padY}
              stroke={hslWithAlpha(readColorVar("--color-border", "hsl(0 0% 35%)"), 0.35)}
            />
            <text
              x={x + 3}
              y={H - 2}
              fontSize="10"
              fill={readColorVar("--color-muted-foreground", "hsl(0 0% 70%)")}
            >
              {label}
            </text>
          </g>
        ))}
        {/* lines */}
        {paths.map((d, i) =>
          d ? (
            <path
              key={`l-${i}`}
              d={d}
              fill="none"
              stroke={scaledLines[i].color}
              strokeWidth={1.5}
            />
          ) : null,
        )}
        {/* detver markers */}
        {detverXs.map((x, i) => (
          <g key={`dv-${i}`}>
            <line
              x1={x}
              x2={x}
              y1={padY}
              y2={H - padY}
              stroke={markerColor}
              strokeDasharray="2 2"
            />
            <circle cx={x} cy={padY + 3} r={1.5} fill={markerColor} />
          </g>
        ))}
        {/* floor */}
        <line
          x1={padX}
          y1={padY + innerH + 0.5}
          x2={W - padX}
          y2={padY + innerH + 0.5}
          stroke={hslWithAlpha(readColorVar("--color-border", "hsl(0 0% 40%)"), 0.5)}
        />
      </svg>
    </div>
  )
}

/* ---------------------------------- UI bits --------------------------------- */

function KPIPill({
  label,
  value,
  deltaPct,
}: {
  label: string
  value: string
  deltaPct: number | null
}) {
  return (
    <div className="border-border/40 rounded-md border px-3 py-2">
      <div className="text-muted-foreground text-[11px]">{label}</div>
      <div className="mt-0.5 flex items-baseline gap-2">
        <div className="text-lg font-semibold">{value}</div>
        {deltaPct !== null && (
          <div className={`text-xs font-medium ${classForDelta(deltaPct)}`}>
            {arrow(deltaPct)} {fmtPct2.format(Math.abs(deltaPct))}
          </div>
        )}
      </div>
    </div>
  )
}

const CAT_COLORS: Record<string, string> = {
  generic: "hsl(0 86% 54%)",
  self_own: "hsl(280 83% 67%)",
  bot_rage: "hsl(199 89% 48%)",
  tooling_rage: "hsl(161 84% 40%)",
  lang_rage: "hsl(32 95% 60%)",
}

/* ----------------------------------- main ----------------------------------- */

export default function YearlyTrends() {
  const scope = useGlobalOptions()

  const [data, setData] = useState<YearlyTrendsResp | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // view controls
  const [mode, setMode] = useState<"yoy" | "abs">("yoy")
  const [perSeriesScale, setPerSeriesScale] = useState(false) // Absolute mode helper
  const [visible, setVisible] = useState<Record<LineName, boolean>>({
    hits: true,
    rate: true,
    severity: true,
  })

  const { minYear, maxYear } = useMemo(() => {
    const min = new Date(config.exploreMinDate).getUTCFullYear()
    const max = Number(config.exploreDefaultYear) || new Date().getUTCFullYear()
    return { minYear: min, maxYear: max }
  }, [])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    const { repo_hids, actor_hids, nl_langs, code_langs, detver, tz, lang_reliable } = scope as any
    const yr = { min: minYear, max: maxYear }
    const rg = rangeFromYears(yr.min, yr.max)

    const payload: YearlyTrendsInput = {
      repo_hids,
      actor_hids,
      nl_langs,
      code_langs,
      detver,
      tz,
      lang_reliable,
      range: rg,
      interval: "month",
      year_range: yr,
      include: ["hits", "rate", "severity", "seasonality", "mix", "detver_markers"],
    }

    fetchYearlyTrends(payload)
      .then((resp) => !cancelled && setData(resp))
      .catch((e) => !cancelled && setError(e?.message ?? "Failed to load yearly trends"))
      .finally(() => !cancelled && setLoading(false))

    return () => {
      cancelled = true
    }
  }, [JSON.stringify(scope), minYear, maxYear])

  // derive everything up-front so hook order never changes
  const years = data?.years ?? []
  const hitsByYear = data?.monthly.hits ?? {}
  const rateByYear = data?.monthly.rate ?? {}
  const sevByYear = data?.monthly.severity ?? {}
  const markers = data?.detver_markers ?? []
  const mix = data?.mix

  // Colors
  const colHits = readColorVar("--color-chart-hits", "hsl(350 86% 54%)")
  const colRate = readColorVar("--color-chart-rarity", "hsl(199 89% 48%)")
  const colSev = readColorVar("--color-chart-intensity", "hsl(268 83% 60%)")

  const yearsSorted = useMemo(() => [...(data?.years ?? [])].sort((a, b) => a - b), [data?.years])

  // YoY long arrays (12 * Y points); baseline year -> NaN to break the line
  const yoyLong = useMemo(() => {
    const outHits: number[] = []
    const outRate: number[] = []
    const outSev: number[] = []

    const get = (obj: Record<string, number[] | undefined>, y: number, m: number) =>
      obj[String(y)]?.[m]

    for (const y of yearsSorted) {
      for (let m = 0; m < 12; m++) {
        const hC = get(hitsByYear, y, m)
        const hP = get(hitsByYear, y - 1, m)
        const rC = get(rateByYear, y, m)
        const rP = get(rateByYear, y - 1, m)
        const sC = get(sevByYear, y, m)
        const sP = get(sevByYear, y - 1, m)

        const yo = (c?: number, p?: number) =>
          Number.isFinite(c) && Number.isFinite(p) && (p as number) !== 0
            ? (c as number) / (p as number) - 1
            : NaN

        outHits.push(yo(hC, hP))
        outRate.push(yo(rC, rP))
        outSev.push(yo(sC, sP))
      }
    }
    return { hits: outHits, rate: outRate, severity: outSev }
  }, [yearsSorted, hitsByYear, rateByYear, sevByYear])

  // Absolute long arrays (concatenate all years, no NaNs)
  const absLong = useMemo(() => {
    const out = { hits: [] as number[], rate: [] as number[], severity: [] as number[] }
    for (const y of yearsSorted) {
      out.hits.push(...(hitsByYear[String(y)] ?? []))
      out.rate.push(...(rateByYear[String(y)] ?? []))
      out.severity.push(...(sevByYear[String(y)] ?? []))
    }
    return out
  }, [yearsSorted, hitsByYear, rateByYear, sevByYear])

  // KPI for latest year
  const kpi = useMemo(() => {
    const yCur = years.length ? Math.max(...years) : null
    const yPrev = yCur ? yCur - 1 : null
    if (!yCur || !(String(yCur) in hitsByYear)) return null

    const Hc = hitsByYear[String(yCur)] ?? []
    const Hp = yPrev && hitsByYear[String(yPrev)] ? hitsByYear[String(yPrev)]! : []

    const Rc = rateByYear[String(yCur)] ?? []
    const Rp = yPrev && rateByYear[String(yPrev)] ? rateByYear[String(yPrev)]! : []
    const Uc = Hc.map((h, i) => safeDiv(h, Rc[i] || 0))
    const Up = Hp.map((h, i) => safeDiv(h, Rp[i] || 0))

    const hitsCur = sum(Hc)
    const hitsPrev = sum(Hp)
    const rateCur = safeDiv(sum(Hc), sum(Uc))
    const ratePrev = safeDiv(sum(Hp), sum(Up))

    const Sc = sevByYear[String(yCur)] ?? []
    const Sp = yPrev && sevByYear[String(yPrev)] ? sevByYear[String(yPrev)]! : []
    const sevCur = safeDiv(sum(Sc.map((s, i) => (Hc[i] ?? 0) * s)), sum(Hc))
    const sevPrev = safeDiv(sum(Sp.map((s, i) => (Hp[i] ?? 0) * s)), sum(Hp))

    const dHits = hitsPrev ? (hitsCur - hitsPrev) / hitsPrev : null
    const dRate = ratePrev ? (rateCur - ratePrev) / ratePrev : null
    const dSev = sevPrev ? (sevCur - sevPrev) / sevPrev : null

    return {
      labelYear: yCur,
      hits: { cur: hitsCur, delta: dHits },
      rate: { cur: rateCur, delta: dRate },
      severity: { cur: sevCur, delta: dSev },
    }
  }, [years, hitsByYear, rateByYear, sevByYear])

  if (loading) return <Skeleton className="h-64 w-full" />
  if (error) return <div className="text-destructive text-sm">{error}</div>
  if (!data) return null

  // Lines (respect visibility)
  const baseLines: Line[] =
    mode === "yoy"
      ? [
          { name: "hits", color: colHits, data: yoyLong.hits },
          { name: "rate", color: colRate, data: yoyLong.rate },
          { name: "severity", color: colSev, data: yoyLong.severity },
        ]
      : [
          { name: "hits", color: colHits, data: absLong.hits },
          { name: "rate", color: colRate, data: absLong.rate },
          { name: "severity", color: colSev, data: absLong.severity },
        ]
  const lines = baseLines.filter((l) => visible[l.name])

  const detverTag =
    (data.detver_markers?.length ?? 0) > 0
      ? `v${data.detver_markers![0]!.version} @ ${data.detver_markers![0]!.date}`
      : "no detver markers"
  const compareHref = "/explore?view=detver-compare"

  return (
    <div className="space-y-3">
      {/* KPI pills */}
      {kpi && (
        <div className="mb-1 grid grid-cols-3 gap-2">
          <KPIPill
            label={`Total hits (${kpi.labelYear})`}
            value={fmtInt.format(kpi.hits.cur)}
            deltaPct={kpi.hits.delta ?? 0}
          />
          <KPIPill
            label="Rate (hits / all)"
            value={fmtPct2.format(kpi.rate.cur)}
            deltaPct={kpi.rate.delta ?? 0}
          />
          <KPIPill
            label="Avg severity"
            value={fmt12.format(kpi.severity.cur)}
            deltaPct={kpi.severity.delta ?? 0}
          />
        </div>
      )}

      {/* Title + controls */}
      <div className="flex items-center justify-between">
        <div className="text-muted-foreground pr-1 text-xs">
          {mode === "yoy"
            ? "YoY Δ (hits, rate, severity)"
            : "Absolute levels (hits, rate, severity)"}
        </div>
        <div className="flex items-center gap-2">
          {/* series toggles */}
          {(["hits", "rate", "severity"] as LineName[]).map((k) => (
            <button
              key={k}
              onClick={() => setVisible((v) => ({ ...v, [k]: !v[k] }))}
              className={`rounded-full px-2 py-0.5 text-[11px] ring-1 transition ${
                visible[k] ? "ring-foreground/30" : "ring-border opacity-60 hover:opacity-80"
              }`}
              style={{
                background: visible[k]
                  ? hslWithAlpha(k === "hits" ? colHits : k === "rate" ? colRate : colSev, 0.12)
                  : "transparent",
              }}
              title={visible[k] ? "Click to hide" : "Click to show"}
            >
              {k[0].toUpperCase() + k.slice(1)}
            </button>
          ))}

          {/* mode toggle */}
          <div className="border-border/50 ml-2 inline-flex items-center rounded-md border p-0.5 text-xs">
            <button
              onClick={() => setMode("yoy")}
              className={`rounded px-2 py-1 ${mode === "yoy" ? "bg-muted/50" : ""}`}
            >
              YoY Δ
            </button>
            <button
              onClick={() => setMode("abs")}
              className={`rounded px-2 py-1 ${mode === "abs" ? "bg-muted/50" : ""}`}
            >
              Absolute
            </button>
          </div>

          {/* per-series scale (abs only) */}
          {mode === "abs" && (
            <button
              onClick={() => setPerSeriesScale((s) => !s)}
              className={`border-border/50 ml-2 rounded-md border px-2 py-1 text-[11px] ${
                perSeriesScale ? "bg-muted/50" : ""
              }`}
              title="Normalize each line to its own p98 so smaller series are readable"
            >
              Per-series scale
            </button>
          )}
        </div>
      </div>

      {/* Unified long chart */}
      <LongLinesChart
        years={yearsSorted}
        lines={lines}
        markers={markers}
        height={160}
        mode={mode}
        perSeriesScale={mode === "abs" && perSeriesScale}
      />

      {/* legend + note */}
      <div className="mt-2 flex flex-wrap items-center gap-4 text-[11px]">
        <span className="flex items-center gap-2">
          <i className="inline-block h-2 w-6 rounded" style={{ background: colHits }} /> Hits
        </span>
        <span className="flex items-center gap-2">
          <i className="inline-block h-2 w-6 rounded" style={{ background: colRate }} /> Rate
        </span>
        <span className="flex items-center gap-2">
          <i className="inline-block h-2 w-6 rounded" style={{ background: colSev }} /> Severity
        </span>
        <span className="text-muted-foreground ml-4">
          {mode === "yoy"
            ? "0% baseline is dashed; extreme values are p98-clamped."
            : perSeriesScale
              ? "Each line scaled by its own p98; zero baseline (if in range) is dashed."
              : "Zero baseline (if in range) is dashed; extreme values are p98-clamped."}
        </span>
      </div>

      {/* Mix snapshot (wider bars; no section title) */}
      {mix && (
        <div className="mt-2 flex flex-wrap items-center gap-6">
          {/* This year */}
          <div className="flex min-w-[260px] flex-1 items-center gap-2" title="This year mix">
            <div className="text-muted-foreground w-16 shrink-0 text-[11px]">This year</div>
            <div className="flex h-2 flex-1 overflow-hidden rounded">
              {mix.this_year.map((i) => (
                <div
                  key={i.key}
                  style={{
                    width: `${i.share * 100}%`,
                    background:
                      {
                        generic: "hsl(0 86% 54%)",
                        self_own: "hsl(280 83% 67%)",
                        bot_rage: "hsl(199 89% 48%)",
                        tooling_rage: "hsl(161 84% 40%)",
                        lang_rage: "hsl(32 95% 60%)",
                      }[i.key] ?? "hsl(0 0% 40%)",
                  }}
                  className="h-full"
                  title={`${i.key}: ${fmtPct2.format(i.share)} (${fmtInt.format(i.hits)} hits)`}
                />
              ))}
            </div>
            <div className="text-muted-foreground w-14 shrink-0 text-right text-[10px]">
              {fmtInt.format(sum(mix.this_year.map((i) => i.hits)))}
            </div>
          </div>

          {/* Last year */}
          <div className="flex min-w-[260px] flex-1 items-center gap-2" title="Last year mix">
            <div className="text-muted-foreground w-16 shrink-0 text-[11px]">Last year</div>
            <div className="flex h-2 flex-1 overflow-hidden rounded">
              {mix.last_year.map((i) => (
                <div
                  key={i.key}
                  style={{
                    width: `${i.share * 100}%`,
                    background:
                      {
                        generic: "hsl(0 86% 54%)",
                        self_own: "hsl(280 83% 67%)",
                        bot_rage: "hsl(199 89% 48%)",
                        tooling_rage: "hsl(161 84% 40%)",
                        lang_rage: "hsl(32 95% 60%)",
                      }[i.key] ?? "hsl(0 0% 40%)",
                  }}
                  className="h-full"
                  title={`${i.key}: ${fmtPct2.format(i.share)} (${fmtInt.format(i.hits)} hits)`}
                />
              ))}
            </div>
            <div className="text-muted-foreground w-14 shrink-0 text-right text-[10px]">
              {fmtInt.format(sum(mix.last_year.map((i) => i.hits)))}
            </div>
          </div>
        </div>
      )}

      <div className="text-muted-foreground mt-1 flex items-center justify-between text-[11px]">
        <span>Shifts may reflect detector changes (last marker: {detverTag}).</span>
        <Link href={compareHref} className="underline underline-offset-2">
          Compare detectors
        </Link>
      </div>
    </div>
  )
}
