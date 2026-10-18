import type { ContestPhase, FeedbackGranularity, OrgRole, Submission } from '~/types/api'
import { forbidden, notFound, unauthorized, unverified } from './errors'
import type {
  MockAccount,
  MockContest,
  MockContext,
  MockContestProblem,
  MockEntrant,
  MockOrg,
  MockResult,
  MockState,
  MockSubmission,
} from './types'
import { type Audience, type Visibility, contestPhase, projectSubmission, visible } from './scoring'

/**
 * Authorization and visibility resolution.
 *
 * These are ports of `access.Audience`, `scoring.Module.audience`,
 * `audienceForSubmission` and `scoring.Visible`. They live in one place because
 * both the standings endpoint and the submission endpoints depend on getting
 * them *identical* — a mock that computed visibility twice would drift.
 */

export function requireAccount(ctx: MockContext): MockAccount {
  if (!ctx.account) throw unauthorized()
  return ctx.account
}

export function requireVerified(account: MockAccount): MockAccount {
  if (!account.emailVerified) throw unverified()
  return account
}

export function findOrg(state: MockState, id: string): MockOrg | null {
  return state.orgs.find((o) => o.id === id) ?? null
}

export function requireOrg(state: MockState, id: string): MockOrg {
  const org = findOrg(state, id)
  if (!org) throw notFound('organization not found')
  return org
}

export function findContest(state: MockState, id: string): MockContest | null {
  return state.contests.find((c) => c.id === id) ?? null
}

export function requireContest(state: MockState, id: string): MockContest {
  const contest = findContest(state, id)
  if (!contest) throw notFound('contest not found')
  return contest
}

export function findContestProblem(state: MockState, id: string): MockContestProblem | null {
  return state.contestProblems.find((cp) => cp.id === id) ?? null
}

/* --------------------------------------------------------------- roles */

export function orgRole(state: MockState, orgId: string, accountId: string): OrgRole | null {
  const row = state.orgMembers.find((m) => m.orgId === orgId && m.accountId === accountId)
  return row?.role ?? null
}

export function canManageOrg(
  state: MockState,
  orgId: string,
  account: MockAccount | null,
): boolean {
  if (!account) return false
  if (account.siteAdmin) return true
  const role = orgRole(state, orgId, account.id)
  return role === 'owner' || role === 'admin'
}

/** Mirrors `contest.Access`, derived server-side and never taken from the client. */
export interface ContestAccess {
  siteAdmin: boolean
  orgMember: boolean
  orgManage: boolean
  registered: boolean
  accepted: boolean
}

export function contestAccess(
  state: MockState,
  contest: MockContest,
  account: MockAccount | null,
): ContestAccess {
  const siteAdmin = account?.siteAdmin === true
  const membership = account
    ? state.orgMembers.find((m) => m.orgId === contest.orgId && m.accountId === account.id)
    : undefined
  const entrantMember = account
    ? state.entrantMembers.find((m) => m.contestId === contest.id && m.accountId === account.id)
    : undefined
  const entrant = entrantMember
    ? state.entrants.find((e) => e.id === entrantMember.entrantId)
    : undefined
  return {
    siteAdmin,
    orgMember: membership !== undefined,
    orgManage: canManageOrg(state, contest.orgId, account),
    registered: entrantMember !== undefined,
    accepted: entrant?.status === 'accepted',
  }
}

/** Port of `CanViewContest`. */
export function canViewContest(
  contest: MockContest,
  signedIn: boolean,
  access: ContestAccess,
): boolean {
  if (access.siteAdmin || access.orgMember || access.accepted || access.registered) return true
  if (contest.allowExternal && signedIn) return true
  return false
}

/** Port of `CanViewContestProblems`. */
export function canViewContestProblems(access: ContestAccess): boolean {
  return access.siteAdmin || access.orgMember || access.accepted
}

/** Port of `ContestantLettersOnly`: letters only, no titles or revisions. */
export function contestantLettersOnly(access: ContestAccess): boolean {
  return access.accepted && !access.orgMember && !access.siteAdmin
}

export function requireOrgManager(ctx: MockContext, orgId: string): MockAccount {
  const account = requireAccount(ctx)
  if (!canManageOrg(ctx.state, orgId, account)) {
    throw forbidden('organization manager role required')
  }
  return account
}

/* ------------------------------------------------------------ audience */

/** Port of `scoring.Module.audience` — the caller's relation to the contest. */
export function audienceForContest(
  state: MockState,
  contest: MockContest,
  account: MockAccount | null,
): Audience {
  if (!account) return 'anonymous'
  if (account.siteAdmin) return 'org_admin'
  const role = orgRole(state, contest.orgId, account.id)
  if (role === 'owner' || role === 'admin') return 'org_admin'
  const membership = state.entrantMembers.find(
    (m) => m.contestId === contest.id && m.accountId === account.id,
  )
  if (membership) return 'self'
  return 'logged_in'
}

