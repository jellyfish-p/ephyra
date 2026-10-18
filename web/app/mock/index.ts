import { MockEventBus } from './bus'
import {
  clearSession,
  CSRF_COOKIE_NAME,
  CSRF_HEADER_NAME,
  issueCsrfToken,
  readCookie,
  sessionAccountId,
} from './cookies'
import { MockError } from './errors'
import { JudgeScheduler } from './judging'
import { dispatchMock } from './router'
import { createMockState, stateNow } from './state'
import type { HttpMethod } from '~/composables/useApi'
import type { Submission } from '~/types/api'
import type { MockAccount, MockRequest, MockResponse, MockRuntime, MockState } from './types'

/**
 * The mock's entry point.
 *
 * `mockFetch` mirrors `$fetch` closely enough that `useApi` cannot tell the
 * difference: same paths, same bodies, same error shape, same status codes.
 * `mockSubscribe` replaces the SSE stream, which `EventSource` makes
 * uninterceptable.
 */

export interface MockOptions {
  /** Simulated round-trip latency; `0` in tests. */
  latencyMs?: number
  /** Freezes the clock, so contest phases are deterministic in tests. */
  fixedNow?: Date
}

let state: MockState | null = null
let scheduler: JudgeScheduler | null = null
let bus: MockEventBus | null = null
let runtime: MockRuntime | null = null
let latencyMs = 80
let fixedNow: Date | null = null

function build(): MockRuntime {
  // Contest times are laid out around `fixedNow` (or the wall clock), and the
  // clock is frozen to the same instant so a test's phases never drift.
  state = createMockState(fixedNow ?? new Date(), fixedNow !== null)
  scheduler = new JudgeScheduler()
  bus = new MockEventBus()
  const currentBus = bus
  const currentScheduler = scheduler
  runtime = {
    state,
    now: () => stateNow(state!),
    schedule: (delay, run) => currentScheduler.schedule(delay, run),
    publish: (contestId, project) => currentBus.publish(contestId, project),
    cancelScheduled: () => currentScheduler.cancelAll(),
    reset: () => {
      currentScheduler.cancelAll()
      currentBus.clear()
      clearSession()
      state = null
      scheduler = null
      bus = null
      runtime = null
    },
  }
  return runtime
}

function ensureRuntime(): MockRuntime {
  return runtime ?? build()
}

export function configureMock(options: MockOptions): void {
  if (options.latencyMs !== undefined) latencyMs = options.latencyMs
  if (options.fixedNow !== undefined) fixedNow = options.fixedNow
  // A configuration change invalidates the world it was built with.
  runtime = null
}

/** Drops all mock state and signs out. */
export function resetMock(): void {
  ensureRuntime().reset()
}

export function mockState(): MockState {
  return ensureRuntime().state
}

export function mockRuntime(): MockRuntime {
  return ensureRuntime()
}

/* ---------------------------------------------------------------- session */

function currentAccount(): MockAccount | null {
  const id = sessionAccountId()
  if (!id) return null
  return ensureRuntime().state.accounts.find((a) => a.id === id) ?? null
}

/* ----------------------------------------------------------------- fetch */

const UNSAFE: HttpMethod[] = ['POST', 'PATCH', 'PUT', 'DELETE']

export interface MockFetchOptions {
  method?: HttpMethod
  query?: Record<string, unknown>
  body?: unknown
  headers?: Record<string, string>
  responseType?: 'json' | 'text' | 'blob' | 'arrayBuffer'
}

/**
 * The CSRF check the real backend performs. Enforcing it here means the app's
 * CSRF handling is genuinely exercised in mock mode instead of being dead code.
 */
function requireCsrf(method: HttpMethod, headers: Record<string, string>): void {
  if (!UNSAFE.includes(method)) return
  const cookie = readCookie(CSRF_COOKIE_NAME) ?? issueCsrfToken()
  const sent = headers[CSRF_HEADER_NAME]
  if (!sent || sent !== cookie) {
    throw new MockError('forbidden', 'missing or invalid CSRF token')
  }
}

/** Shapes a failure the way `$fetch` does, so `useApi`'s normalizer works. */
function transportError(response: { status: number; body: unknown }): Error {
  const body = response.body as { error?: { message?: string } } | undefined
  const error = new Error(body?.error?.message ?? 'request failed') as Error & {
    response: { status: number; _data: unknown }
  }
  error.response = { status: response.status, _data: response.body }
  return error
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function mockFetch<T>(path: string, options: MockFetchOptions = {}): Promise<T> {
  const method = options.method ?? 'GET'
  const headers = options.headers ?? {}
  let response: MockResponse
  try {
    requireCsrf(method, headers)
    const req: MockRequest = {
      method,
      path,
      query: options.query ?? {},
      body: options.body,
      headers,
    }
    response = await dispatchMock({
      runtime: ensureRuntime(),
      req,
      account: currentAccount(),
    })
  } catch (e) {
    // The transport boundary, where a thrown `MockError` becomes the same
    // status-plus-body response the Go error middleware would have produced.
    if (!(e instanceof MockError)) throw e
    response = { status: e.status, body: e.toBody() }
  }
  if (latencyMs > 0) await delay(latencyMs)
  if (response.status >= 400) throw transportError(response)
  if (response.contentType || options.responseType === 'text') {
    return String(response.body) as T
  }
  return response.body as T
}

/* ------------------------------------------------------------------ SSE */

/**
 * Replaces `GET /api/contests/:id/events`. The subscription resolves the viewer
 * once, so every later frame is projected for that viewer specifically.
 */
export function mockSubscribe(
  contestId: string,
  listener: (submission: Submission) => void,
): () => void {
  ensureRuntime()
  const account = currentAccount()
  return bus!.subscribe(contestId, account?.id ?? null, listener)
}
