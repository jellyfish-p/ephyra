/**
 * Wire types for the Ephyra Go HTTP API.
 *
 * These mirror the DTOs in `server/internal/**` exactly, including field
 * naming. Most DTOs carry explicit `json` tags and are snake_case. Two
 * exceptions are called out below; do not "fix" them here without changing the
 * Go structs first.
 */

/* ------------------------------------------------------------------ errors */

/** Body of every non-2xx response: `{"error":{"code":"...","message":"..."}}`. */
export interface ApiErrorBody {
  error: { code: ApiErrorCode; message: string }
}

/** Mirrors `apperr.Code` in `server/internal/apperr`. */
export type ApiErrorCode =
  | 'unauthorized'
  | 'forbidden'
  | 'unverified'
  | 'not_found'
  | 'conflict'
  | 'invalid'
  | 'too_many'
  | 'gone'
  | 'not_acceptable'
  | 'unavailable'
  | 'internal'

/* -------------------------------------------------------------------- auth */

/** `POST /api/auth/login`, `GET /api/me`. */
export interface Account {
  account_id: string
  email: string
  display_name: string
  email_verified: boolean
  site_admin: boolean
}

/* ----------------------------------------------------------- organizations */

/** `GET /api/organizations`, `POST /api/organizations`, `GET /api/organizations/:id`. */
export interface Organization {
  id: string
  slug: string
  name: string
}

/** `GET /api/me/organizations` — an organization the account can see. */
export interface MyOrganization extends Organization {
  /** `owner` | `admin` | `member`; `site_admin` for a site administrator. */
  role: string
  member: boolean
}

/** `GET /api/organizations/:id/members`. */
export interface OrganizationMember {
  account_id: string
  display_name: string
  role: OrgRole
}

export type OrgRole = 'owner' | 'admin' | 'member'

/** `GET /api/organizations/:id/join-requests`. */
export interface JoinRequest {
  id: string
  account_id: string
  display_name: string
  status: string
  created_at: string
}

/** `GET /api/organizations/:id/membership`. */
export interface Membership {
  organization_id: string
  member: boolean
  role?: OrgRole
}

/* ---------------------------------------------------------------- contests */

export type ContestRule = 'acm' | 'oi' | 'ioi'
export type EntryMode = 'individual' | 'team'
export type FeedbackGranularity = 'case' | 'subtask'
export type ContestPhase = 'before' | 'running' | 'ended_unpublished' | 'published'

/** `GET /api/contests/:id`, `GET /api/organizations/:id/contests`. */
export interface Contest {
  id: string
  organization_id: string
  slug: string
  title: string
  rule: ContestRule
  entry_mode: EntryMode
  starts_at: string
  ends_at: string
  register_starts_at: string
  allow_external: boolean
  source_public_after: boolean
  source_visibility_locked: boolean
  published_at: string | null
}

/** `GET /api/contests/:id/problems`. */
export interface ContestProblem {
  /** Contest-problem row id; used to target rejudges. */
  id: string
  letter: string
  title?: string
  problem_id?: string
  scoring_revision_id?: string
  target_revision_id?: string
}

/** `GET /api/contests/:id/problems/:letter`. */
export interface ContestProblemDetail {
  letter: string
  title: string
  problem_id: string
  scoring_revision_id: string
  target_revision_id: string
  kind: ProblemKind
  checker_kind: CheckerKind
  time_limit_ms: number
  memory_limit_bytes: number
  subtasks: { id: string; score: string }[]
  statement_markdown: string | null
}

export type ProblemKind = 'batch' | 'interactive'
export type CheckerKind = 'exact' | 'token' | 'testlib'

/** `GET /api/organizations/:id/problems`. */
export interface BankProblem {
  id: string
  slug: string
  title: string
}

/** `GET /api/problems/:id/revisions`. */
export interface ProblemRevision {
  revision_id: string
  revision_n: number
  created_at: string
}

/* --------------------------------------------------------------- entrants */

export type EntrantStatus = 'pending' | 'accepted' | 'withdrawn' | 'rejected'

/** `GET /api/contests/:id/my-entrant`. */
export interface Entrant {
  id: string
  contest_id: string
  kind: EntryMode
  display_name: string
  status: EntrantStatus
  registered_at: string
  accepted_at: string | null
  roster_frozen_at: string | null
  members: EntrantMember[]
}

export interface EntrantMember {
  account_id: string
  display_name: string
  role: 'captain' | 'member'
  confirmed: boolean
}

