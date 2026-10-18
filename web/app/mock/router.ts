import { MockError } from './errors'
import { authRoutes } from './handlers/auth'
import { contestRoutes } from './handlers/contest'
import { identityRoutes } from './handlers/identity'
import { judgeRoutes } from './handlers/judge'
import { metaRoutes } from './handlers/meta'
import { problemRoutes } from './handlers/problem'
import { submissionRoutes } from './handlers/submission'
import type {
  MockAccount,
  MockContext,
  MockRequest,
  MockResponse,
  MockRoute,
  MockRuntime,
} from './types'
import type { HttpMethod } from '~/composables/useApi'

/**
 * The route table.
 *
 * Grouped the way `server/internal/httpapi` groups them, so a change on one side
 * has an obvious counterpart on the other.
 */
export const routes: MockRoute[] = [
  ...authRoutes,
  ...identityRoutes,
  ...problemRoutes,
  ...contestRoutes,
  ...judgeRoutes,
  ...submissionRoutes,
  ...metaRoutes,
]

export interface RouteMatch {
  route: MockRoute
  params: Record<string, string>
}

function segments(path: string): string[] {
  return path.split('/').filter((part) => part !== '')
}

export function matchRoute(method: HttpMethod, path: string): RouteMatch | null {
  const parts = segments(path)
  for (const route of routes) {
    if (route.method !== method) continue
    const pattern = segments(route.pattern)
    if (pattern.length !== parts.length) continue
    const params: Record<string, string> = {}
    let ok = true
    for (let i = 0; i < pattern.length; i++) {
      const expected = pattern[i]!
      const actual = parts[i]!
      if (expected.startsWith(':')) {
        params[expected.slice(1)] = decodeURIComponent(actual)
        continue
      }
      if (expected !== actual) {
        ok = false
        break
      }
    }
    if (ok) return { route, params }
  }
  return null
}

/**
 * Patterns that are registered twice. A duplicate is always a bug — one of the
 * two handlers is unreachable — so it is surfaced rather than silently resolved.
 */
export function duplicatePatterns(): string[] {
  const seen = new Set<string>()
  const duplicates = new Set<string>()
  for (const route of routes) {
    const key = `${route.method} ${route.pattern}`
    if (seen.has(key)) duplicates.add(key)
    seen.add(key)
  }
  return [...duplicates]
}

export interface DispatchInput {
  runtime: MockRuntime
  req: MockRequest
  account: MockAccount | null
}

export async function dispatchMock(input: DispatchInput): Promise<MockResponse> {
  const { runtime, req, account } = input
  const match = matchRoute(req.method, req.path)
  if (!match) {
    // Loud on purpose: an unmocked path means the mock has fallen behind the
    // frontend, and a silent 200 would hide that.
    throw new MockError('not_found', `the mock has no route for ${req.method} ${req.path}`)
  }
  const ctx: MockContext = {
    runtime,
    req,
    params: match.params,
    account,
    state: runtime.state,
    now: runtime.now(),
    body: req.body,
  }
  return await match.route.handler(ctx)
}
