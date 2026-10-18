import { mockFetch } from '~/mock'
import type { ApiErrorBody, ApiErrorCode } from '~/types/api'

const SESSION_COOKIE = 'ephyra_session'
const CSRF_COOKIE = 'ephyra_csrf'
const CSRF_HEADER = 'X-CSRF-Token'

/** A normalized failure from the Ephyra API. */
export interface ApiError extends Error {
  /** HTTP status, or `0` when the request never reached the server. */
  status: number
  code: ApiErrorCode | 'network'
}

export function isApiError(e: unknown): e is ApiError {
  return e instanceof Error && 'code' in e && 'status' in e
}

export function asApiError(e: unknown): ApiError {
  if (isApiError(e)) return e
  const err = e as { message?: string }
  const out = new Error(err?.message || '请求失败') as ApiError
  out.status = 0
  out.code = 'network'
  return out
}

function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null
  const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = document.cookie.match(new RegExp(`(?:^|; )${escaped}=([^;]*)`))
  return match?.[1] ? decodeURIComponent(match[1]) : null
}

function toApiError(e: unknown): ApiError {
  const err = e as {
    message?: string
    status?: number
    statusCode?: number
    data?: ApiErrorBody
    response?: { status?: number; _data?: ApiErrorBody }
  }
  const status = err?.response?.status ?? err?.status ?? err?.statusCode ?? 0
  const body = err?.data ?? err?.response?._data
  const code = body?.error?.code ?? (status === 0 ? 'network' : 'internal')
  const message =
    body?.error?.message ??
    (status === 0 ? '无法连接到服务器，请确认后端已启动' : err?.message || '请求失败')
  const out = new Error(message) as ApiError
  out.status = status
  out.code = code
  return out
}

export type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'

export interface RequestOptions {
  method?: HttpMethod
  query?: Record<string, unknown>
  body?: unknown
  headers?: Record<string, string>
  responseType?: 'json' | 'text' | 'blob' | 'arrayBuffer'
}

export function useApi() {
  const base = useRuntimeConfig().public.apiBase as string
  const mock = useMockEnabled()

  /** Absolute URL for cases where `$fetch` is not usable (EventSource, downloads). */
  function apiUrl(path: string): string {
    return `${base.replace(/\/$/, '')}${path}`
  }

  /**
   * The backend rejects every unsafe request without `X-CSRF-Token`. The token
   * is a readable cookie, issued by `GET /api/auth/csrf` and refreshed on login.
   */
  async function ensureCsrfCookie(): Promise<void> {
    if (readCookie(CSRF_COOKIE)) return
    try {
      if (mock) {
        // The mock enforces CSRF too, so it has to issue the cookie the same way.
        await mockFetch('/api/auth/csrf')
        return
      }
      await $fetch('/api/auth/csrf', { baseURL: base, credentials: 'include' })
    } catch {
      // The request itself will surface a useful error.
    }
  }

  async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const method: HttpMethod = options.method ?? 'GET'
    const headers: Record<string, string> = { ...options.headers }

    if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
      await ensureCsrfCookie()
      const token = readCookie(CSRF_COOKIE)
      if (token) headers[CSRF_HEADER] = token
    }

    if (mock) {
      return await mockFetch<T>(path, {
        method,
        query: options.query,
        body: options.body,
        headers,
        responseType: options.responseType,
      })
    }

    try {
      return await $fetch<T>(path, {
        baseURL: base,
        credentials: 'include',
        method,
        headers,
        query: options.query,
        body: options.body as Record<string, unknown> | undefined,
        responseType: options.responseType ?? 'json',
      })
    } catch (e) {
      throw toApiError(e)
    }
  }

  const get = <T>(path: string, query?: Record<string, unknown>) => request<T>(path, { query })

  const post = <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body })

  const patch = <T>(path: string, body?: unknown) => request<T>(path, { method: 'PATCH', body })

  const postForm = <T>(path: string, form: FormData) =>
    request<T>(path, { method: 'POST', body: form })

  const getText = (path: string) => request<string>(path, { responseType: 'text' })

  return { request, get, post, patch, postForm, getText, apiUrl, base }
}
