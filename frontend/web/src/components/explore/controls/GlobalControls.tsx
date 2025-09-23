"use client"
import { PeriodControl } from "./PeriodControl"
import { DateField } from "./DateField"
import { BucketToggle } from "./BucketToggle"
import { MetricToggle } from "./MetricToggle"
import { SeriesToggle } from "./SeriesToggle"
import { TimezoneField } from "./TimezoneField"
import { ScopeChips } from "./ScopeChips"
import { ExportMenu } from "./ExportMenu"
import { ResetButton } from "./ResetButton"

export function GlobalControls() {
  return (
    <div className="supports-[backdrop-filter]:bg-background/70 sticky top-0 z-20 border-b py-3 backdrop-blur">
      <div className="container mx-auto space-y-3 px-0">
        <div className="grid grid-cols-1 gap-2 md:grid-cols-12">
          <div className="flex flex-wrap items-center gap-2 md:col-span-5">
            <PeriodControl />
            <DateField />
          </div>
          <div className="flex flex-wrap items-center gap-2 md:col-span-4">
            <BucketToggle />
            <MetricToggle />
            <SeriesToggle />
          </div>
          <div className="flex flex-wrap items-center gap-2 md:col-span-3 md:justify-end">
            <TimezoneField />
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <ScopeChips />
          <div className="ml-auto flex items-center gap-2">
            <ResetButton />
            <ExportMenu />
          </div>
        </div>
      </div>
    </div>
  )
}
