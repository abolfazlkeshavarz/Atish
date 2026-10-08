const BASE: string = (import.meta.env.VITE_API_BASE as string | undefined) ?? ''

export class ApiError extends Error {
  status: number
  code: string
  data?: Record<string, unknown>
  constructor(status: number, code: string, message: string, data?: Record<string, unknown>) {
    super(message)
    this.status = status
    this.code = code
    this.data = data
  }
}

const TOKEN_KEY = 'atish.token'
let token: string | null = null
try {
  token = localStorage.getItem(TOKEN_KEY)
} catch {
  /* storage blocked */
}

export const auth = {
  get: () => token,
  set(t: string | null) {
    token = t
    try {
      if (t) localStorage.setItem(TOKEN_KEY, t)
      else localStorage.removeItem(TOKEN_KEY)
    } catch {
      /* ignore */
    }
  },
}

let onUnauthorized: (() => Promise<boolean>) | null = null
export const setUnauthorizedHandler = (fn: () => Promise<boolean>) => {
  onUnauthorized = fn
}

interface Opts {
  body?: unknown
  form?: FormData
  query?: Record<string, string | number | undefined | null>
  noAuth?: boolean
  token?: string | null
}

export async function api<T = unknown>(method: string, path: string, opts: Opts = {}, retried = false): Promise<T> {
  let url = BASE + path
  if (opts.query) {
    const q = new URLSearchParams()
    for (const [k, v] of Object.entries(opts.query)) if (v !== undefined && v !== null && v !== '') q.set(k, String(v))
    const s = q.toString()
    if (s) url += (url.includes('?') ? '&' : '?') + s
  }
  const headers: Record<string, string> = {}
  const t = opts.token === undefined ? token : opts.token
  if (t && !opts.noAuth) headers.Authorization = `Bearer ${t}`
  let body: BodyInit | undefined
  if (opts.form) body = opts.form
  else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  let res: Response
  try {
    res = await fetch(url, { method, headers, body })
  } catch {
    throw new ApiError(0, 'network', 'No connection. Check your internet and try again.')
  }
  if (res.status === 204) return undefined as T
  const text = await res.text()
  let json: any = null
  try {
    json = text ? JSON.parse(text) : null
  } catch {
    /* non-json */
  }

  if (res.status === 401 && !retried && !opts.noAuth && onUnauthorized) {
    if (await onUnauthorized()) return api<T>(method, path, opts, true)
  }
  if (!res.ok) {
    const e = json?.error
    throw new ApiError(res.status, e?.code ?? 'error', e?.message ?? 'Something went wrong', e?.data)
  }
  return json as T
}

type Q = Opts['query']
export const get = <T>(path: string, query?: Q) => api<T>('GET', path, { query })
export const post = <T>(path: string, body?: unknown) => api<T>('POST', path, { body })
export const patch = <T>(path: string, body?: unknown) => api<T>('PATCH', path, { body })
export const put = <T>(path: string, body?: unknown) => api<T>('PUT', path, { body })
export const del = <T>(path: string, body?: unknown) => api<T>('DELETE', path, { body })
