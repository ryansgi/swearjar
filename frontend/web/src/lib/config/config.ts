// Conf class + high-level app config
import { env, publicEnv } from "./helpers"
import * as Must from "./must"
import * as May from "./may"

export class Conf {
  private readonly prefixStr: string
  constructor(prefix = "") {
    this.prefixStr = prefix
  }
  Prefix(p: string) {
    return new Conf(this.prefixStr + p)
  }
  key(k: string) {
    return this.prefixStr + k
  }
  get(k: string): string | undefined {
    return env(this.key(k))
  }

  // Must*
  MustString(key: string) {
    return Must.mustString(this, key)
  }
  MustInt(key: string) {
    return Must.mustInt(this, key)
  }
  MustFloat64(key: string) {
    return Must.mustFloat64(this, key)
  }
  MustBool(key: string) {
    return Must.mustBool(this, key)
  }
  MustDurationMs(key: string) {
    return Must.mustDurationMs(this, key)
  }
  MustURL(key: string) {
    return Must.mustURL(this, key)
  }
  MustPort(key: string) {
    return Must.mustPort(this, key)
  }
  Require(...keys: string[]) {
    return Must.requireKeys(this, ...keys)
  }

  // May*
  MayString(key: string, def: string) {
    return May.mayString(this, key, def)
  }
  MayInt(key: string, def: number) {
    return May.mayInt(this, key, def)
  }
  MayFloat64(key: string, def: number) {
    return May.mayFloat64(this, key, def)
  }
  MayBool(key: string, def: boolean) {
    return May.mayBool(this, key, def)
  }
  MayDurationMs(key: string, defMs: number) {
    return May.mayDurationMs(this, key, defMs)
  }
  MayCSV(key: string, def: string[]) {
    return May.mayCSV(this, key, def)
  }
  MayEnum(key: string, def: string, ...allowed: string[]) {
    return May.mayEnum(this, key, def, ...allowed)
  }
}

// A ready-to-use, namespaced config rooted at CORE_FRONTEND_
export const CORE = new Conf("CORE_FRONTEND_")

export type AppConfig = {
  apiUrl: string
  exploreMinDate: string
  exploreDefaultYear: string
}

// App-level config (browser-safe values must be allowlisted via publicEnv)
export const config: Readonly<AppConfig> = Object.freeze({
  apiUrl: CORE.MayString("API_URL", "http://sw_api:4000"),
  exploreMinDate: CORE.MayString("EXPLORE_MIN_DATE", "2011-02-12T00:00:00Z"),
  exploreDefaultYear: CORE.MayString("EXPLORE_DEFAULT_YEAR", "2014"),
})

// Re-export browser-ship-safe env map
export { publicEnv } from "./helpers"
