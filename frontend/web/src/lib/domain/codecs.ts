import { z } from "zod"

import type {
  TimeseriesHitsRespDTO,
  TimeseriesPointDTO,
  HeatmapWeeklyRespDTO,
  LangBarsRespDTO,
  GlobalOptionsDTO,
} from "./dto"
import type {
  TimeseriesHitsResp,
  TimeseriesPoint,
  HeatmapWeeklyResp,
  LangBarsResp,
  GlobalOptions,
} from "./models"

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
  ratio: z.number().optional(),
})

export const HeatmapWeeklyRespDTOZ = z.object({
  z: z.enum(["hits", "ratio"]),
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

export function intoTimeseriesPoint(d: TimeseriesPointDTO): TimeseriesPoint {
  return {
    t: d.t,
    hits: d.hits,
    offendingUtterances: d.offending_utterances,
    allUtterances: d.all_utterances,
    intensity: d.intensity,
    coverage: d.coverage,
    rarity: d.rarity,
  }
}

export function intoTimeseriesHitsResp(d: TimeseriesHitsRespDTO): TimeseriesHitsResp {
  return {
    interval: d.interval,
    series: d.series.map(intoTimeseriesPoint),
  }
}

export function intoHeatmapWeeklyResp(d: HeatmapWeeklyRespDTO): HeatmapWeeklyResp {
  return {
    z: d.z,
    grid: d.grid.map((c) => ({ ...c })),
  }
}

export function intoLangBarsResp(d: LangBarsRespDTO): LangBarsResp {
  return {
    items: d.items.map((i) => ({ ...i })),
    totalHits: d.total_hits,
  }
}

export function toGlobalOptionsDTO(m: GlobalOptions): GlobalOptionsDTO {
  return {
    range: m.range,
    interval: m.interval,
    tz: m.tz,
    normalize: m.normalize,
    lang_reliable: m.langReliable,
    detver: m.detver,
    repo_hids: m.repoHids,
    actor_hids: m.actorHids,
    nl_langs: m.nlLangs,
    code_langs: m.codeLangs,
    page: m.page,
  }
}
