// Must* getters: throw on missing or invalid values
import { isAbsUrl, toBool, toDurationMs, toFloat, toInt } from "./helpers"

// Minimal reader interface to avoid circular imports
export interface ConfReader {
  key(k: string): string
  get(k: string): string | undefined
}

export function mustString(r: ConfReader, key: string): string {
  const v = (r.get(key) ?? "").trim()
  if (!v) throw new Error(`missing required env: ${r.key(key)}`)
  return v
}
export function mustInt(r: ConfReader, key: string): number {
  return toInt(mustString(r, key))
}
export function mustFloat64(r: ConfReader, key: string): number {
  return toFloat(mustString(r, key))
}
export function mustBool(r: ConfReader, key: string): boolean {
  return toBool(mustString(r, key))
}
export function mustDurationMs(r: ConfReader, key: string): number {
  return toDurationMs(mustString(r, key))
}
export function mustURL(r: ConfReader, key: string): string {
  const s = mustString(r, key)
  if (!isAbsUrl(s)) throw new Error(`invalid absolute URL for ${r.key(key)}: ${s}`)
  return s
}
export function mustPort(r: ConfReader, key: string): string {
  const s = mustString(r, key)
  const p = Number.parseInt(s, 10)
  if (!Number.isInteger(p) || p < 1 || p > 65535) {
    throw new Error(`invalid TCP port for ${r.key(key)}: expected 1..65535, got ${s}`)
  }
  return `:${p}`
}
export function requireKeys(r: ConfReader, ...keys: string[]) {
  for (const k of keys) mustString(r, k)
}
