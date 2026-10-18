import {
  audienceForContest,
  canManageOrg,
  canViewContest,
  canViewContestProblems,
  contestAccess,
  contestPhaseOf,
  contestantLettersOnly,
  entrantMembers,
  requireAccount,
  requireContest,
  requireEntrant,
  requireOrg,
  sourceVisibilityLocked,
  viewerEntrantId,
  type ContestAccess,
} from '../access'
import { conflict, forbidden, invalid, notFound, unverified } from '../errors'
import { nextId } from '../ids'
import {
  acmFrozen,
  contestInputFrom,
  filterFrozenStandings,
  scoreContest,
  visible,
  type VisibilityInput,
} from '../scoring'
import { json } from '../types'
import type { MockContext, MockContest, MockEntrant, MockRoute, MockState } from '../types'

/* --------------------------------------------------------------- shaping */

function contestDTO(state: MockState, contest: MockContest, now: Date) {
  return {
    id: contest.id,
    organization_id: contest.orgId,
    slug: contest.slug,
    title: contest.title,
    rule: contest.rule,
    entry_mode: contest.entryMode,
    starts_at: contest.startsAt.toISOString(),
    ends_at: contest.endsAt.toISOString(),
    register_starts_at: contest.registerStartsAt.toISOString(),
    allow_external: contest.allowExternal,
    source_public_after: contest.sourcePublicAfter,
    source_visibility_locked: sourceVisibilityLocked(contest, now),
    published_at: contest.publishedAt?.toISOString() ?? null,
  }
}

function problemListDTO(state: MockState, contestId: string, access: ContestAccess) {
  const lettersOnly = contestantLettersOnly(access)
  return state.contestProblems
    .filter((cp) => cp.contestId === contestId)
    .sort((a, b) => (a.letter < b.letter ? -1 : 1))
    .map((cp) => {
      const row: Record<string, unknown> = { id: cp.id, letter: cp.letter }
      if (lettersOnly) return row
      const problem = state.problems.find((p) => p.id === cp.problemId)
      row.title = problem?.title ?? ''
      row.problem_id = cp.problemId
      if (access.orgManage || access.siteAdmin) {
        row.scoring_revision_id = cp.scoringRevisionId
        row.target_revision_id = cp.targetRevisionId
      }
      return row
    })
}

function entrantDTO(state: MockState, entrant: MockEntrant) {
  return {
    id: entrant.id,
    contest_id: entrant.contestId,
    kind: entrant.kind,
    display_name: entrant.displayName,
    status: entrant.status,
    registered_at: entrant.registeredAt.toISOString(),
    accepted_at: entrant.acceptedAt?.toISOString() ?? null,
    roster_frozen_at: entrant.rosterFrozenAt?.toISOString() ?? null,
    members: entrantMembers(state, entrant.id).map((m) => {
      const account = state.accounts.find((a) => a.id === m.accountId)!
      return {
        account_id: m.accountId,
        display_name: account.displayName,
        role: m.role,
        confirmed: m.confirmedAt !== null,
      }
    }),
  }
}

/** The contest-level visibility input, used for the scoreboard decision. */
function contestVisibilityInput(
  state: MockState,
  contest: MockContest,
  account: MockContext['account'],
  now: Date,
): VisibilityInput {
  return {
    rule: contest.rule,
    phase: contestPhaseOf(contest, now),
    now,
    endsAt: contest.endsAt,
    freezeFor: contest.freezeMinutesBeforeEnd ?? 0,
    granularity: contest.caseFeedback,
    ioiPublicBoard: contest.ioiPublicScoreboard,
    sourcePublic: contest.sourcePublicAfter,
    published: contest.publishedAt !== null,
    audience: audienceForContest(state, contest, account),
    submissionRevision: '',
    scoringRevision: '',
  }
}

/* ------------------------------------------------------------- register */

function requireRegistrationOpen(ctx: MockContext, contest: MockContest, accountId: string) {
  const account = requireAccount(ctx)
  if (!account.emailVerified) throw unverified()
  const open =
    ctx.now.getTime() >= contest.registerStartsAt.getTime() &&
    ctx.now.getTime() < contest.endsAt.getTime()
  if (!open) throw conflict('registration is closed')
  const member = ctx.state.orgMembers.some(
    (m) => m.orgId === contest.orgId && m.accountId === accountId,
  )
  if (!member && !contest.allowExternal) {
    throw forbidden('this contest does not allow external contestants')
  }
}