/** Port of `scoring.Module.audienceForSubmission`. */
export function audienceForSubmission(
  state: MockState,
  contest: MockContest,
  submission: MockSubmission,
  account: MockAccount | null,
): Audience {
  const aud = audienceForContest(state, contest, account)
  if (aud === 'org_admin') return 'contest_admin'
  if (!account) return aud
  if (account.id === submission.submitterAccountId) return 'self'
  const teammate = state.entrantMembers.find(
    (m) =>
      m.contestId === contest.id &&
      m.accountId === account.id &&
      m.entrantId === submission.entrantId,
  )
  if (teammate) return 'teammate'
  if (aud === 'self') return 'other_contestant'
  return aud
}

/** The entrant the viewer belongs to in this contest, if any. */
export function viewerEntrantId(
  state: MockState,
  contestId: string,
  account: MockAccount | null,
): string | null {
  if (!account) return null
  const membership = state.entrantMembers.find(
    (m) => m.contestId === contestId && m.accountId === account.id,
  )
  return membership?.entrantId ?? null
}

export function entrantMembers(state: MockState, entrantId: string) {
  return state.entrantMembers
    .filter((m) => m.entrantId === entrantId)
    .sort((a, b) => (a.accountId < b.accountId ? -1 : 1))
}

export function findEntrant(state: MockState, id: string): MockEntrant | null {
  return state.entrants.find((e) => e.id === id) ?? null
}

export function requireEntrant(state: MockState, id: string): MockEntrant {
  const entrant = findEntrant(state, id)
  if (!entrant) throw notFound('entrant not found')
  return entrant
}

/* ---------------------------------------------------------- visibility */

export function scoringRevisionOf(state: MockState, submission: MockSubmission): string {
  const cp = findContestProblem(state, submission.contestProblemId)
  return cp?.scoringRevisionId ?? submission.revisionId
}

/** The result shown for a submission, and the revision that produced it. */
export interface DisplayedResult {
  result: MockResult
  revisionId: string
}

/**
 * Port of `scoring.Module.loadResult`: target revision, then the submission's
 * accepted revision, then the scoring revision.
 *
 * The order matters. While a rejudge is open, `target` holds the new results, so
 * a submission judged on the new data displays them and is flagged
 * `not_effective`; a submission that only has an old result still displays the
 * old one, which is what keeps the previous scoreboard readable.
 */
export function resolveDisplayedResult(
  state: MockState,
  submission: MockSubmission,
): DisplayedResult | null {
  const cp = findContestProblem(state, submission.contestProblemId)
  const order = [cp?.targetRevisionId, submission.revisionId, cp?.scoringRevisionId]
  const seen = new Set<string>()
  for (const revisionId of order) {
    if (!revisionId || seen.has(revisionId)) continue
    seen.add(revisionId)
    const result = submission.results[revisionId]
    if (result) return { result, revisionId }
  }
  return null
}

export function visibilityFor(
  state: MockState,
  contest: MockContest,
  submission: MockSubmission,
  account: MockAccount | null,
  now: Date,
): Visibility {
  const displayed = resolveDisplayedResult(state, submission)
  return visible({
    rule: contest.rule,
    phase: contestPhase(contest, now),
    now,
    endsAt: contest.endsAt,
    freezeFor: contest.freezeMinutesBeforeEnd ?? 0,
    granularity: contest.caseFeedback as FeedbackGranularity,
    ioiPublicBoard: contest.ioiPublicScoreboard,
    sourcePublic: contest.sourcePublicAfter,
    published: contest.publishedAt !== null,
    audience: audienceForSubmission(state, contest, submission, account),
    submissionRevision: displayed?.revisionId ?? submission.revisionId,
    scoringRevision: scoringRevisionOf(state, submission),
  })
}

/** Projects one submission for one viewer, exactly as the backend would. */
export function projectFor(
  state: MockState,
  contest: MockContest,
  submission: MockSubmission,
  account: MockAccount | null,
  now: Date,
): { submission: Submission; view: Visibility } {
  const view = visibilityFor(state, contest, submission, account, now)
  const displayed = resolveDisplayedResult(state, submission)
  const projected = projectSubmission(
    {
      id: submission.id,
      contestId: submission.contestId,
      entrantId: submission.entrantId,
      submitterAccountId: submission.submitterAccountId,
      languageId: submission.languageId,
      receivedAt: submission.receivedAt,
      receiveSeq: submission.receiveSeq,
      revisionId: displayed?.revisionId ?? submission.revisionId,
    },
    view,
    displayed?.result ?? null,
  )
  return { submission: projected, view }
}

/** Contest phase plus the flags the contest view needs. */
export function contestPhaseOf(contest: MockContest, now: Date): ContestPhase {
  return contestPhase(contest, now)
}

/**
 * `source_visibility_locked` follows `SourceVisibilityShouldLock`: the setting
 * freezes as soon as registration opens.
 */
export function sourceVisibilityLocked(contest: MockContest, now: Date): boolean {
  return now.getTime() >= contest.registerStartsAt.getTime()
}
