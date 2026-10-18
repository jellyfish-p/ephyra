import { canManageOrg, orgRole, requireAccount, requireOrg } from '../access'
import { conflict, forbidden, invalid, notFound } from '../errors'
import { nextId } from '../ids'
import { json } from '../types'
import type { MockOrg, MockRoute } from '../types'

const orgDTO = (org: MockOrg) => ({ id: org.id, slug: org.slug, name: org.name })

export const identityRoutes: MockRoute[] = [
  {
    method: 'GET',
    pattern: '/api/organizations',
    handler: (ctx) => {
      requireAccount(ctx)
      const orgs = [...ctx.state.orgs].sort((a, b) => (a.slug < b.slug ? -1 : 1)).map(orgDTO)
      return json({ organizations: orgs })
    },
  },
  {
    method: 'POST',
    pattern: '/api/organizations',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      if (!account.siteAdmin) {
        throw forbidden('only site admins can create organizations')
      }
      const raw = (ctx.body ?? {}) as Record<string, unknown>
      const slug = String(raw.slug ?? '')
        .trim()
        .toLowerCase()
      const name = String(raw.name ?? '').trim()
      if (!slug || !name) throw invalid('slug and name are required')
      if (ctx.state.orgs.some((o) => o.slug === slug)) throw conflict('slug already used')

      const org: MockOrg = { id: nextId(ctx.state.idCounters, 'org'), slug, name }
      ctx.state.orgs.push(org)
      ctx.state.orgMembers.push({ orgId: org.id, accountId: account.id, role: 'owner' })

      // `createOrg` ensures default routing, so a new organization can judge.
      const systemPool = ctx.state.pools.find((p) => p.kind === 'system')
      if (systemPool && !ctx.state.routing.some((r) => r.orgId === org.id)) {
        ctx.state.routing.push({ orgId: org.id, poolId: systemPool.id, priority: 10 })
      }
      return json(orgDTO(org), 201)
    },
  },
  {
    method: 'GET',
    pattern: '/api/me/organizations',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const rows = ctx.state.orgMembers
        .filter((m) => m.accountId === account.id)
        .map((m) => {
          const org = ctx.state.orgs.find((o) => o.id === m.orgId)!
          return { ...orgDTO(org), role: m.role, member: true }
        })
        .sort((a, b) => (a.slug < b.slug ? -1 : 1))
      return json({ organizations: rows })
    },
  },
  {
    method: 'GET',
    pattern: '/api/organizations/:id',
    handler: (ctx) => {
      requireAccount(ctx)
      return json(orgDTO(requireOrg(ctx.state, ctx.params.id!)))
    },
  },
  {
    method: 'GET',
    pattern: '/api/organizations/:id/membership',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const org = requireOrg(ctx.state, ctx.params.id!)
      const role = orgRole(ctx.state, org.id, account.id)
      return json(
        role
          ? { organization_id: org.id, member: true, role }
          : { organization_id: org.id, member: false },
      )
    },
  },
  {
    method: 'GET',
    pattern: '/api/organizations/:id/members',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const org = requireOrg(ctx.state, ctx.params.id!)
      const role = orgRole(ctx.state, org.id, account.id)
      if (!account.siteAdmin && !role) {
        throw forbidden('not an organization member')
      }
      const members = ctx.state.orgMembers
        .filter((m) => m.orgId === org.id)
        .map((m) => {
          const acc = ctx.state.accounts.find((a) => a.id === m.accountId)!
          return { account_id: m.accountId, display_name: acc.displayName, role: m.role }
        })
      return json({ members })
    },
  },
  {
    method: 'POST',
    pattern: '/api/organizations/:id/join',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const org = requireOrg(ctx.state, ctx.params.id!)
      if (orgRole(ctx.state, org.id, account.id)) throw conflict('already a member')
      if (
        ctx.state.joinRequests.some(
          (r) => r.orgId === org.id && r.accountId === account.id && r.status === 'pending',
        )
      ) {
        throw conflict('request already pending')
      }
      ctx.state.joinRequests.push({
        id: nextId(ctx.state.idCounters, 'jreq'),
        orgId: org.id,
        accountId: account.id,
        status: 'pending',
        createdAt: ctx.now,
      })
      return json({ ok: true }, 202)
    },
  },
  {
    method: 'GET',
    pattern: '/api/organizations/:id/join-requests',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const org = requireOrg(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, org.id, account)) {
        throw forbidden('organization manager role required')
      }
      const join_requests = ctx.state.joinRequests
        .filter((r) => r.orgId === org.id && r.status === 'pending')
        .map((r) => {
          const acc = ctx.state.accounts.find((a) => a.id === r.accountId)!
          return {
            id: r.id,
            account_id: r.accountId,
            display_name: acc.displayName,
            status: r.status,
            created_at: r.createdAt.toISOString(),
          }
        })
      return json({ join_requests })
    },
  },
  {
    method: 'POST',
    pattern: '/api/organizations/:id/join-requests/:requestId/decision',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const org = requireOrg(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, org.id, account)) {
        throw forbidden('organization manager role required')
      }
      const request = ctx.state.joinRequests.find((r) => r.id === ctx.params.requestId)
      if (!request) throw notFound('request not found')
      if (request.status !== 'pending') throw conflict('request already decided')
      const approve = (ctx.body as { approve?: boolean } | undefined)?.approve === true
      request.status = approve ? 'approved' : 'rejected'
      if (approve) {
        if (orgRole(ctx.state, org.id, request.accountId)) throw conflict('already a member')
        ctx.state.orgMembers.push({
          orgId: org.id,
          accountId: request.accountId,
          role: 'member',
        })
      }
      return json({ ok: true })
    },
  },
]
