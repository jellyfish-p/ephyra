import { canManageOrg, requireAccount, requireOrg } from '../access'
import { forbidden, invalid, notFound } from '../errors'
import { nextId } from '../ids'
import { json } from '../types'
import type { MockContext, MockProblem, MockRevision, MockRoute } from '../types'

/** `CanViewProblemBank`: a site admin or any organization member. */
function requireBankAccess(ctx: MockContext, orgId: string): void {
  const account = requireAccount(ctx)
  if (
    !account.siteAdmin &&
    !ctx.state.orgMembers.some((m) => m.orgId === orgId && m.accountId === account.id)
  ) {
    throw forbidden('not an organization member')
  }
}

/** Reads the uploaded zip's name; the mock does not unpack archives. */
async function uploadName(ctx: MockContext, field: string): Promise<string> {
  if (typeof FormData !== 'undefined' && ctx.body instanceof FormData) {
    const file = ctx.body.get(field)
    if (file && typeof file === 'object' && 'name' in file) {
      return String((file as File).name || '')
    }
  }
  return ''
}

function slugify(name: string): string {
  return name
    .replace(/\.zip$/i, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function placeholderStatement(title: string, slug: string): string {
  return `# ${title}

This problem was imported into the mock from \`${slug}.zip\`.

The mock does not unpack archives, so the real statement is not available. Point
the frontend at the Go backend to see the actual problem text.
`
}

function newRevision(
  ctx: MockContext,
  problemId: string,
  n: number,
  statement: string | null,
): MockRevision {
  const revision: MockRevision = {
    id: nextId(ctx.state.idCounters, 'rev'),
    problemId,
    n,
    createdAt: ctx.now,
    statement,
    kind: 'batch',
    checkerKind: 'token',
    timeLimitMs: 2000,
    memoryLimitBytes: 268_435_456,
    subtasks: [
      { id: '1', score: '30.00' },
      { id: '2', score: '30.00' },
      { id: '3', score: '40.00' },
    ],
  }
  ctx.state.revisions.push(revision)
  return revision
}

export const problemRoutes: MockRoute[] = [
  {
    method: 'GET',
    pattern: '/api/organizations/:id/problems',
    handler: (ctx) => {
      const org = requireOrg(ctx.state, ctx.params.id!)
      requireBankAccess(ctx, org.id)
      const problems = ctx.state.problems
        .filter((p) => p.orgId === org.id)
        .sort((a, b) => (a.slug < b.slug ? -1 : 1))
        .map((p) => ({ id: p.id, slug: p.slug, title: p.title }))
      return json({ problems })
    },
  },
  {
    method: 'POST',
    pattern: '/api/organizations/:id/problems/import',
    handler: async (ctx) => {
      const org = requireOrg(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, org.id, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const name = await uploadName(ctx, 'zip')
      if (!name) throw invalid('zip is required')

      const slug = slugify(name) || `imported-${ctx.state.problems.length + 1}`
      const title = slug
        .split('-')
        .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
        .join(' ')
      const problem: MockProblem = {
        id: nextId(ctx.state.idCounters, 'pbm'),
        orgId: org.id,
        slug,
        title,
      }
      ctx.state.problems.push(problem)
      const revision = newRevision(ctx, problem.id, 1, placeholderStatement(title, slug))
      return json({ problem_id: problem.id, revision_id: revision.id, revision_n: revision.n }, 201)
    },
  },
  {
    method: 'GET',
    pattern: '/api/problems/:id/revisions',
    handler: (ctx) => {
      const problem = ctx.state.problems.find((p) => p.id === ctx.params.id)
      if (!problem) throw notFound('problem not found')
      if (!canManageOrg(ctx.state, problem.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const revisions = ctx.state.revisions
        .filter((r) => r.problemId === problem.id)
        .sort((a, b) => b.n - a.n)
        .map((r) => ({
          revision_id: r.id,
          revision_n: r.n,
          created_at: r.createdAt.toISOString(),
        }))
      return json({ revisions })
    },
  },
  {
    method: 'POST',
    pattern: '/api/problems/:id/revisions',
    handler: async (ctx) => {
      const problem = ctx.state.problems.find((p) => p.id === ctx.params.id)
      if (!problem) throw notFound('problem not found')
      if (!canManageOrg(ctx.state, problem.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const name = await uploadName(ctx, 'zip')
      if (!name) throw invalid('zip is required')
      const nextN =
        ctx.state.revisions
          .filter((r) => r.problemId === problem.id)
          .reduce((max, r) => Math.max(max, r.n), 0) + 1
      const revision = newRevision(
        ctx,
        problem.id,
        nextN,
        placeholderStatement(problem.title, slugify(name)),
      )
      return json({ revision_id: revision.id, revision_n: revision.n }, 201)
    },
  },
]
