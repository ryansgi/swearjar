// May* getters: return defaults when missing; warn on invalid values
import { toBool, toDurationMs, toFloat, toInt } from "./helpers"
import type { ConfReader } from "./must"
import { logger } from "../logger"

export function mayString(r: ConfReader, key: string, def: string): string {
  const v = r.get(key)
  return (v ?? "").trim() || def
}
export function mayInt(r: ConfReader, key: string, def: number): number {
  const raw = (r.get(key) ?? "").trim()
  if (!raw) return def
  try {
    return toInt(raw)
  } catch {
    logger.warn({ key: r.key(key), value: raw, default: def }, "invalid int; using default")
    return def
  }
}
export function mayFloat64(r: ConfReader, key: string, def: number): number {
  const raw = (r.get(key) ?? "").trim()
  if (!raw) return def
  try {
    return toFloat(raw)
  } catch {
    logger.warn({ key: r.key(key), value: raw, default: def }, "invalid float; using default")
    return def
  }
}
export function mayBool(r: ConfReader, key: string, def: boolean): boolean {
  const raw = (r.get(key) ?? "").trim()
  if (!raw) return def
  try {
    return toBool(raw)
  } catch {
    logger.warn({ key: r.key(key), value: raw, default: def }, "invalid bool; using default")
    return def
  }
}
export function mayDurationMs(r: ConfReader, key: string, defMs: number): number {
  const raw = (r.get(key) ?? "").trim()
  if (!raw) return defMs
  try {
    return toDurationMs(raw)
  } catch {
    logger.warn({ key: r.key(key), value: raw, default: defMs }, "invalid duration; using default")
    return defMs
  }
}
export function mayCSV(r: ConfReader, key: string, def: string[]): string[] {
  const raw = (r.get(key) ?? "").trim()
  if (!raw) return def
  const parts = raw
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
  return parts.length ? parts : def
}
export function mayEnum(r: ConfReader, key: string, def: string, ...allowed: string[]): string {
  const v = mayString(r, key, def)
  if (!v) return v
  if (allowed.some((a) => a.toLowerCase() === v.toLowerCase())) return v
  throw new Error(`invalid enum for ${r.key(key)}: "${v}" not in [${allowed.join(", ")}]`)
}
