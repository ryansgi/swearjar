export type ApiError = {
  status: number
  code?: string | number
  message: string
  details?: unknown
}

export type FetcherOpts = {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE"
  headers?: Record<string, string>
  body?: any
  signal?: AbortSignal
  credentials?: RequestCredentials
}

export type Page = {
  total: number
  page: number
  pageSize: number
  cursor?: string
}

const BASE = "/api/"

async function parse<T>(res: Response): Promise<T> {
  const txt = await res.text()
  if (!txt) return undefined as unknown as T
  try {
    return JSON.parse(txt) as T
  } catch {
    return txt as unknown as T
  }
}

async function doFetch<T>(path: string, opts: FetcherOpts = {}): Promise<T> {
  const res = await fetch(`${BASE}${path.replace(/^\//, "")}`, {
    method: opts.method ?? "GET",
    headers: { "content-type": "application/json", ...(opts.headers ?? {}) },
    credentials: opts.credentials ?? "include",
    body: opts.body ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
  })

  if (res.ok) return parse<T>(res)

  // HTTP-level error (non-2xx)
  const payload = await parse<any>(res).catch(() => undefined)
  const err: ApiError = {
    status: res.status,
    code: payload?.code ?? payload?.status_code,
    message:
      payload?.error ?? payload?.message ?? payload?.status ?? res.statusText ?? "Request failed",
    details: { requestId: payload?.request_id, raw: payload },
  }
  throw err
}

// Minimal passthrough hook (kept for future retries/backoff)
async function passthrough<T>(fn: () => Promise<T>): Promise<T> {
  return fn()
}

/**
 * Unwraps a backend Envelope and returns { data, page }.
 * Throws ApiError when the envelope indicates failure, even if HTTP 200
 */
function unwrapEnvelope<T>(raw: any): { data: T; page?: Page } {
  // Accept both camel/snake variants defensively
  const statusCode: number | undefined =
    typeof raw?.status_code === "number" ? raw.status_code : raw?.statusCode
  const statusText: string | undefined = raw?.status ?? raw?.statusText
  const code = raw?.code
  const errorMsg: string | undefined = raw?.error

  // Page (normalize to camelCase)
  const pageRaw = raw?.page
  const page: Page | undefined = pageRaw
    ? {
        total: pageRaw.total,
        page: pageRaw.page,
        pageSize: pageRaw.page_size ?? pageRaw.pageSize,
        cursor: pageRaw.cursor,
      }
    : undefined

  // If status_code is present, use envelope semantics
  if (typeof statusCode === "number") {
    const ok = statusCode >= 200 && statusCode < 300
    if (!ok || errorMsg) {
      const err: ApiError = {
        status: statusCode,
        code,
        message: errorMsg || statusText || "Request failed",
        details: { requestId: raw?.request_id, page, raw },
      }
      throw err
    }
  }

  // Default to raw.data when present, otherwise raw (for non-enveloped endpoints)
  const data: T = (raw?.data ?? raw) as T
  return { data, page }
}

export const api = {
  // Basic JSON fetchers (no validation/scan)
  get: <T>(path: string, opts?: FetcherOpts) => passthrough(() => doFetch<T>(path, opts)),
  post: <T>(path: string, body?: any, opts?: FetcherOpts) =>
    passthrough(() => doFetch<T>(path, { ...opts, method: "POST", body })),
  put: <T>(path: string, body?: any, opts?: FetcherOpts) =>
    passthrough(() => doFetch<T>(path, { ...opts, method: "PUT", body })),
  patch: <T>(path: string, body?: any, opts?: FetcherOpts) =>
    passthrough(() => doFetch<T>(path, { ...opts, method: "PATCH", body })),
  del: <T>(path: string, opts?: FetcherOpts) =>
    passthrough(() => doFetch<T>(path, { ...opts, method: "DELETE" })),

  /**
   * Decode helpers: fetch -> unwrap envelope -> validate (schema.parse) -> scan (into)
   * Keep call sites small and uniform.
   */
  decode: {
    get: async <DTO, MODEL>(
      path: string,
      schema: { parse: (u: unknown) => DTO },
      into: (dto: DTO) => MODEL,
      opts?: FetcherOpts,
    ): Promise<MODEL> => {
      const raw = await doFetch<unknown>(path, opts)
      const { data } = unwrapEnvelope<unknown>(raw)
      const dto = schema.parse(data)
      return into(dto)
    },

    post: async <DTO, MODEL>(
      path: string,
      body: any,
      schema: { parse: (u: unknown) => DTO },
      into: (dto: DTO) => MODEL,
      opts?: FetcherOpts,
    ): Promise<MODEL> => {
      const raw = await doFetch<unknown>(path, { ...opts, method: "POST", body })
      const { data } = unwrapEnvelope<unknown>(raw)
      const dto = schema.parse(data)
      return into(dto)
    },

    // Variant that also returns pagination info from the envelope.
    getWithPage: async <DTO, MODEL>(
      path: string,
      schema: { parse: (u: unknown) => DTO },
      into: (dto: DTO) => MODEL,
      opts?: FetcherOpts,
    ): Promise<{ data: MODEL; page?: Page }> => {
      const raw = await doFetch<unknown>(path, opts)
      const unwrapped = unwrapEnvelope<unknown>(raw)
      const dto = schema.parse(unwrapped.data)
      return { data: into(dto), page: unwrapped.page }
    },

    postWithPage: async <DTO, MODEL>(
      path: string,
      body: any,
      schema: { parse: (u: unknown) => DTO },
      into: (dto: DTO) => MODEL,
      opts?: FetcherOpts,
    ): Promise<{ data: MODEL; page?: Page }> => {
      const raw = await doFetch<unknown>(path, { ...opts, method: "POST", body })
      const unwrapped = unwrapEnvelope<unknown>(raw)
      const dto = schema.parse(unwrapped.data)
      return { data: into(dto), page: unwrapped.page }
    },
  },
}