function requireCaptain(state: MockState, entrantId: string, accountId: string) {
  const membership = state.entrantMembers.find(
    (m) => m.entrantId === entrantId && m.accountId === accountId,
  )
  if (!membership) throw forbidden('not a team member')
  if (membership.role !== 'captain') throw forbidden('only the captain can do this')
}

function teamSizeOK(contest: MockContest, size: number) {
  if (size < 1) throw invalid('team must have at least one member')
  if (size > contest.maxTeamSize) throw invalid('team exceeds max size')
}

/* ----------------------------------------------------------------- routes */

export const contestRoutes: MockRoute[] = [
  {
    method: 'GET',
    pattern: '/api/organizations/:id/contests',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const org = requireOrg(ctx.state, ctx.params.id!)
      const all = ctx.state.contests.filter((c) => c.orgId === org.id)
      const member =
        account.siteAdmin ||
        ctx.state.orgMembers.some((m) => m.orgId === org.id && m.accountId === account.id)
      const visibleContests = member
        ? all
        : all.filter((c) => {
            const access = contestAccess(ctx.state, c, account)
            return canViewContest(c, true, access)
          })
      if (!member && visibleContests.length === 0) {
        throw forbidden('not an organization member')
      }
      return json({
        contests: visibleContests
          .sort((a, b) => a.startsAt.getTime() - b.startsAt.getTime())
          .map((c) => contestDTO(ctx.state, c, ctx.now)),
      })
    },
  },
  {
    method: 'POST',
    pattern: '/api/organizations/:id/contests',
    handler: (ctx) => {
      const org = requireOrg(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, org.id, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const raw = (ctx.body ?? {}) as Record<string, unknown>
      const slug = String(raw.slug ?? '').trim()
      const title = String(raw.title ?? '').trim()
      const rule = String(raw.rule ?? '')
      const entryMode = String(raw.entry_mode ?? '')
      if (!slug || !title) throw invalid('slug and title are required')
      if (rule !== 'acm' && rule !== 'oi' && rule !== 'ioi') {
        throw invalid('invalid rule or entry mode')
      }
      if (entryMode !== 'individual' && entryMode !== 'team') {
        throw invalid('invalid rule or entry mode')
      }
      const parseTime = (key: string): Date => {
        const value = raw[key]
        const date = typeof value === 'string' ? new Date(value) : new Date(Number.NaN)
        if (Number.isNaN(date.getTime())) throw invalid(`${key} is required`)
        return date
      }
      const contest: MockContest = {
        id: nextId(ctx.state.idCounters, 'ctst'),
        orgId: org.id,
        slug,
        title,
        rule,
        entryMode,
        startsAt: parseTime('starts_at'),
        endsAt: parseTime('ends_at'),
        registerStartsAt: parseTime('register_starts_at'),
        allowExternal: raw.allow_external === true,
        registrationRequiresReview: raw.registration_requires_review === true,
        maxTeamSize: Number(raw.max_team_size ?? 1) || 1,
        acmPenaltyMinutes: Number(raw.acm_penalty_minutes ?? 20),
        freezeMinutesBeforeEnd:
          raw.freeze_minutes_before_end === null || raw.freeze_minutes_before_end === undefined
            ? null
            : Number(raw.freeze_minutes_before_end),
        ioiPublicScoreboard: raw.ioi_public_scoreboard === true,
        caseFeedback: raw.case_feedback === 'case' ? 'case' : 'subtask',
        sourcePublicAfter: raw.source_public_after === true,
        publishedAt: null,
        sourceVisibilityLocked: false,
      }
      ctx.state.contests.push(contest)
      return json(contestDTO(ctx.state, contest, ctx.now), 201)
    },
  },
  {
    method: 'GET',
    pattern: '/api/contests/:id',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      const access = contestAccess(ctx.state, contest, ctx.account)
      if (!canViewContest(contest, ctx.account !== null, access)) {
        throw ctx.account ? forbidden('cannot view this contest') : forbidden('not signed in')
      }
      return json(contestDTO(ctx.state, contest, ctx.now))
    },
  },
  {
    method: 'PATCH',
    pattern: '/api/contests/:id/source-public',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, contest.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const publicAfter =
        (ctx.body as { source_public_after?: boolean } | undefined)?.source_public_after === true
      // Widening visibility after registration opened is refused outright.
      if (publicAfter && !contest.sourcePublicAfter && sourceVisibilityLocked(contest, ctx.now)) {
        throw conflict('cannot expand source visibility after registration opened')
      }
      contest.sourcePublicAfter = publicAfter
      return json({ ok: true })
    },
  },
  {
    method: 'GET',
    pattern: '/api/contests/:id/problems',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      const access = contestAccess(ctx.state, contest, ctx.account)
      if (!canViewContestProblems(access)) throw forbidden('cannot view contest problems')
      return json({ problems: problemListDTO(ctx.state, contest.id, access) })
    },
  },
  {
    method: 'GET',
    pattern: '/api/contests/:id/problems/:letter',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      const access = contestAccess(ctx.state, contest, ctx.account)
      if (!canViewContestProblems(access)) throw forbidden('cannot view contest problems')
      const cp = ctx.state.contestProblems.find(
        (c) => c.contestId === contest.id && c.letter === ctx.params.letter,
      )
      if (!cp) throw notFound('contest problem not found')
      const revision = ctx.state.revisions.find((r) => r.id === cp.scoringRevisionId)
      if (!revision) throw notFound('revision not found')
      const problem = ctx.state.problems.find((p) => p.id === cp.problemId)
      // Judge data is never part of this projection: only the statement text.
      return json({
        letter: cp.letter,
        title: problem?.title ?? '',
        problem_id: cp.problemId,
        scoring_revision_id: cp.scoringRevisionId,
        target_revision_id: cp.targetRevisionId,
        kind: revision.kind,
        checker_kind: revision.checkerKind,
        time_limit_ms: revision.timeLimitMs,
        memory_limit_bytes: revision.memoryLimitBytes,
        subtasks: revision.subtasks,
        statement_markdown: revision.statement,
      })
    },
  },
  {
    method: 'POST',
    pattern: '/api/contests/:id/problems',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, contest.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const raw = (ctx.body ?? {}) as Record<string, unknown>
      const problemId = String(raw.problem_id ?? '')
      const revisionId = String(raw.revision_id ?? '')
      const letter = String(raw.letter ?? '')
        .trim()
        .toUpperCase()
      const problem = ctx.state.problems.find((p) => p.id === problemId)
      if (!problem || problem.orgId !== contest.orgId) {
        throw notFound('problem not found in this organization')
      }
      const revision = ctx.state.revisions.find(
        (r) => r.id === revisionId && r.problemId === problemId,
      )
      if (!revision) throw notFound('revision not found')
      if (
        ctx.state.contestProblems.some((cp) => cp.contestId === contest.id && cp.letter === letter)
      ) {
        throw conflict('letter or problem already in contest')
      }
      const cp = {
        id: nextId(ctx.state.idCounters, 'cpbm'),
        contestId: contest.id,
        problemId,
        letter,
        scoringRevisionId: revisionId,
        targetRevisionId: revisionId,
      }
      ctx.state.contestProblems.push(cp)
      return json(
        { id: cp.id, letter: cp.letter, problem_id: cp.problemId, revision_id: revisionId },
        201,
      )
    },
  },
  {
    method: 'GET',
    pattern: '/api/contests/:id/standings',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      const access = contestAccess(ctx.state, contest, ctx.account)
      const cps = ctx.state.contestProblems.filter((cp) => cp.contestId === contest.id)
      const accepted = ctx.state.entrants.filter(
        (e) => e.contestId === contest.id && e.status === 'accepted',
      )
      const rows = scoreContest(
        contestInputFrom(
          contest,
          cps,
          accepted.map((e) => ({ id: e.id, displayName: e.displayName })),
          ctx.state.submissions.filter((s) => s.contestId === contest.id),
        ),
      )

      const input = contestVisibilityInput(ctx.state, contest, ctx.account, ctx.now)
      const admin = input.audience === 'org_admin'
      const view = visible(input)
      const self = input.audience === 'self'

      if (view.showOnBoard || admin) {
        if (contest.rule === 'acm' && acmFrozen(input) && !admin) {
          const receivedAt = (id: string) =>
            ctx.state.submissions.find((s) => s.id === id)?.receivedAt.getTime()
          const viewer = viewerEntrantId(ctx.state, contest.id, ctx.account)
          return json({
            standings: filterFrozenStandings(input, viewer ?? '', rows, receivedAt),
          })
        }
        return json({ standings: rows })
      }
      if (self) {
        const viewer = viewerEntrantId(ctx.state, contest.id, ctx.account)
        return json({ standings: rows.filter((r) => r.entrant_id === viewer) })
      }
      throw forbidden('scoreboard is not visible')
    },
  },
  {
    method: 'GET',
    pattern: '/api/contests/:id/entrants',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      if (!canManageOrg(ctx.state, contest.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const status = typeof ctx.req.query.status === 'string' ? ctx.req.query.status : ''
      const entrants = ctx.state.entrants
        .filter((e) => e.contestId === contest.id && (!status || e.status === status))
        .sort((a, b) => a.registeredAt.getTime() - b.registeredAt.getTime())
        .map((e) => entrantDTO(ctx.state, e))
      return json({ entrants })
    },
  },
  {
    method: 'GET',
    pattern: '/api/contests/:id/my-entrant',
    handler: (ctx) => {
      const contest = requireContest(ctx.state, ctx.params.id!)
      const access = contestAccess(ctx.state, contest, ctx.account)
      if (!canViewContest(contest, ctx.account !== null, access)) {
        throw forbidden('cannot view this contest')
      }
      const account = ctx.account
      if (!account) return json({ entrant: null })
      const membership = ctx.state.entrantMembers.find(
        (m) => m.contestId === contest.id && m.accountId === account.id,
      )
      if (!membership) return json({ entrant: null })
      const entrant = ctx.state.entrants.find((e) => e.id === membership.entrantId)
      if (!entrant) throw notFound('entrant not found')
      return json({ entrant: entrantDTO(ctx.state, entrant) })
    },
  },
  {
    method: 'POST',
    pattern: '/api/contests/:id/register',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const contest = requireContest(ctx.state, ctx.params.id!)
      if (contest.entryMode !== 'individual') throw conflict('contest uses team entry')
      requireRegistrationOpen(ctx, contest, account.id)
      const already = ctx.state.entrantMembers.some(
        (m) => m.contestId === contest.id && m.accountId === account.id,
      )
      if (already) throw conflict('already registered in this contest')
      const accepted = !contest.registrationRequiresReview
      const entrant: MockEntrant = {
        id: nextId(ctx.state.idCounters, 'entr'),
        contestId: contest.id,
        kind: 'individual',
        displayName: account.displayName,
        status: accepted ? 'accepted' : 'pending',
        registeredAt: ctx.now,
        acceptedAt: accepted ? ctx.now : null,
        rosterFrozenAt: accepted ? ctx.now : null,
      }
      ctx.state.entrants.push(entrant)
      ctx.state.entrantMembers.push({
        contestId: contest.id,
        entrantId: entrant.id,
        accountId: account.id,
        role: 'captain',
        confirmedAt: ctx.now,
      })
      return json({ entrant_id: entrant.id, status: entrant.status }, 201)
    },
  },
  {
    method: 'POST',
    pattern: '/api/contests/:id/teams',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const contest = requireContest(ctx.state, ctx.params.id!)
      if (contest.entryMode !== 'team') throw conflict('contest uses individual entry')
      const name = String((ctx.body as { name?: string } | undefined)?.name ?? '').trim()
      requireRegistrationOpen(ctx, contest, account.id)
      if (!name) throw invalid('team name is required')
      const already = ctx.state.entrantMembers.some(
        (m) => m.contestId === contest.id && m.accountId === account.id,
      )
      if (already) throw conflict('already registered in this contest')
      const entrant: MockEntrant = {
        id: nextId(ctx.state.idCounters, 'entr'),
        contestId: contest.id,
        kind: 'team',
        displayName: name,
        status: 'pending',
        registeredAt: ctx.now,
        acceptedAt: null,
        rosterFrozenAt: null,
      }
      ctx.state.entrants.push(entrant)
      ctx.state.entrantMembers.push({
        contestId: contest.id,
        entrantId: entrant.id,
        accountId: account.id,
        role: 'captain',
        confirmedAt: ctx.now,
      })
      return json({ entrant_id: entrant.id, status: entrant.status }, 201)
    },
  },
  {
    method: 'POST',
    pattern: '/api/entrants/:id/invite',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const entrant = requireEntrant(ctx.state, ctx.params.id!)
      const contest = requireContest(ctx.state, entrant.contestId)
      requireCaptain(ctx.state, entrant.id, account.id)
      if (entrant.acceptedAt && ctx.now.getTime() >= contest.startsAt.getTime()) {
        throw conflict('roster is frozen after start')
      }
      const targetId = String((ctx.body as { account_id?: string } | undefined)?.account_id ?? '')
      if (!ctx.state.accounts.some((a) => a.id === targetId)) throw notFound('account not found')
      const size = ctx.state.entrantMembers.filter((m) => m.entrantId === entrant.id).length
      teamSizeOK(contest, size + 1)
      if (
        ctx.state.entrantMembers.some((m) => m.contestId === contest.id && m.accountId === targetId)
      ) {
        throw conflict('account already belongs to an entrant in this contest')
      }
      ctx.state.entrantMembers.push({
        contestId: contest.id,
        entrantId: entrant.id,
        accountId: targetId,
        role: 'member',
        confirmedAt: null,
      })
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/entrants/:id/confirm',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const entrant = requireEntrant(ctx.state, ctx.params.id!)
      const membership = ctx.state.entrantMembers.find(
        (m) => m.entrantId === entrant.id && m.accountId === account.id,
      )
      if (!membership) throw notFound('invitation not found')
      if (membership.confirmedAt) return json({ ok: true })
      if (!account.emailVerified) throw unverified()
      const contest = requireContest(ctx.state, entrant.contestId)
      const orgMember = ctx.state.orgMembers.some(
        (m) => m.orgId === contest.orgId && m.accountId === account.id,
      )
      if (!orgMember && !contest.allowExternal) {
        throw forbidden('this contest does not allow external contestants')
      }
      membership.confirmedAt = ctx.now
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/entrants/:id/submit-registration',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const entrant = requireEntrant(ctx.state, ctx.params.id!)
      requireCaptain(ctx.state, entrant.id, account.id)
      const contest = requireContest(ctx.state, entrant.contestId)
      const members = ctx.state.entrantMembers.filter((m) => m.entrantId === entrant.id)
      if (members.some((m) => m.confirmedAt === null)) {
        throw conflict('all members must confirm before registration')
      }
      teamSizeOK(contest, members.length)
      const accepted = !contest.registrationRequiresReview
      entrant.status = accepted ? 'accepted' : 'pending'
      entrant.acceptedAt = accepted ? ctx.now : null
      if (accepted) entrant.rosterFrozenAt = ctx.now
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/entrants/:id/decision',
    handler: (ctx) => {
      const entrant = requireEntrant(ctx.state, ctx.params.id!)
      const contest = requireContest(ctx.state, entrant.contestId)
      if (!canManageOrg(ctx.state, contest.orgId, requireAccount(ctx))) {
        throw forbidden('organization manager role required')
      }
      const approve = (ctx.body as { approve?: boolean } | undefined)?.approve === true
      if (!approve) {
        entrant.status = 'rejected'
        return json({ ok: true })
      }
      entrant.status = 'accepted'
      entrant.acceptedAt = ctx.now
      entrant.rosterFrozenAt = ctx.now
      return json({ ok: true })
    },
  },
  {
    method: 'POST',
    pattern: '/api/entrants/:id/withdraw',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const entrant = requireEntrant(ctx.state, ctx.params.id!)
      const contest = requireContest(ctx.state, entrant.contestId)
      if (entrant.acceptedAt && ctx.now.getTime() >= contest.startsAt.getTime()) {
        throw conflict('cannot withdraw after start')
      }
      requireCaptain(ctx.state, entrant.id, account.id)
      ctx.state.entrantMembers = ctx.state.entrantMembers.filter((m) => m.entrantId !== entrant.id)
      entrant.status = 'withdrawn'
      entrant.acceptedAt = null
      return json({ ok: true })
    },
  },
]
