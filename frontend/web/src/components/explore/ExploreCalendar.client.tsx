"use client"

import dynamic from "next/dynamic"
import * as React from "react"
import { buildNivoTheme, readColorVar, hslWithAlpha } from "@/lib/chartTheme"
import { getVariantColors, deriveMinMax, type Variant } from "@/lib/chartScales"
import { useMedia } from "@/lib/useMedia"
import type { CalendarLegendProps } from "@nivo/calendar"
import type { TimeseriesPoint } from "@/lib/domain/models"

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

// Lazy-load Nivo Calendar (Canvas variant for perf)
const ResponsiveCalendarCanvas = dynamic(
  () => import("@nivo/calendar").then((m) => m.ResponsiveCalendarCanvas),
  { ssr: false },
)

function buildYearOptions(start = 2011) {
  const now = new Date().getFullYear()
  const years: number[] = []
  for (let y = now; y >= start; y--) years.push(y)
  return years
}

export type ExploreCalendarProps = {
  year?: number
  onYearChange?: (y: number) => void
  variant?: Variant
  onVariantChange?: (v: Variant) => void
  initialYear?: number
  initialVariant?: Variant
  scale?: { steps?: number; min?: number; max?: number; reverse?: boolean }
  /** Real data from the API (camelCase domain model) */
  points?: TimeseriesPoint[]
}

/** Map variant -> which field to visualize in the calendar */
function valueForVariant(p: TimeseriesPoint, variant: Variant): number {
  // 'commit-crimes' = total hits; 'utterances' = all events/utterances
  return variant === "utterances" ? (p.allUtterances ?? 0) : (p.hits ?? 0)
}

