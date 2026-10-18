/**
 * Cookie access for the mock.
 *
 * The browser path writes real cookies, because `useApi` reads the CSRF token
 * straight from `document.cookie`. The jar is the fallback for environments
 * without a DOM (unit tests), so the mock's auth flow is testable in plain Node.
 *
 * Fidelity gap: the real session cookie is `HttpOnly`, and JavaScript cannot set
 * that. Nothing in the app reads it, but a curious user could.
 */

const CSRF_COOKIE = 'ephyra_csrf'
const SESSION_COOKIE = 'ephyra_session'

const jar = new Map<string, string>()

function hasDocument(): boolean {
  return typeof document !== 'undefined'
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function readCookie(name: string): string | null {
  if (hasDocument()) {
    const match = document.cookie.match(new RegExp(`(?:^|; )${escapeRegExp(name)}=([^;]*)`))
    return match?.[1] ? decodeURIComponent(match[1]) : null
  }
  return jar.get(name) ?? null
}

export function setCookie(name: string, value: string, maxAgeSeconds?: number): void {
  if (hasDocument()) {
    const maxAge = maxAgeSeconds === undefined ? '' : `; Max-Age=${maxAgeSeconds}`
    document.cookie = `${name}=${encodeURIComponent(value)}; Path=/${maxAge}; SameSite=Lax`
    return
  }
  jar.set(name, value)
}

export function clearCookie(name: string): void {
  if (hasDocument()) {
    document.cookie = `${name}=; Path=/; Max-Age=0; SameSite=Lax`
    return
  }
  jar.delete(name)
}

/* --------------------------------------------------------------- session */

/**
 * The real backend stores an opaque token server-side. There is no server here,
 * so the cookie carries the account id: it keeps the session alive across a page
 * reload without needing to persist the whole mock world.
 */
const SESSION_PREFIX = 'mock-session-'

export function issueSession(accountId: string): void {
  setCookie(SESSION_COOKIE, `${SESSION_PREFIX}${accountId}`, 12 * 3600)
}

export function clearSession(): void {
  clearCookie(SESSION_COOKIE)
}

export function sessionAccountId(): string | null {
  const raw = readCookie(SESSION_COOKIE)
  if (!raw || !raw.startsWith(SESSION_PREFIX)) return null
  return raw.slice(SESSION_PREFIX.length) || null
}

/** Issues the readable CSRF cookie, mirroring `GET /api/auth/csrf`. */
let csrfCounter = 0

export function issueCsrfToken(): string {
  const existing = readCookie(CSRF_COOKIE)
  if (existing) return existing
  const token = `mock-csrf-token-${++csrfCounter}`
  setCookie(CSRF_COOKIE, token, 12 * 3600)
  return token
}

export const CSRF_COOKIE_NAME = CSRF_COOKIE
export const CSRF_HEADER_NAME = 'X-CSRF-Token'
