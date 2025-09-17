export type TimeRange = { start: string; end: string }

export type PageOpts = { cursor?: string; limit?: number }

export type GlobalOptions = {
  range: TimeRange
  interval?: "auto" | "hour" | "day" | "week" | "month"
  tz?: string
  normalize?: "none" | "per_utterance"
  langReliable?: boolean
  detver?: number[]
  repoHids?: string[]
  actorHids?: string[]
  nlLangs?: string[]
  codeLangs?: string[]
  page?: PageOpts
}

export type TimeseriesHitsInput = GlobalOptions

export type TimeseriesPoint = {
  t: string
  hits: number
  offendingUtterances: number
  allUtterances?: number
  intensity?: number
  coverage?: number
  rarity?: number
}

export type TimeseriesHitsResp = {
  interval: "hour" | "day" | "week" | "month"
  series: TimeseriesPoint[]
}

export type HeatmapWeeklyInput = GlobalOptions

export type HeatmapCell = {
  dow: number
  hour: number
  hits: number
  utterances: number
  ratio?: number
}
export type HeatmapWeeklyResp = { z: "hits" | "ratio"; grid: HeatmapCell[] }

export type LangBarsInput = GlobalOptions

export type LangBarItem = {
  lang: string
  hits: number
  utterances: number
  ratio?: number
}
export type LangBarsResp = { items: LangBarItem[]; totalHits: number }
