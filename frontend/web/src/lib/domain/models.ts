// Mirror Go JSON exactly

export type TimeRange = import("./dto").TimeRangeDTO
export type PageOpts = import("./dto").PageOptsDTO
export type Metric = import("./dto").MetricDTO
export type Series = import("./dto").SeriesDTO
export type GlobalOptions = import("./dto").GlobalOptionsDTO

export type TimeseriesHitsInput = GlobalOptions
export type TimeseriesPoint = import("./dto").TimeseriesPointDTO
export type TimeseriesHitsResp = import("./dto").TimeseriesHitsRespDTO

export type HeatmapWeeklyInput = GlobalOptions
export type HeatmapCell = import("./dto").HeatmapCellDTO
export type HeatmapWeeklyResp = import("./dto").HeatmapWeeklyRespDTO

export type LangBarsInput = GlobalOptions
export type LangBarItem = import("./dto").LangBarItemDTO
export type LangBarsResp = import("./dto").LangBarsRespDTO

export type KPIStripInput = GlobalOptions
export type KPIStripResp = import("./dto").KPIStripRespDTO

export type YearRange = import("./dto").YearRangeDTO
export type IncludeKey = import("./dto").IncludeKeyDTO
export type MonthBand = import("./dto").MonthBandDTO
export type CategoryShare = import("./dto").CategoryShareDTO
export type DetverMarker = import("./dto").DetverMarkerDTO
export type YearlyTrendsInput = import("./dto").YearlyTrendsInputDTO
export type YearlyTrendsResp = import("./dto").YearlyTrendsRespDTO

export type CategoriesStackInput = import("./dto").CategoriesStackInputDTO
export type CategoryStackItem = import("./dto").CategoryStackItemDTO
export type CategoriesStackResp = import("./dto").CategoriesStackRespDTO
