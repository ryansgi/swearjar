import { z } from "zod"
import type * as DTO from "./dto"
import type * as M from "./models"

export const TimeseriesPointDTOZ = z.object({
  t: z.string(),
  hits: z.number(),
  offending_utterances: z.number(),
  all_utterances: z.number().optional(),
  intensity: z.number().optional(),
  coverage: z.number().optional(),
  rarity: z.number().optional(),
})
export const TimeseriesHitsRespDTOZ = z.object({
  interval: z.enum(["hour", "day", "week", "month"]),
  series: z.array(TimeseriesPointDTOZ),
})

export const HeatmapCellDTOZ = z.object({
  dow: z.number().int().min(0).max(6),
  hour: z.number().int().min(0).max(23),
  hits: z.number(),
  utterances: z.number(),
  offending_utterances: z.number().optional(),
  ratio: z.number().optional(),
})
export const HeatmapWeeklyRespDTOZ = z.object({
  z: z.enum(["hits", "ratio", "offending_utterances", "all_utterances"]),
  grid: z.array(HeatmapCellDTOZ),
})

export const LangBarItemDTOZ = z.object({
  lang: z.string(),
  hits: z.number(),
  utterances: z.number(),
  ratio: z.number().optional(),
})
export const LangBarsRespDTOZ = z.object({
  items: z.array(LangBarItemDTOZ),
  total_hits: z.number(),
})

export const KPIStripRespDTOZ = z.object({
  day: z.string(),
  hits: z.number(),
  offending_utterances: z.number(),
  repos: z.number(),
  actors: z.number(),
  all_utterances: z.number().optional(),
  intensity: z.number().optional(),
  coverage: z.number().optional(),
  rarity: z.number().optional(),
})

export const intoTimeseriesPoint = (d: DTO.TimeseriesPointDTO): M.TimeseriesPoint => d
export const intoTimeseriesHitsResp = (d: DTO.TimeseriesHitsRespDTO): M.TimeseriesHitsResp => d
export const intoHeatmapWeeklyResp = (d: DTO.HeatmapWeeklyRespDTO): M.HeatmapWeeklyResp => d
export const intoLangBarsResp = (d: DTO.LangBarsRespDTO): M.LangBarsResp => d
export const intoKPIStripResp = (d: DTO.KPIStripRespDTO): M.KPIStripResp => d

export const toGlobalOptionsDTO = (m: M.GlobalOptions): DTO.GlobalOptionsDTO => m

const MonthBandDTOZ = z.object({
  m: z.number().int().min(1).max(12),
  median: z.number(),
  p25: z.number(),
  p75: z.number(),
})
const CategoryShareDTOZ = z.object({
  key: z.string(),
  hits: z.number(),
  share: z.number(),
})
const DetverMarkerDTOZ = z.object({
  date: z.string(), // YYYY-MM-DD
  version: z.number().int(),
})

export const YearlyTrendsRespDTOZ = z.object({
  years: z.array(z.number().int()),
  monthly: z.object({
    hits: z.record(z.string(), z.array(z.number())).optional(),
    rate: z.record(z.string(), z.array(z.number())).optional(),
    severity: z.record(z.string(), z.array(z.number())).optional(),
  }),
  seasonality: z
    .object({
      hits: z.array(MonthBandDTOZ).optional(),
      rate: z.array(MonthBandDTOZ).optional(),
      severity: z.array(MonthBandDTOZ).optional(),
    })
    .optional(),
  mix: z
    .object({
      this_year: z.array(CategoryShareDTOZ),
      last_year: z.array(CategoryShareDTOZ),
    })
    .optional(),
  detver_markers: z.array(DetverMarkerDTOZ).optional(),
  meta: z.object({
    data_min_year: z.number().int(),
    data_max_year: z.number().int(),
    interval: z.literal("month"),
    generated_at: z.string(),
  }),
})

const CategoryStackItemDTOZ = z.object({
  key: z.string(),
  label: z.string(),
  counts: z.record(z.string(), z.number()).optional(),
  shares: z.record(z.string(), z.number()).optional(),
  mild: z.number(),
  strong: z.number(),
  slur_masked: z.number(),
  total: z.number(),
})

export const CategoriesStackRespDTOZ = z.object({
  severity_keys: z.array(z.enum(["mild", "strong", "slur_masked"])),
  totals_by_sev: z.record(z.string(), z.number()).optional(),
  totals_share_sev: z.record(z.string(), z.number()).optional(),
  sorted_by: z.enum(["hits", "share", "mild", "strong", "slur_masked"]).optional(),
  window: z.object({ start: z.string(), end: z.string() }),
  stack: z.array(CategoryStackItemDTOZ),
  total_hits: z.number(),
})

export const intoCategoriesStackResp = (d: DTO.CategoriesStackRespDTO): M.CategoriesStackResp => d
export const intoYearlyTrendsResp = (d: DTO.YearlyTrendsRespDTO): M.YearlyTrendsResp => d
