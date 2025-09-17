"use client"
import * as React from "react"

export function useMedia(query: string) {
  const [matches, set] = React.useState(false)
  React.useEffect(() => {
    const m = window.matchMedia(query)
    const on = () => set(m.matches)
    on()
    m.addEventListener?.("change", on)
    return () => m.removeEventListener?.("change", on)
  }, [query])
  return matches
}
