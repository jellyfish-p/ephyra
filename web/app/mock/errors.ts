import type { ApiErrorCode } from '~/types/api'

/**
 * HTTP status for each `apperr.Code`, mirroring `statusFor` in
 * `server/internal/httpapi/http.go`. Keeping the mapping identical is the point
 * of this mock: the UI branches on status (`403` ⇒ "scoreboard not visible",
 * `401` ⇒ sign in), so a mock that returned 200-with-empty would hide bugs.
 */
export const STATUS_FOR: Record<ApiErrorCode, number> = {
  unauthorized: 401,
  forbidden: 403,
  unverified: 403,
  not_found: 404,
  conflict: 409,
  invalid: 400,
  too_many: 429,
  gone: 410,
  not_acceptable: 406,
  unavailable: 503,
  internal: 500,
}

/** A failure shaped exactly like `{"error":{"code","message"}}`. */
export class MockError extends Error {
  readonly code: ApiErrorCode
  readonly status: number

  constructor(code: ApiErrorCode, message: string) {
    super(message)
    this.name = 'MockError'
    this.code = code
    this.status = STATUS_FOR[code]
  }

  toBody() {
    return { error: { code: this.code, message: this.message } }
  }
}

export const unauthorized = (message = 'not signed in') => new MockError('unauthorized', message)
export const forbidden = (message = 'forbidden') => new MockError('forbidden', message)
export const notFound = (message = 'not found') => new MockError('not_found', message)
export const conflict = (message: string) => new MockError('conflict', message)
export const invalid = (message: string) => new MockError('invalid', message)
export const unverified = () => new MockError('unverified', 'email is not verified')
