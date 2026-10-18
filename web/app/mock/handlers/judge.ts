import {
  canManageOrg,
  findContestProblem,
  requireAccount,
  requireContest,
  requireOrg,
} from '../access'
import { conflict, forbidden, invalid, notFound } from '../errors'
import { nextId } from '../ids'
import { dispatchTask, judgeDepsOf } from '../judging'
import { isNormalTerminal } from '../scoring'
import { json } from '../types'
import type { MockContext, MockNode, MockPool, MockRoute, MockTask } from '../types'

/**
 * Judge infrastructure administration.
 *
 * A node's plaintext credential is returned only by `POST /api/pools/:id/nodes`
 * and `POST /api/nodes/:id/rotate`; every other response goes through `nodeDTO`,
 * which has no credential field at all.
 */

function nodeDTO(node: MockNode) {
  return {
    id: node.id,
    pool_id: node.poolId,
    name: node.name,
    enabled: node.enabled,
    connected: node.connected,
    revoked: node.revoked,
    max_slots: node.maxSlots,
    last_heartbeat_at: node.lastHeartbeatAt?.toISOString() ?? null,
    advertised_profiles: node.advertisedProfiles,
  }
}

function poolDTO(pool: MockPool) {
  return {
    id: pool.id,
    kind: pool.kind,
    organization_id: pool.orgId,
    name: pool.name,
    enabled: pool.enabled,
  }
}

/** Port of `judging.authorizePool`: site admin for system pools, manage otherwise. */
function requirePoolAccess(ctx: MockContext, pool: MockPool): void {
  const account = requireAccount(ctx)
  if (pool.kind === 'system') {
    if (!account.siteAdmin) throw forbidden('only site admins can manage system pools')
    return
  }
  if (!canManageOrg(ctx.state, pool.orgId ?? '', account)) {
    throw forbidden('organization manager role required')
  }
}

function requirePool(ctx: MockContext, id: string): MockPool {
  const pool = ctx.state.pools.find((p) => p.id === id)
  if (!pool) throw notFound('pool not found')
  return pool
}

function requireTask(ctx: MockContext, id: string): MockTask {
  const task = ctx.state.tasks.find((t) => t.id === id)
  if (!task) throw notFound('task not found')
  return task
}

function taskDTO(ctx: MockContext, task: MockTask) {
  const attempts = ctx.state.attempts
    .filter((a) => a.taskId === task.id)
    .sort((a, b) => b.generation - a.generation)
  const latest = attempts[0]
  return {
    id: task.id,
    submission_id: task.submissionId,
    state: task.state,
    purpose: task.purpose,
    revision_id: task.revisionId,
    created_at: task.createdAt.toISOString(),
    // Dispatch bookkeeping only; the raw judge result is never part of this view.
    attempt: latest
      ? {
          id: latest.id,
          generation: latest.generation,
          node_id: latest.nodeId,
          pool_id: latest.poolId,
          state: latest.state,
          dispatched_at: latest.dispatchedAt.toISOString(),
          lease_until: latest.leaseUntil.toISOString(),
          finished_at: latest.finishedAt?.toISOString() ?? null,
          system_fault: latest.systemFault,
        }
      : null,
  }
}

