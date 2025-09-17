export type TimeRangeDTO = {
  start: string
  end: string
}

export type PageOptsDTO = {
  cursor?: string
  limit?: number
}

export type GlobalOptionsDTO = {
  range: TimeRangeDTO
  interval?: "auto" | "hour" | "day" | "week" | "month"
  tz?: string
  normalize?: "none" | "per_utterance"
  lang_reliable?: boolean

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
  utterances: number
  ratio?: number
}

export type HeatmapWeeklyRespDTO = {
  z: "hits" | "ratio"
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
