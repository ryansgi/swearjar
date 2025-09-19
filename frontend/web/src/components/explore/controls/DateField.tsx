// src/components/explore/controls/DateField.tsx
"use client"

import { useMemo } from "react"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Calendar } from "@/components/ui/calendar"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { useExploreParams } from "@/components/hooks/useExploreParams"
import { config } from "@/lib/config/config"

const pad2 = (n: number) => (n < 10 ? `0${n}` : String(n))
const ymd = (d: Date, tz: "UTC" | "local" = "UTC") => {
  if (tz === "UTC")
    return `${d.getUTCFullYear()}-${pad2(d.getUTCMonth() + 1)}-${pad2(d.getUTCDate())}`
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}
const todayYMD = (tz: "UTC" | "local") => {
  const now = new Date()
  if (tz === "UTC") {
    return `${now.getUTCFullYear()}-${pad2(now.getUTCMonth() + 1)}-${pad2(now.getUTCDate())}`
  }
  return `${now.getFullYear()}-${pad2(now.getMonth() + 1)}-${pad2(now.getDate())}`
}
const toLocalDate = (s: string | undefined) => {
  if (!s) return undefined
  const y = Number(s.slice(0, 4))
  const m = Number(s.slice(5, 7))
  const d = Number(s.slice(8, 10) || "01")
  if (!y || !m || !d) return undefined
  return new Date(y, m - 1, d) // local midnight
}

// Hardcode min date for now (wire config later)
const MIN_DAY = config.exploreMinDate
const MIN_YEAR = parseInt(MIN_DAY.slice(0, 4), 10)

const UI_DEFAULTS = {
  year: config.exploreDefaultYear,
  month: `${config.exploreDefaultYear}-10`,
  day: `${config.exploreDefaultYear}-10-05`,
  customStart: `${config.exploreDefaultYear}-01-01`,
  customEnd: `${config.exploreDefaultYear}-12-31`,
}

// simple string clamps (safe for fixed-width YYYY[-MM[-DD]])
const clampYearStr = (y: string, minYear: number, maxYear: number) =>
  String(Math.min(Math.max(+y || minYear, minYear), maxYear))
const clampMonthStr = (m: string, minMonth: string, maxMonth: string) =>
  m < minMonth ? minMonth : m > maxMonth ? maxMonth : m
const clampDayStr = (d: string, minDay: string, maxDay: string) =>
  d < minDay ? minDay : d > maxDay ? maxDay : d

export function DateField() {
  const { period, tz = "UTC", date, start, end, setDate, setRange } = useExploreParams()
  const tzMode: "UTC" | "local" = tz === "UTC" ? "UTC" : "local"

  // bounds derived from TZ
  const maxDay = useMemo(() => todayYMD(tzMode), [tzMode])
  const maxYear = useMemo(() => parseInt(maxDay.slice(0, 4), 10), [maxDay])
  const minMonth = useMemo(() => MIN_DAY.slice(0, 7), [])
  const maxMonth = useMemo(() => maxDay.slice(0, 7), [maxDay])

  // precompute years list (top-level to keep hook order stable)
  const years = useMemo(() => {
    const list: number[] = []
    for (let y = maxYear; y >= MIN_YEAR; y--) list.push(y)
    return list
  }, [maxYear])

  const uiYear = useMemo(
    () => (date && period === "year" ? date : clampYearStr(UI_DEFAULTS.year, MIN_YEAR, maxYear)),
    [date, period, maxYear],
  )
  const uiMonth = useMemo(
    () =>
      date && period === "month"
        ? date
        : clampMonthStr(UI_DEFAULTS.month, `${MIN_YEAR}-01`, maxMonth),
    [date, period, maxMonth],
  )
  const uiDay = useMemo(
    () => (date && period === "day" ? date : clampDayStr(UI_DEFAULTS.day, MIN_DAY, maxDay)),
    [date, period, maxDay],
  )
  const uiRange = useMemo(() => {
    const s = start || UI_DEFAULTS.customStart
    const e = end || UI_DEFAULTS.customEnd
    let S = clampDayStr(s, MIN_DAY, maxDay)
    let E = clampDayStr(e, MIN_DAY, maxDay)
    if (S > E) [S, E] = [E, S]
    return { start: S, end: E }
  }, [start, end, maxDay])

  if (period === "custom") {
    const selected = {
      from: toLocalDate(uiRange.start),
      to: toLocalDate(uiRange.end),
    }
    return (
      <Popover>
        <PopoverTrigger asChild>
          <Input
            readOnly
            value={uiRange.start && uiRange.end ? `${uiRange.start} → ${uiRange.end}` : ""}
            placeholder="Range (YYYY-MM-DD → YYYY-MM-DD)"
            className="h-8 w-[320px] cursor-pointer"
          />
        </PopoverTrigger>
        <PopoverContent className="p-0" align="start">
          <Calendar
            mode="range"
            selected={selected}
            onSelect={(r) => {
              if (!r) return
              const from = r.from ? ymd(r.from, tzMode) : ""
              const to = r.to ? ymd(r.to, tzMode) : from
              if (from && to) setRange(from, to) // write to URL only on user change
            }}
            fromDate={toLocalDate(MIN_DAY)}
            toDate={toLocalDate(maxDay)}
            fromMonth={new Date(MIN_YEAR, 0, 1)}
            toMonth={
              new Date(
                parseInt(maxMonth.slice(0, 4), 10),
                parseInt(maxMonth.slice(5, 7), 10) - 1,
                1,
              )
            }
            captionLayout="dropdown"
            initialFocus
          />
        </PopoverContent>
      </Popover>
    )
  }

  if (period === "year") {
    return (
      <Select
        value={uiYear}
        onValueChange={(v) => {
          setDate(v) // write only when user actually picks
        }}
      >
        <SelectTrigger className="h-8 w-[120px]">
          <SelectValue placeholder="YYYY" />
        </SelectTrigger>
        <SelectContent className="max-h-72">
          {years.map((y) => (
            <SelectItem key={y} value={String(y)}>
              {y}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    )
  }

  const uiSingle = period === "month" ? uiMonth : uiDay
  const selectedSingle = period === "month" ? toLocalDate(`${uiSingle}-01`) : toLocalDate(uiSingle)

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Input
          readOnly
          placeholder={period === "month" ? "YYYY-MM" : "YYYY-MM-DD"}
          value={uiSingle || ""}
          className="h-8 w-[180px] cursor-pointer"
        />
      </PopoverTrigger>
      <PopoverContent className="p-0" align="start">
        <Calendar
          mode="single"
          selected={selectedSingle}
          onSelect={(d) => {
            if (!d) return
            const v = period === "month" ? ymd(d, tzMode).slice(0, 7) : ymd(d, tzMode)
            setDate(v) // write to URL only on user change
          }}
          fromDate={toLocalDate(MIN_DAY)}
          toDate={toLocalDate(maxDay)}
          fromMonth={new Date(MIN_YEAR, 0, 1)}
          toMonth={
            new Date(parseInt(maxMonth.slice(0, 4), 10), parseInt(maxMonth.slice(5, 7), 10) - 1, 1)
          }
          captionLayout="dropdown"
          initialFocus
        />
      </PopoverContent>
    </Popover>
  )
}
