import type {
  AttemptState,
  CheckerKind,
  ContestRule,
  EntryMode,
  FeedbackGranularity,
  LanguageID,
  OrgRole,
  PoolKind,
  ProblemKind,
  Submission,
  TaskState,
  Verdict,
} from '~/types/api'
import type { HttpMethod } from '~/composables/useApi'

/* ------------------------------------------------------------------- rows */

/** Internal mock tables. These are the analogue of `persist` models. */

export interface MockAccount {
  id: string
  email: string
  password: string
  displayName: string
  emailVerified: boolean
  siteAdmin: boolean
}

export interface MockOrg {
  id: string
  slug: string
  name: string
}

export interface MockOrgMember {
  orgId: string
  accountId: string
  role: OrgRole
}

export interface MockJoinRequest {
  id: string
  orgId: string
  accountId: string
  status: 'pending' | 'approved' | 'rejected'
  createdAt: Date
}

export interface MockContest {
  id: string
  orgId: string
  slug: string
  title: string
  rule: ContestRule
  entryMode: EntryMode
  startsAt: Date
  endsAt: Date
  registerStartsAt: Date
  allowExternal: boolean
  registrationRequiresReview: boolean
  maxTeamSize: number
  acmPenaltyMinutes: number
  freezeMinutesBeforeEnd: number | null
  ioiPublicScoreboard: boolean
  caseFeedback: FeedbackGranularity
  sourcePublicAfter: boolean
  publishedAt: Date | null
  sourceVisibilityLocked: boolean
}

export interface MockProblem {
  id: string
  orgId: string
  slug: string
  title: string
}

export interface MockRevision {
  id: string
  problemId: string
  n: number
  createdAt: Date
  statement: string | null
  kind: ProblemKind
  checkerKind: CheckerKind
  timeLimitMs: number
  memoryLimitBytes: number
  subtasks: { id: string; score: string }[]
}

export interface MockContestProblem {
  id: string
  contestId: string
  problemId: string
  scoringRevisionId: string
  targetRevisionId: string
  letter: string
}

export interface MockEntrant {
  id: string
  contestId: string
  kind: EntryMode
  displayName: string
  status: 'pending' | 'accepted' | 'withdrawn' | 'rejected'
  registeredAt: Date
  acceptedAt: Date | null
  rosterFrozenAt: Date | null
}

export interface MockEntrantMember {
  contestId: string
  entrantId: string
  accountId: string
  role: 'captain' | 'member'
  confirmedAt: Date | null
}

export interface MockResult {
  verdict: Verdict
  /** `exact.Score` — a decimal string, never a float. */
  score: string
  compileFailed: boolean
  /** A system fault hides the result from non-admins. */
  systemFault: boolean
  subtasks: { id: string; score: string; verdict: Verdict }[]
  cases: { case_id: string; verdict: Verdict }[]
}

export interface MockSubmission {
  id: string
  contestId: string
  contestProblemId: string
  entrantId: string
  submitterAccountId: string
  languageId: LanguageID
  receivedAt: Date
  receiveSeq: number
  /** Judge data revision this submission was accepted with; never rewritten. */
  revisionId: string
  source: string
  /**
   * Results keyed by the revision that produced them. A rejudge adds an entry
   * for the new revision and leaves the old one in place, which is what keeps
   * the old scoreboard standing until the batch is switched.
   */
  results: Record<string, MockResult>
}

export interface MockPool {
  id: string
  kind: PoolKind
  orgId: string | null
  name: string
  enabled: boolean
}

export interface MockNode {
  id: string
  poolId: string
  name: string
  enabled: boolean
  connected: boolean
  revoked: boolean
  maxSlots: number
  lastHeartbeatAt: Date | null
  advertisedProfiles: string[]
  /** Plaintext, but only ever returned at register/rotate time. */
  credential: string
}

export interface MockRouting {
  orgId: string
  poolId: string
  priority: number
}

