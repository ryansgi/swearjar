// Lopgger - no dependencies, just simple log level filtering
// Usage: logger.info("something happened"), logger.error({err, code}, "failed to do X")
// Optionally set LOG_LEVEL env var to "debug", "info", "warn", "error", or "silent" (default: "info")
// You can also create child loggers with a base context: const log = logger.child({reqId}); log.info("...")

type Level = "debug" | "info" | "warn" | "error" | "silent"

const LEVELS: Record<Level, number> = {
  debug: 10,
  info: 20,
  warn: 30,
  error: 40,
  silent: 90,
}

// Read level from env (default: "info")
import { CORE } from "./config"
const levelName = (CORE.MayEnum("LOG_LEVEL", "info", "debug", "info", "warn", "error", "silent") ||
  "info") as Level

let currentLevel = LEVELS[levelName] ?? LEVELS.info

export type LogCtx = Record<string, unknown>

function logAt(consoleFn: (...args: any[]) => void, n: number, msg: string, ctx?: LogCtx) {
  if (n < currentLevel) return
  if (ctx && Object.keys(ctx).length) {
    consoleFn(msg, ctx)
  } else {
    consoleFn(msg)
  }
}

export const logger = {
  setLevel(l: Level) {
    currentLevel = LEVELS[l] ?? LEVELS.info
  },
  child(base: LogCtx) {
    return {
      debug: (ctx: LogCtx | string, msg?: string) =>
        typeof ctx === "string"
          ? logger.debug(base, ctx)
          : logger.debug({ ...base, ...(ctx || {}) }, msg || ""),
      info: (ctx: LogCtx | string, msg?: string) =>
        typeof ctx === "string"
          ? logger.info(base, ctx)
          : logger.info({ ...base, ...(ctx || {}) }, msg || ""),
      warn: (ctx: LogCtx | string, msg?: string) =>
        typeof ctx === "string"
          ? logger.warn(base, ctx)
          : logger.warn({ ...base, ...(ctx || {}) }, msg || ""),
      error: (ctx: LogCtx | string, msg?: string) =>
        typeof ctx === "string"
          ? logger.error(base, ctx)
          : logger.error({ ...base, ...(ctx || {}) }, msg || ""),
    }
  },
  debug(ctx: LogCtx | string, msg?: string) {
    if (typeof ctx === "string") return logAt(console.debug, LEVELS.debug, ctx)
    return logAt(console.debug, LEVELS.debug, msg || "", ctx)
  },
  info(ctx: LogCtx | string, msg?: string) {
    if (typeof ctx === "string") return logAt(console.info, LEVELS.info, ctx)
    return logAt(console.info, LEVELS.info, msg || "", ctx)
  },
  warn(ctx: LogCtx | string, msg?: string) {
    if (typeof ctx === "string") return logAt(console.warn, LEVELS.warn, ctx)
    return logAt(console.warn, LEVELS.warn, msg || "", ctx)
  },
  error(ctx: LogCtx | string, msg?: string) {
    if (typeof ctx === "string") return logAt(console.error, LEVELS.error, ctx)
    return logAt(console.error, LEVELS.error, msg || "", ctx)
  },
}
