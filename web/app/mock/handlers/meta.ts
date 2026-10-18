import { completeTask, type JudgeDeps } from '../judging'
import { json } from '../types'
import type { MockRoute } from '../types'

/**
 * Mock-only control endpoints, under a reserved `/api/_mock` prefix so they can
 * never collide with a real route.
 *
 * These exist because two things the real backend does are invisible in a
 * browser-only mock: it sends email (so the token-paste flows would have no
 * token to paste), and it judges asynchronously (so a pending submission would
 * sit there for the full delay). Both are exposed here rather than by bending a
 * real response shape to carry mock-only fields.
 */
export const metaRoutes: MockRoute[] = [
  {
    method: 'GET',
    pattern: '/api/_mock/state',
    handler: (ctx) =>
      json({
        now: ctx.now.toISOString(),
        accounts: ctx.state.accounts.map((a) => ({
          id: a.id,
          email: a.email,
          display_name: a.displayName,
          email_verified: a.emailVerified,
          site_admin: a.siteAdmin,
          password: a.password,
        })),
        contests: ctx.state.contests.map((c) => ({
          id: c.id,
          slug: c.slug,
          title: c.title,
          rule: c.rule,
          starts_at: c.startsAt.toISOString(),
          ends_at: c.endsAt.toISOString(),
          published_at: c.publishedAt?.toISOString() ?? null,
        })),
        organizations: ctx.state.orgs.map((o) => ({
          id: o.id,
          slug: o.slug,
          name: o.name,
        })),
        problems: ctx.state.problems.map((p) => ({
          id: p.id,
          slug: p.slug,
          title: p.title,
          organization_id: p.orgId,
        })),
        pools: ctx.state.pools.map((p) => ({
          id: p.id,
          name: p.name,
          kind: p.kind,
          organization_id: p.orgId,
        })),
        pending_tasks: ctx.state.tasks.filter((t) => t.state !== 'completed').length,
        pending_submissions: [
          ...new Set(
            ctx.state.tasks.filter((t) => t.state !== 'completed').map((t) => t.submissionId),
          ),
        ],
      }),
  },
  {
    method: 'GET',
    pattern: '/api/_mock/tokens',
    handler: (ctx) =>
      json({
        tokens: ctx.state.tokens.map((t) => ({
          token: t.token,
          kind: t.kind,
          email: ctx.state.accounts.find((a) => a.id === t.accountId)?.email ?? '',
          expires_at: t.expiresAt.toISOString(),
        })),
      }),
  },
  {
    method: 'POST',
    pattern: '/api/_mock/judge/:submissionId/complete',
    handler: (ctx) => {
      const submission = ctx.state.submissions.find((s) => s.id === ctx.params.submissionId)
      if (!submission) return json({ ok: false, reason: 'unknown submission' }, 404)
      const deps: JudgeDeps = {
        state: ctx.state,
        now: ctx.runtime.now,
        schedule: ctx.runtime.schedule,
        publish: ctx.runtime.publish,
      }
      let completed = 0
      for (const task of ctx.state.tasks) {
        if (task.submissionId !== submission.id || task.state === 'completed') continue
        const attempt = ctx.state.attempts
          .filter((a) => a.taskId === task.id && a.state === 'dispatched')
          .sort((a, b) => b.generation - a.generation)[0]
        if (!attempt) continue
        completeTask(deps, task, attempt)
        completed += 1
      }
      return json({ ok: true, completed })
    },
  },
  {
    method: 'POST',
    pattern: '/api/_mock/reset',
    handler: (ctx) => {
      ctx.runtime.reset()
      return json({ ok: true })
    },
  },
]