export interface MockTask {
  id: string
  submissionId: string
  contestId: string
  revisionId: string
  profile: string
  purpose: string
  state: TaskState
  createdAt: Date
  /** Set for `purpose: 'rejudge'` tasks; the batch they belong to. */
  batchId: string | null
}

export interface MockAttempt {
  id: string
  taskId: string
  generation: number
  nodeId: string
  poolId: string
  state: AttemptState
  dispatchedAt: Date
  leaseUntil: Date
  finishedAt: Date | null
  systemFault: boolean
}

export interface MockRejudgeBatch {
  id: string
  contestProblemId: string
  fromRevisionId: string
  toRevisionId: string
  state: 'running' | 'switched'
  createdAt: Date
}

/** A pending email token; the backend mails a bare token, never a link. */
export interface MockToken {
  token: string
  accountId: string
  kind: 'verify' | 'reset'
  expiresAt: Date
}

/* --------------------------------------------------------------- requests */

export interface MockRequest {
  method: HttpMethod
  path: string
  query: Record<string, unknown>
  body: unknown
  headers: Record<string, string>
}

export interface MockResponse {
  status: number
  body: unknown
  /** When set, the transport returns `body` verbatim instead of JSON. */
  contentType?: string
}

/** Every handler returns one of these, so the status is never implicit. */
export function json(body: unknown, status = 200): MockResponse {
  return { status, body }
}

/** Raw body, for the one endpoint that returns source text. */
export function text(body: string, status = 200): MockResponse {
  return { status, body, contentType: 'text/plain; charset=utf-8' }
}

export interface MockContext {
  runtime: MockRuntime
  req: MockRequest
  params: Record<string, string>
  /** Resolved from the session cookie; `null` when signed out. */
  account: MockAccount | null
  /** `runtime.state`, hoisted for handler ergonomics. */
  state: MockState
  /** `runtime.now()`, captured once per request. */
  now: Date
  /** `req.body`, hoisted for handler ergonomics. */
  body: unknown
}

/** Everything a handler needs that is not request data. */
export interface MockRuntime {
  state: MockState
  /** Respects the frozen clock used by tests. */
  now: () => Date
  /** Deferred judge completion, so pending states are observable. */
  schedule: (delayMs: number, run: () => void) => void
  /**
   * Publishes one SSE frame. The projector runs per subscriber, because
   * visibility is per viewer — the same submission projects differently for an
   * admin, the owner, and an unrelated contestant.
   */
  publish: (contestId: string, project: (accountId: string | null) => Submission | null) => void
  /** Drops in-flight judge timers, so a reset does not leak them. */
  cancelScheduled: () => void
  /** Rebuilds the seeded world and clears cookies. */
  reset: () => void
}

export type MockHandler = (ctx: MockContext) => MockResponse | Promise<MockResponse>

export interface MockRoute {
  method: HttpMethod
  /** `/api/contests/:id/problems/:letter` — `:name` captures one segment. */
  pattern: string
  handler: MockHandler
}

export interface MockState {
  /** Frozen clock for tests; `null` means "use the wall clock". */
  fixedNow: Date | null
  accounts: MockAccount[]
  orgs: MockOrg[]
  orgMembers: MockOrgMember[]
  joinRequests: MockJoinRequest[]
  contests: MockContest[]
  problems: MockProblem[]
  revisions: MockRevision[]
  contestProblems: MockContestProblem[]
  entrants: MockEntrant[]
  entrantMembers: MockEntrantMember[]
  submissions: MockSubmission[]
  pools: MockPool[]
  nodes: MockNode[]
  routing: MockRouting[]
  tasks: MockTask[]
  attempts: MockAttempt[]
  rejudgeBatches: MockRejudgeBatch[]
  tokens: MockToken[]
  /** Monotonic per contest; the analogue of `receive_seq`. */
  receiveSeq: number
  /** Per-kind id counters, so ids stay unique across mutations. */
  idCounters: Record<string, number>
}
