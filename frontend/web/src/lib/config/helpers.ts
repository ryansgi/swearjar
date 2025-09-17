// Shared helpers for config env handling + parsing utils

// Detect browser vs server
export const isBrowser = typeof window !== "undefined" && typeof document !== "undefined"

// Read an env var from available sources in priority:
// runtime injection (window.__ENV) -> bundler (import.meta.env) -> SSR (globalThis.process.env)
export function readRawEnv(key: string): string | undefined {
  // Runtime-injected (browser)
  if (isBrowser && (window as any).__ENV && (window as any).__ENV[key] !== undefined) {
    return (window as any).__ENV[key]
  }

  // import.meta.env (Vite/ESM bundlers)
  try {
    const metaEnv = (import.meta as any)?.env
    if (metaEnv && metaEnv[key] !== undefined) return metaEnv[key]
  } catch {}

  // process.env (SSR/node)
  try {
    const proc = (globalThis as any)?.process
    if (proc?.env && proc.env[key] !== undefined) {
      return proc.env[key]
    }
  } catch {}

  return undefined
}

// Public allowlist: only these CORE_FRONTEND_* keys are safe to ship to the client
export const PUBLIC_ALLOWLIST: Set<string> = new Set(["CORE_FRONTEND_API_URL"])

// Collect all CORE_FRONTEND_* vars from known sources
function collectAllCoreEnv(): Record<string, string> {
  const out: Record<string, string> = {}
  const candidates: Record<string, any>[] = []

  if (isBrowser && (window as any).__ENV) candidates.push((window as any).__ENV)

  try {
    const metaEnv = (import.meta as any)?.env
    if (metaEnv) candidates.push(metaEnv)
  } catch {}

  try {
    const proc = (globalThis as any)?.process
    if (proc?.env) candidates.push(proc.env)
  } catch {}

  for (const src of candidates) {
    for (const k of Object.keys(src)) {
      if (k.startsWith("CORE_FRONTEND_") && out[k] === undefined) {
        const v = String(src[k])
        if (v !== "undefined") out[k] = v
      }
    }
  }

  return out
}

const ALL_CORE_ENV = collectAllCoreEnv()

// Public (browser-safe) env: only allowlisted keys
export const publicEnv: Readonly<Record<string, string | undefined>> = Object.freeze(
  Object.fromEntries(Array.from(PUBLIC_ALLOWLIST).map((k) => [k, ALL_CORE_ENV[k]])),
)

// Server-only env: everything else in the namespace (not shipped to client)
// In the browser this will be an empty frozen object
export const serverEnv: Readonly<Record<string, string>> = Object.freeze(
  isBrowser
    ? {}
    : Object.fromEntries(Object.entries(ALL_CORE_ENV).filter(([k]) => !PUBLIC_ALLOWLIST.has(k))),
)

// Convenience single-value accessor
export function env(key: string, fallback?: string): string | undefined {
  const val = readRawEnv(key)
  return val ?? fallback
}

// Parsing utilities

export const toInt = (s: string) => {
  const v = Number.parseInt(s, 10)
  if (Number.isNaN(v)) throw new Error(`invalid int: ${s}`)
  return v
}

export const toFloat = (s: string) => {
  const v = Number.parseFloat(s)
  if (Number.isNaN(v)) throw new Error(`invalid float: ${s}`)
  return v
}

export const toBool = (s: string) => {
  const v = s.trim().toLowerCase()
  if (["true", "1", "yes", "y", "on"].includes(v)) return true
  if (["false", "0", "no", "n", "off"].includes(v)) return false
  throw new Error(`invalid bool: ${s}`)
}

const DUR_RE = /^(-?\d+)(ms|s|m|h)$/i
export const toDurationMs = (s: string) => {
  const m = s.trim().match(DUR_RE)
  if (!m) throw new Error(`invalid duration (e.g., 250ms, 2s, 1m, 1h): ${s}`)
  const n = Number(m[1])
  const unit = m[2].toLowerCase()
  const mult = unit === "ms" ? 1 : unit === "s" ? 1000 : unit === "m" ? 60000 : 3600000
  return n * mult
}

export const isAbsUrl = (s: string) => {
  try {
    const u = new URL(s)
    return !!u.protocol && !!u.host
  } catch {
    return false
  }
}