export const judgeRoutes: MockRoute[] = [
  {
    method: 'GET',
    pattern: '/api/organizations/:id/pools',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const org = requireOrg(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, org.id, account)) {
        throw forbidden('organization manager role required')
      }
      // A site admin also sees the system pools, like `judging.ListPools`.
      const pools = ctx.state.pools
        .filter((p) => p.orgId === org.id || (account.siteAdmin && p.kind === 'system'))
        .map(poolDTO)
      return json({ pools })
    },
  },
  {
    method: 'POST',
    pattern: '/api/pools',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const raw = (ctx.body ?? {}) as Record<string, unknown>
      const kind = raw.kind === 'system' ? 'system' : 'organization'
      const name = String(raw.name ?? '').trim()
      if (kind === 'system' && !account.siteAdmin) {
        throw forbidden('only site admins can create system pools')
      }
      const orgId = typeof raw.organization_id === 'string' ? raw.organization_id : null
      if (kind === 'organization' && !orgId) {
        throw invalid('organization pool requires organization id')
      }
      if (kind === 'organization' && !canManageOrg(ctx.state, orgId!, account)) {
        throw forbidden('organization manager role required')
      }
      const pool: MockPool = {
        id: nextId(ctx.state.idCounters, 'pool'),
        kind,
        orgId: kind === 'system' ? null : orgId,
        name,
        enabled: true,
      }
      ctx.state.pools.push(pool)
      return json(poolDTO(pool), 201)
    },
  },
  {
    method: 'GET',
    pattern: '/api/pools/:id/nodes',
    handler: (ctx) => {
      const pool = requirePool(ctx, ctx.params.id!)
      requirePoolAccess(ctx, pool)
      const nodes = ctx.state.nodes.filter((n) => n.poolId === pool.id).map(nodeDTO)
      return json({ nodes })
    },
  },
  {
    method: 'POST',
    pattern: '/api/pools/:id/nodes',
    handler: (ctx) => {
      const pool = requirePool(ctx, ctx.params.id!)
      requirePoolAccess(ctx, pool)
      const name = String((ctx.body as { name?: string } | undefined)?.name ?? '').trim()
      if (!name) throw invalid('name is required')
      const node: MockNode = {
        id: nextId(ctx.state.idCounters, 'node'),
        poolId: pool.id,
        name,
        enabled: true,
        connected: false,
        revoked: false,
        maxSlots: 1,
        lastHeartbeatAt: null,
        advertisedProfiles: [],
        credential: `mock-credential-${nextId(ctx.state.idCounters, 'cred')}`,
      }
      ctx.state.nodes.push(node)
      // The only response that ever carries the plaintext credential.
      return json({ id: node.id, credential: node.credential }, 201)
    },
  },
  {
    method: 'POST',
    pattern: '/api/nodes/:id/rotate',
    handler: (ctx) => {
      const node = ctx.state.nodes.find((n) => n.id === ctx.params.id)
      if (!node) throw notFound('node not found')
      requirePoolAccess(ctx, requirePool(ctx, node.poolId))
      node.credential = `mock-credential-${nextId(ctx.state.idCounters, 'cred')}`
      return json({ credential: node.credential })
    },
  },
  {
    method: 'POST',
    pattern: '/api/nodes/:id/revoke',
    handler: (ctx) => {
      const node = ctx.state.nodes.find((n) => n.id === ctx.params.id)
      if (!node) throw notFound('node not found')
      requirePoolAccess(ctx, requirePool(ctx, node.poolId))
      node.revoked = true
      node.enabled = false
      node.connected = false
      return json({ ok: true })
    },
  },
  {
    method: 'GET',
    pattern: '/api/organizations/:id/routing',
    handler: (ctx) => {
      const org = requireOrg(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, org.id, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const entries = ctx.state.routing
        .filter((r) => r.orgId === org.id)
        .sort((a, b) => a.priority - b.priority || (a.poolId < b.poolId ? -1 : 1))
        .map((r) => ({ pool_id: r.poolId, priority: r.priority }))
      return json({ entries })
    },
  },
  {
    method: 'POST',
    pattern: '/api/organizations/:id/routing',
    handler: (ctx) => {
      const org = requireOrg(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, org.id, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const raw = (ctx.body ?? {}) as { entries?: { pool_id?: string; priority?: number }[] }
      const entries = Array.isArray(raw.entries) ? raw.entries : []
      for (const entry of entries) {
        if (!entry?.pool_id || !ctx.state.pools.some((p) => p.id === entry.pool_id)) {
          throw invalid('unknown pool in routing')
        }
      }
      // A new policy version replaces the old one wholesale.
      ctx.state.routing = ctx.state.routing.filter((r) => r.orgId !== org.id)
      for (const entry of entries) {
        ctx.state.routing.push({
          orgId: org.id,
          poolId: entry.pool_id!,
          priority: Number(entry.priority ?? 0),
        })
      }
      return json({ ok: true })
    },
  },
  {
    method: 'GET',
    pattern: '/api/contests/:id/tasks',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, contest.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const state = typeof ctx.req.query.state === 'string' ? ctx.req.query.state : ''
      const rawLimit = Number(ctx.req.query.limit ?? 0)
      const limit = rawLimit > 0 ? Math.min(200, Math.max(1, rawLimit)) : 50
      const tasks = ctx.state.tasks
        .filter((t) => t.contestId === contest.id && (!state || t.state === state))
        .sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime())
        .slice(0, limit)
        .map((t) => taskDTO(ctx, t))
      return json({ tasks })
    },
  },
  {
    method: 'POST',
    pattern: '/api/tasks/:id/redispatch',
    handler: (ctx) => {
      const task = requireTask(ctx, ctx.params.id!)
      const contest = requireContest(ctx.state, task.contestId)
      if (!canManageOrg(ctx.state, contest.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      // `PlanRedispatch`: invalidate in-flight attempts, then re-schedule.
      for (const attempt of ctx.state.attempts) {
        if (attempt.taskId === task.id && attempt.state === 'dispatched') {
          attempt.state = 'invalidated'
        }
      }
      task.state = 'pending_schedule'
      dispatchTask(judgeDepsOf(ctx), task)
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/contest-problems/:id/rejudge',
    handler: (ctx) => {
      const cp = findContestProblem(ctx.state, ctx.params.id!)
      if (!cp) throw notFound('contest problem not found')
      const contest = requireContest(ctx.state, cp.contestId)
      if (!canManageOrg(ctx.state, contest.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const toRevisionId = String(
        (ctx.body as { revision_id?: string } | undefined)?.revision_id ?? '',
      )
      const revision = ctx.state.revisions.find(
        (r) => r.id === toRevisionId && r.problemId === cp.problemId,
      )
      if (!revision) throw notFound('revision not found')
      if (
        ctx.state.rejudgeBatches.some((b) => b.contestProblemId === cp.id && b.state === 'running')
      ) {
        throw conflict('an open rejudge batch already exists for this problem')
      }

      const batch = {
        id: nextId(ctx.state.idCounters, 'rjud'),
        contestProblemId: cp.id,
        fromRevisionId: cp.scoringRevisionId,
        toRevisionId,
        state: 'running' as const,
        createdAt: ctx.now,
      }
      ctx.state.rejudgeBatches.push(batch)
      // New submissions are judged on the target revision from now on, but the
      // scoreboard keeps using the scoring revision until the batch is switched.
      cp.targetRevisionId = toRevisionId

      const snapshot = ctx.state.submissions
        .filter((s) => s.contestProblemId === cp.id)
        .sort((a, b) => a.receiveSeq - b.receiveSeq)
      const deps = judgeDepsOf(ctx)
      for (const submission of snapshot) {
        const task: MockTask = {
          id: nextId(ctx.state.idCounters, 'task'),
          submissionId: submission.id,
          contestId: submission.contestId,
          revisionId: toRevisionId,
          profile: 'cpp23.gcc.linux-amd64.v0',
          purpose: 'rejudge',
          state: 'pending_schedule',
          createdAt: ctx.now,
          batchId: batch.id,
        }
        ctx.state.tasks.push(task)
        dispatchTask(deps, task)
      }
      return json({ batch_id: batch.id, state: batch.state }, 202)
    },
  },
  {
    method: 'POST',
    pattern: '/api/rejudge-batches/:id/switch',
    handler: (ctx) => {
      const batch = ctx.state.rejudgeBatches.find((b) => b.id === ctx.params.id)
      if (!batch) throw notFound('batch not found')
      const cp = findContestProblem(ctx.state, batch.contestProblemId)
      if (!cp) throw notFound('contest problem not found')
      const contest = requireContest(ctx.state, cp.contestId)
      if (!canManageOrg(ctx.state, contest.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      // Every item must have landed a normal, non-faulted result on the new
      // revision; switching early would mix two revisions on one board.
      const items = ctx.state.tasks.filter((t) => t.batchId === batch.id)
      for (const task of items) {
        const submission = ctx.state.submissions.find((s) => s.id === task.submissionId)
        const result = submission?.results[batch.toRevisionId]
        if (!result || result.systemFault || !isNormalTerminal(result.verdict)) {
          throw conflict('batch is not ready to switch')
        }
      }
      cp.scoringRevisionId = batch.toRevisionId
      batch.state = 'switched'
      return json({ ok: true })
    },
  },
]
