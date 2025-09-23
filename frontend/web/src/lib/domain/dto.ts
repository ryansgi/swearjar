export type TimeRangeDTO = {
  start: string
  end: string
}

export type PageOptsDTO = {
  cursor?: string
  limit?: number
}

export type MetricDTO = "counts" | "intensity" | "coverage" | "rarity"
export type SeriesDTO = "hits" | "offending_utterances" | "all_utterances"

export type GlobalOptionsDTO = {
  range: TimeRangeDTO
  interval?: "auto" | "hour" | "day" | "week" | "month"
  tz?: string
  normalize?: "none" | "per_utterance"
  lang_reliable?: boolean
  metric?: MetricDTO
  series?: SeriesDTO
  detver?: number[]
  repo_hids?: string[]
  actor_hids?: string[]
  nl_langs?: string[]
  code_langs?: string[]

  page?: PageOptsDTO
}

export type TimeseriesHitsInputDTO = {} & GlobalOptionsDTO

export type TimeseriesPointDTO = {
  t: string
  hits: number
  offending_utterances: number
  all_utterances?: number
  intensity?: number
  coverage?: number
  rarity?: number
}

export type TimeseriesHitsRespDTO = {
  interval: "hour" | "day" | "week" | "month"
  series: TimeseriesPointDTO[]
}

export type HeatmapWeeklyInputDTO = GlobalOptionsDTO

export type HeatmapCellDTO = {
  dow: number
  hour: number
  hits: number
  offending_utterances?: number
  utterances: number
  ratio?: number
}

export type HeatmapWeeklyRespDTO = {
  z: "hits" | "offending_utterances" | "all_utterances" | "ratio"
  grid: HeatmapCellDTO[]
}

export type LangBarsInputDTO = GlobalOptionsDTO

export type LangBarItemDTO = {
  lang: string
  hits: number
  utterances: number
  ratio?: number
}

export type LangBarsRespDTO = {
  items: LangBarItemDTO[]
  total_hits: number
}

export type KPIStripInputDTO = GlobalOptionsDTO

export type KPIStripRespDTO = {
  day: string
  hits: number
  offending_utterances: number
  repos: number
  actors: number
  all_utterances?: number
  intensity?: number // hits / offending_utterances
  coverage?: number // offending_utterances / all_utterances
  rarity?: number // hits / all_utterances
}

export type YearRangeDTO = { min?: number; max?: number }
export type IncludeKeyDTO = "hits" | "rate" | "severity" | "mix" | "detver_markers" | "seasonality"

export type MonthBandDTO = { m: number; median: number; p25: number; p75: number }
export type CategoryShareDTO = { key: string; hits: number; share: number }
export type DetverMarkerDTO = { date: string; version: number }

export type YearlyTrendsInputDTO = GlobalOptionsDTO & {
  year_range?: YearRangeDTO
  include?: IncludeKeyDTO[]
}

export type YearlyTrendsRespDTO = {
  years: number[]
  monthly: {
    hits?: Record<string, number[]> // "2014" -> [12]
    rate?: Record<string, number[]> // [12]
    severity?: Record<string, number[]> // [12]
  }
  seasonality?: {
    hits?: MonthBandDTO[]
    rate?: MonthBandDTO[]
    severity?: MonthBandDTO[]
  }
  mix?: {
    this_year: CategoryShareDTO[]
    last_year: CategoryShareDTO[]
  }
  detver_markers?: DetverMarkerDTO[]
  meta: {
    data_min_year: number
    data_max_year: number
    interval: "month"
    generated_at: string
  }
}

export type CategoriesStackInputDTO = GlobalOptionsDTO & {
  top_n?: number
  sort_by?: "hits" | "share" | "mild" | "strong" | "slur_masked"
  include_other?: boolean
  as_share?: boolean
  severities?: Array<"mild" | "strong" | "slur_masked">
  categories?: string[]
}

export type CategoryStackItemDTO = {
  key: string
  label: string
  counts?: Record<string, number>
  shares?: Record<string, number>
  mild: number
  strong: number
  slur_masked: number
  total: number
}

export type CategoriesStackRespDTO = {
  severity_keys: Array<"mild" | "strong" | "slur_masked">
  totals_by_sev?: Record<string, number>
  totals_share_sev?: Record<string, number>
  sorted_by?: "hits" | "share" | "mild" | "strong" | "slur_masked"
  window: { start: string; end: string }
  stack: CategoryStackItemDTO[]
  total_hits: number
}
