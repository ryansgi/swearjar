"use client"
import { useCallback, useMemo } from "react"
import { usePathname, useRouter, useSearchParams } from "next/navigation"

export type ExploreTab = "overview" | "repositories" | "actors" | "terms" | "samples"

/**
 * URL state manager for SPA-style permalinks.
 * Adds/updates `tab`, and leaves room for future keys like `year`, `repo`, `actor`, etc.
 */
export function useExploreState() {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()

  const tab = (searchParams.get("tab") as ExploreTab | null) ?? null

  const buildUrl = useCallback(
    (next: Partial<{ tab: ExploreTab }>) => {
      const sp = new URLSearchParams(searchParams.toString())
      if (next.tab) sp.set("tab", next.tab)
      if (next.tab === undefined) sp.delete("tab")
      const qs = sp.toString()
      return qs ? `${pathname}?${qs}` : pathname
    },
    [pathname, searchParams],
  )

  const setTab = useCallback(
    (next: ExploreTab) => {
      router.replace(buildUrl({ tab: next }), { scroll: false })
    },
    [router, buildUrl],
  )

  return useMemo(() => ({ tab, setTab }), [tab, setTab])
}