/* ------------------------------------------------------------ submissions */

export type LanguageID = 'cpp23' | 'c17' | 'c23'

export type Verdict =
  | 'accepted'
  | 'wrong_answer'
  | 'time_limit_exceeded'
  | 'memory_limit_exceeded'
  | 'runtime_error'
  | 'compilation_error'
  | 'system_error'

/**
 * How much of a result the viewer is allowed to receive.
 * `none` = the submission exists but nothing about it may be shown.
 */
export type FeedbackLevel = 'none' | 'compile' | 'score' | 'full'

/** `GET /api/submissions/:id`, SSE `submission` event, `GET /api/contests/:id/submissions`. */
export interface Submission {
  id: string
  contest_id: string
  entrant_id: string
  submitter_account_id: string
  language_id: string
  received_at: string
  receive_seq: number
  revision_id: string
  feedback: FeedbackLevel
  /**
   * Set by the contest submission list. Absent from the single-submission and
   * SSE payloads, which do not resolve entrant names.
   */
  entrant_display_name?: string
  /** Present when this result is not the one the contest scores. */
  not_effective?: boolean
  compile_failed?: boolean
  verdict?: Verdict
  /** `exact.Score` marshals as a decimal string, e.g. `"50.00"`. */
  score?: string
  subtasks?: SubmissionSubtask[]
  cases?: SubmissionCase[]
}

export interface SubmissionSubtask {
  id: string
  score: string
  verdict: string
}

export interface SubmissionCase {
  case_id: string
  verdict: string
}

/** `POST /api/contests/:id/problems/:letter/submissions`. */
export interface SubmitAccepted {
  submission_id: string
  receive_seq: number
  received_at: string
  task_id: string
}

/* -------------------------------------------------------------- standings */

/**
 * `GET /api/contests/:id/standings`.
 *
 * Target contract: the backend emits snake_case keys plus `display_name` and
 * per-problem `letter`, so the table can label rows and columns without a
 * second lookup. `server/internal/scoring` is being updated to match.
 */
export interface StandingRow {
  entrant_id: string
  display_name: string
  rank: number
  solved: number
  /** `exact.Score` as a decimal string. */
  total_score: string
  penalty_minutes: number
  problems: StandingProblem[]
  unsettled: boolean
}

export interface StandingProblem {
  problem_id: string
  /** Contest problem letter, e.g. `A`. */
  letter: string
  score: string
  solved: boolean
  penalty_minutes: number
  failed_attempts_before_ac: number
  effective_submission_id: string
  pending: boolean
}

/* ------------------------------------------------------------ judge admin */

export type PoolKind = 'system' | 'organization'

/** `GET /api/organizations/:id/pools`. */
export interface Pool {
  id: string
  kind: PoolKind
  organization_id: string | null
  name: string
  enabled: boolean
}

/** `GET /api/pools/:id/nodes`. */
export interface JudgeNode {
  id: string
  pool_id: string
  name: string
  enabled: boolean
  connected: boolean
  revoked: boolean
  max_slots: number
  last_heartbeat_at: string | null
  advertised_profiles: string[]
}

/** `GET /api/organizations/:id/routing`, `POST /api/organizations/:id/routing`. */
export interface RoutingEntry {
  pool_id: string
  priority: number
}

/** `POST /api/pools/:id/nodes` and `POST /api/nodes/:id/rotate` — shown once. */
export interface NodeCredential {
  id?: string
  credential: string
}

/** `POST /api/contest-problems/:id/rejudge`. */
export interface RejudgeBatch {
  batch_id: string
  state: string
}

/**
 * `GET /api/contests/:id/tasks`.
 *
 * A task whose node was lost is resolved by an administrator; redispatch is
 * never automatic.
 */
export interface JudgeTask {
  id: string
  submission_id: string
  state: TaskState
  purpose: string
  revision_id: string
  created_at: string
  attempt: JudgingAttempt | null
}

export type TaskState =
  'pending_schedule' | 'dispatched' | 'completed' | 'awaiting_admin' | 'scheduling_error'

export type AttemptState = 'dispatched' | 'succeeded' | 'failed' | 'invalidated'

/** Dispatch bookkeeping only; never carries the raw judge result. */
export interface JudgingAttempt {
  id: string
  generation: number
  node_id: string
  pool_id: string
  state: AttemptState
  dispatched_at: string
  lease_until: string
  finished_at: string | null
  system_fault: boolean
}

/** `{"ok":true}` acknowledgement. */
export interface Ok {
  ok: boolean
}
