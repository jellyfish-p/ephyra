/**
 * The names the app uses for the wire types of the Ephyra Go HTTP API.
 *
 * The shapes are not declared here. They are generated from the backend's
 * OpenAPI document into `./generated.ts`, so a handler change that is not
 * reflected in the document fails CI (`npm run api:check`), and a change that
 * is reflected there flows into this app automatically.
 *
 * This file therefore does two things only: it gives each schema the name the
 * app already uses, and it records which route the schema belongs to. Deriving
 * an enum through `NonNullable<...>` is deliberate — the union comes from the
 * spec, so adding a verdict on the server surfaces here as a type error at
 * every `switch` that has not handled it.
 *
 * Regenerate with `npm run api:generate`; never edit `./generated.ts`.
 */
import type { components } from './generated'

type S = components['schemas']

/* ------------------------------------------------------------------ errors */

/** Body of every non-2xx response: `{"error":{"code":"...","message":"..."}}`. */
export type ApiErrorBody = S['ErrorResponse']

/** Mirrors `apperr.Code` in `server/internal/apperr`. */
export type ApiErrorCode = S['ErrorBody']['code']

/* -------------------------------------------------------------------- auth */

/** `POST /api/auth/register`, `POST /api/auth/login`, `GET /api/me`. */
export type Account = S['accountResponse']

/* ----------------------------------------------------------- organizations */

/** `GET /api/organizations`, `POST /api/organizations`, `GET /api/organizations/:id`. */
export type Organization = S['OrganizationView']

/** `GET /api/me/organizations` — an organization the account can see. */
export type MyOrganization = S['MyOrganizationView']

/** `GET /api/organizations/:id/members`. */
export type OrganizationMember = S['Member']

/** `GET /api/organizations/:id/join-requests`. */
export type JoinRequest = S['JoinRequest']

/** `GET /api/organizations/:id/membership`. */
export type Membership = S['membershipResponse']

/** Organization roles. `site_admin` is not a role but a global flag on the account. */
export type OrgRole = NonNullable<S['membershipResponse']['role']>

/* ---------------------------------------------------------------- contests */

/** `GET /api/contests/:id`, `GET /api/organizations/:id/contests`. */
export type Contest = S['contestResponse']

export type ContestRule = NonNullable<S['contestResponse']['rule']>
export type EntryMode = NonNullable<S['contestResponse']['entry_mode']>

/**
 * Derived on the client from the contest window; the API does not send it, so
 * it has no schema to bind to.
 */
export type ContestPhase = 'before' | 'running' | 'ended_unpublished' | 'published'

/**
 * A contest setting chosen at creation time, not a response field, so there is
 * no schema for it either.
 */
export type FeedbackGranularity = 'case' | 'subtask'

/** `GET /api/contests/:id/problems`. */
export type ContestProblem = S['ProblemView']

/** `GET /api/contests/:id/problems/:letter`. */
export type ContestProblemDetail = S['ProblemDetail']

export type ProblemKind = NonNullable<S['ProblemDetail']['kind']>
export type CheckerKind = NonNullable<S['ProblemDetail']['checker_kind']>

/** `GET /api/organizations/:id/problems`. */
export type BankProblem = S['BankProblem']

/** `GET /api/problems/:id/revisions`. */
export type ProblemRevision = S['RevisionSummary']

/* --------------------------------------------------------------- entrants */

/** `GET /api/contests/:id/my-entrant`. */
export type Entrant = S['MyEntrantView']

export type EntrantStatus = NonNullable<S['MyEntrantView']['status']>

/** One row of `Entrant.members`. */
export type EntrantMember = S['EntrantMemberView']

/* ------------------------------------------------------------ submissions */

export type LanguageID = NonNullable<S['SubmissionPublic']['language_id']>

export type Verdict = NonNullable<S['SubmissionPublic']['verdict']>

/**
 * How much of a result the viewer is allowed to receive.
 * `none` = the submission exists but nothing about it may be shown.
 */
export type FeedbackLevel = NonNullable<S['SubmissionPublic']['feedback']>

/**
 * `GET /api/submissions/:id`, SSE `submission` event,
 * `GET /api/contests/:id/submissions`.
 *
 * Result fields appear only as far as the viewer's feedback level allows, and
 * `entrant_display_name` is set by the contest submission list alone, so a
 * client must branch on presence rather than assume a fixed shape.
 */
export type Submission = S['SubmissionPublic']

export type SubmissionSubtask = S['SubtaskPublic']
export type SubmissionCase = S['CasePublic']

/** `POST /api/contests/:id/problems/:letter/submissions`. */
export type SubmitAccepted = S['submitAcceptedResponse']

/* -------------------------------------------------------------- standings */

/** `GET /api/contests/:id/standings`. */
export type StandingRow = S['Standing']

export type StandingProblem = S['ProblemScore']

/* ------------------------------------------------------------ judge admin */

/** `GET /api/organizations/:id/pools`. */
export type Pool = S['PoolView']

export type PoolKind = NonNullable<S['PoolView']['kind']>

/** `GET /api/pools/:id/nodes`. */
export type JudgeNode = S['NodeView']

/** `GET /api/organizations/:id/routing`, `POST /api/organizations/:id/routing`. */
export type RoutingEntry = S['RoutingEntry']

/** `POST /api/contest-problems/:id/rejudge`. */
export type RejudgeBatch = S['RejudgeBatchView']

/**
 * `GET /api/contests/:id/tasks`.
 *
 * A task whose node was lost is resolved by an administrator; redispatch is
 * never automatic.
 */
export type JudgeTask = S['TaskView']

export type TaskState = NonNullable<S['TaskView']['state']>

/** Dispatch bookkeeping only; never carries the raw judge result. */
export type JudgingAttempt = S['AttemptView']

export type AttemptState = NonNullable<S['AttemptView']['state']>

/**
 * The plaintext credential for a judge node, returned only at registration and
 * rotation time.
 */
export type NodeCredential = S['CredentialView']

/** `{"ok":true}` acknowledgement. */
export type Ok = S['AckView']