export default function ExploreCalendar({
  year: yearProp,
  onYearChange,
  variant: variantProp,
  onVariantChange,
  initialYear = new Date().getFullYear(),
  initialVariant = "commit-crimes" as Variant,
  scale: initialScale,
  points = [],
}: ExploreCalendarProps) {
  const years = React.useMemo(() => buildYearOptions(2011), [])

  // Local state (used only when not controlled)
  const [yearState, setYearState] = React.useState(initialYear)
  const [variantState, setVariantState] = React.useState<Variant>(() => initialVariant)

  React.useEffect(() => {
    setVariantState(initialVariant)
  }, [initialVariant])

  // Effective values (controlled if props present)
  const year = yearProp ?? yearState
  const variant = (variantProp ?? variantState) as Variant

  const setYear = (y: number) => {
    onYearChange ? onYearChange(y) : setYearState(y)
  }
  const setVariant = (v: Variant) => {
    onVariantChange ? onVariantChange(v) : setVariantState(v)
  }

  // Derive Nivo calendar data from the real timeseries points for the selected year/variant
  const data = React.useMemo(() => {
    const prefix = String(year) + "-"
    return (points ?? [])
      .filter((p) => p.t.startsWith(prefix))
      .map((p) => ({
        day: p.t, // already YYYY-MM-DD
        value: valueForVariant(p, variant),
      }))
  }, [points, year, variant])

  const nivoTheme = React.useMemo(() => buildNivoTheme(), [])
  const emptyColor = React.useMemo(
    () => hslWithAlpha(readColorVar("--color-muted", "hsl(222 25% 16%)"), 0.18),
    [],
  )

  // scale & palette
  const steps = initialScale?.steps ?? 11 // 11 swatches (shadcn has 11)
  const colors = React.useMemo(
    () => getVariantColors(variant, steps, initialScale?.reverse ?? false),
    [variant, steps, initialScale?.reverse],
  )
  const { min, max } = React.useMemo(
    () => deriveMinMax(data, { min: initialScale?.min, max: initialScale?.max }),
    [data, initialScale?.min, initialScale?.max],
  )

  const from = React.useMemo(() => new Date(year, 0, 1, 0, 0, 0, 0), [year])
  const to = React.useMemo(() => new Date(year, 11, 31, 23, 59, 59, 999), [year])

  const legendTicks = React.useMemo(() => {
    const arr = Array.from({ length: 5 }, (_, i) => i)
    return arr.map((i) => Math.round(min + ((max - min) * i) / (arr.length - 1)))
  }, [min, max])

  // responsive layout
  const isMobile = useMedia("(max-width: 640px)")

  // Legends MUST be typed so anchor/direction don't widen to string
  const legends: CalendarLegendProps[] = isMobile
    ? []
    : [
        {
          anchor: "bottom-right",
          direction: "row",
          translateY: 14,
          itemCount: colors.length,
          itemWidth: 30,
          itemHeight: 10,
          itemsSpacing: 6,
          symbolSize: 9,
        },
      ]

  const calendarLayout = isMobile
    ? {
        direction: "vertical" as const,
        containerClass: "h-[560px] overflow-auto",
        margin: { top: 6, right: 10, bottom: 8, left: 26 },
        yearSpacing: 6,
        daySpacing: 1,
        monthLegendOffset: 6,
        monthLegendPosition: "after" as const,
        yearLegend: () => "",
        dayBorderWidth: 1,
      }
    : {
        direction: "horizontal" as const,
        containerClass: "h-[260px] md:h-[300px]",
        margin: { top: 8, right: 20, bottom: 26, left: 24 },
        yearSpacing: 8,
        daySpacing: 0,
        monthLegendOffset: 10,
        monthLegendPosition: "before" as const,
        yearLegend: (y: number) => String(y),
        dayBorderWidth: 1,
      }

  const handleYearChange = (e: { target: { value: string } }) => setYear(Number(e.target.value))

  return (
    <section className="card-surface p-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-2">
          <label htmlFor="year" className="text-muted-foreground text-sm">
            Year
          </label>

          <Select
            value={String(year)}
            onValueChange={(v) => handleYearChange({ target: { value: v } })}
          >
            <SelectTrigger
              aria-label="Year"
              className="border-border bg-popover text-foreground hover:bg-popover/90 focus:ring-ring data-[state=open]:ring-ring h-9 min-w-[96px] rounded-md border px-3 py-2 text-sm shadow-sm focus:ring-2 focus:outline-none data-[state=open]:ring-2"
            >
              <SelectValue />
            </SelectTrigger>

            <SelectContent
              position="popper"
              sideOffset={6}
              className="border-border bg-popover text-popover-foreground z-50 overflow-hidden rounded-md border shadow-md"
            >
              {years.map((y) => (
                <SelectItem
                  key={y}
                  value={String(y)}
                  className="data-[highlighted]:bg-muted data-[highlighted]:text-foreground data-[state=checked]:bg-primary/15 cursor-pointer select-none"
                >
                  {y}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="inline-flex overflow-hidden rounded-lg border">
          <button
            className={`px-3 py-1.5 text-sm ${
              variant === "commit-crimes"
                ? "bg-primary/20 text-foreground"
                : "text-muted-foreground hover:text-foreground"
            }`}
            onClick={() => setVariant("commit-crimes")}
            type="button"
          >
            Commit crimes
          </button>
          <button
            className={`px-3 py-1.5 text-sm ${
              variant === "utterances"
                ? "bg-primary/20 text-foreground"
                : "text-muted-foreground hover:text-foreground"
            }`}
            onClick={() => setVariant("utterances")}
            type="button"
          >
            Events
          </button>
        </div>
      </div>

      <div className={calendarLayout.containerClass}>
        <ResponsiveCalendarCanvas
          data={data}
          from={from}
          to={to}
          align="center"
          theme={nivoTheme}
          colors={colors}
          minValue={min}
          maxValue={max}
          emptyColor={emptyColor}
          dayBorderColor={readColorVar("--color-border")}
          monthBorderColor={readColorVar("--color-border")}
          margin={calendarLayout.margin}
          direction={calendarLayout.direction}
          yearSpacing={calendarLayout.yearSpacing}
          daySpacing={calendarLayout.daySpacing}
          monthLegendOffset={calendarLayout.monthLegendOffset}
          monthLegendPosition={calendarLayout.monthLegendPosition}
          yearLegend={calendarLayout.yearLegend}
          dayBorderWidth={calendarLayout.dayBorderWidth}
          legends={legends}
          pixelRatio={Math.min(2, typeof window !== "undefined" ? window.devicePixelRatio : 1)}
        />
      </div>
    </section>
  )
}
