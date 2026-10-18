import { clearSession, issueCsrfToken, issueSession } from '../cookies'
import { conflict, invalid, unauthorized } from '../errors'
import { nextId } from '../ids'
import { json } from '../types'
import type { MockAccount, MockContext, MockRoute } from '../types'
import { requireAccount } from '../access'

/** Mirrors `Account` / the login response. */
function accountDTO(a: MockAccount) {
  return {
    account_id: a.id,
    email: a.email,
    display_name: a.displayName,
    email_verified: a.emailVerified,
    site_admin: a.siteAdmin,
  }
}

function body(ctx: MockContext): Record<string, unknown> {
  return (ctx.body ?? {}) as Record<string, unknown>
}

function str(ctx: MockContext, key: string): string {
  const value = body(ctx)[key]
  return typeof value === 'string' ? value.trim() : ''
}

/** The backend mails a bare token; there is no mailer here. */
function issueToken(ctx: MockContext, account: MockAccount, kind: 'verify' | 'reset'): string {
  const token = `mock-${kind}-${nextId(ctx.state.idCounters, 'tokn')}`
  ctx.state.tokens = ctx.state.tokens.filter((t) => t.accountId !== account.id || t.kind !== kind)
  ctx.state.tokens.push({
    token,
    accountId: account.id,
    kind,
    expiresAt: new Date(ctx.now.getTime() + 24 * 3600_000),
  })
  // Visible in the browser console, and through `GET /api/_mock/tokens`.
  console.info(`[ephyra mock] ${kind} token for ${account.email}: ${token}`)
  return token
}

export const authRoutes: MockRoute[] = [
  {
    method: 'GET',
    pattern: '/api/auth/csrf',
    handler: (ctx) => {
      issueCsrfToken()
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/auth/register',
    handler: (ctx) => {
      const email = str(ctx, 'email')
      const password = typeof body(ctx).password === 'string' ? String(body(ctx).password) : ''
      const displayName = str(ctx, 'display_name')
      if (!email || !displayName || password.length < 8) {
        throw invalid('email, display name, and a password of at least 8 characters are required')
      }
      if (ctx.state.accounts.some((a) => a.email.toLowerCase() === email.toLowerCase())) {
        throw conflict('email already registered')
      }
      const account: MockAccount = {
        id: nextId(ctx.state.idCounters, 'acct'),
        email,
        password,
        displayName,
        emailVerified: false,
        siteAdmin: false,
      }
      ctx.state.accounts.push(account)
      issueToken(ctx, account, 'verify')
      return json({ ok: true }, 201)
    },
  },
  {
    method: 'POST',
    pattern: '/api/auth/login',
    handler: (ctx) => {
      const email = str(ctx, 'email').toLowerCase()
      const password = typeof body(ctx).password === 'string' ? String(body(ctx).password) : ''
      const account = ctx.state.accounts.find((a) => a.email.toLowerCase() === email)
      if (!account || account.password !== password) {
        throw unauthorized('invalid credentials')
      }
      issueSession(account.id)
      return json(accountDTO(account))
    },
  },
  {
    method: 'POST',
    pattern: '/api/auth/logout',
    handler: () => {
      clearSession()
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/auth/verify-email',
    handler: (ctx) => {
      const token = str(ctx, 'token')
      const row = ctx.state.tokens.find((t) => t.token === token && t.kind === 'verify')
      if (!row) throw invalid('invalid or expired token')
      if (row.expiresAt.getTime() < ctx.now.getTime()) {
        throw invalid('invalid or expired token')
      }
      const account = ctx.state.accounts.find((a) => a.id === row.accountId)
      if (!account) throw invalid('invalid or expired token')
      account.emailVerified = true
      ctx.state.tokens = ctx.state.tokens.filter((t) => t.token !== token)
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/auth/forgot-password',
    handler: (ctx) => {
      const email = str(ctx, 'email').toLowerCase()
      const account = ctx.state.accounts.find((a) => a.email.toLowerCase() === email)
      // The real endpoint never reveals whether the address exists.
      if (account) issueToken(ctx, account, 'reset')
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/auth/reset-password',
    handler: (ctx) => {
      const token = str(ctx, 'token')
      const password = typeof body(ctx).password === 'string' ? String(body(ctx).password) : ''
      const row = ctx.state.tokens.find((t) => t.token === token && t.kind === 'reset')
      if (!row || row.expiresAt.getTime() < ctx.now.getTime()) {
        throw invalid('invalid or expired token')
      }
      if (password.length < 8) {
        throw invalid('password must be at least 8 characters')
      }
      const account = ctx.state.accounts.find((a) => a.id === row.accountId)
      if (!account) throw invalid('invalid or expired token')
      account.password = password
      ctx.state.tokens = ctx.state.tokens.filter((t) => t.token !== token)
      return json({ ok: true })
    },
  },
  {
    method: 'GET',
    pattern: '/api/me',
    handler: (ctx) => json(accountDTO(requireAccount(ctx))),
  },
]
