"use client"
import { useEffect } from "react"

export function setTheme(theme: string) {
  if (typeof document !== "undefined") {
    document.documentElement.setAttribute("data-theme", theme)
    localStorage.setItem("theme", theme)
  }
}

export function useInitTheme(defaultTheme = "crimson") {
  useEffect(() => {
    const saved = localStorage.getItem("theme")
    document.documentElement.setAttribute("data-theme", saved || defaultTheme)
  }, [defaultTheme])
}
