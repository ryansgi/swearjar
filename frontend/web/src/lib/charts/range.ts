export function rangeFromYears(minYear: number, maxYear: number) {
  const clamp = (y: number) => Math.max(1970, Math.min(2100, y | 0))
  const min = clamp(minYear)
  const max = clamp(maxYear)
  return {
    start: `${min}-01-01`,
    end: `${max}-12-31`,
  }
}
