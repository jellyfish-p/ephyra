import type {
  ContestPhase,
  ContestRule,
  FeedbackGranularity,
  FeedbackLevel,
  LanguageID,
  StandingRow,
  StandingProblem,
  Submission,
  Verdict,
} from '~/types/api'
import type { MockContest, MockContestProblem, MockResult, MockSubmission } from './types'

/**
 * A faithful port of `server/internal/scoring`.
 *
 * The mock reimplements the rules rather than hard-coding a plausible-looking
 * board, because the UI branches on them: the standings table renders a
 * different cell per rule, and the submission view renders a different amount of
 * detail per feedback level. A mock that guessed would let those branches rot.
 */

/* ------------------------------------------------------------------ score */

/** `exact.Score` is hundredths in a decimal string, e.g. `"50.00"`. */
export function scoreHundredths(score: string): number {
  const value = Number.parseFloat(score)
  return Number.isFinite(value) ? Math.round(value * 100) : 0
}

export function formatScore(hundredths: number): string {
  const sign = hundredths < 0 ? '-' : ''
  const h = Math.abs(hundredths)
  return `${sign}${Math.floor(h / 100)}.${String(h % 100).padStart(2, '0')}`
}

/* --------------------------------------------------------------- verdicts */

export function isPlayerError(v: Verdict): boolean {
  return (
    v === 'wrong_answer' ||
    v === 'time_limit_exceeded' ||
    v === 'memory_limit_exceeded' ||
    v === 'runtime_error'
  )
}

export function isNormalTerminal(v: Verdict): boolean {
  return isPlayerError(v) || v === 'accepted' || v === 'compilation_error'
}

/* ------------------------------------------------------------------ phase */

export function contestPhase(contest: MockContest, now: Date): ContestPhase {
  if (contest.publishedAt) return 'published'
  if (now.getTime() < contest.startsAt.getTime()) return 'before'
  if (now.getTime() < contest.endsAt.getTime()) return 'running'
  return 'ended_unpublished'
}

/* --------------------------------------------------------------- audience */

/** Mirrors `access.Audience`. */
export type Audience =
  | 'self'
  | 'teammate'
  | 'contest_admin'
  | 'org_admin'
  | 'other_contestant'
  | 'logged_in'
  | 'anonymous'

export function isAdminAudience(a: Audience): boolean {
  return a === 'contest_admin' || a === 'org_admin'
}

export function isSelfAudience(a: Audience): boolean {
  return a === 'self' || a === 'teammate'
}

/* ------------------------------------------------------------- visibility */

export interface VisibilityInput {
  rule: ContestRule
  phase: ContestPhase
  now: Date
  endsAt: Date
  /** ACM freeze window in minutes before the end; `0` disables. */
  freezeFor: number
  granularity: FeedbackGranularity
  ioiPublicBoard: boolean
  sourcePublic: boolean
  published: boolean
  audience: Audience
  submissionRevision: string
  scoringRevision: string
}

export interface Visibility {
  feedback: FeedbackLevel
  granularity: FeedbackGranularity
  showOnBoard: boolean
  showSource: boolean
  notEffective: boolean
}

export function acmFrozen(inp: VisibilityInput): boolean {
  if (inp.rule !== 'acm' || inp.freezeFor <= 0 || inp.published) return false
  const freezeAt = inp.endsAt.getTime() - inp.freezeFor * 60_000
  return inp.now.getTime() >= freezeAt && inp.phase !== 'published'
}

/** Port of `scoring.Visible`. */
export function visible(inp: VisibilityInput): Visibility {
  const admin = isAdminAudience(inp.audience)
  const self = isSelfAudience(inp.audience)
  const out: Visibility = {
    feedback: 'none',
    granularity: inp.granularity,
    showOnBoard: false,
    showSource: sourceVisible(inp, admin, self),
    notEffective: revisionNotEffective(inp),
  }

  if (admin) {
    out.feedback = 'full'
    out.showOnBoard = true
    return out
  }

  const frozen = acmFrozen(inp)
  switch (inp.rule) {
    case 'oi':
      switch (inp.phase) {
        case 'running':
        case 'before':
          if (self) out.feedback = 'compile'
          out.showOnBoard = false
          break
        case 'ended_unpublished':
          if (self) out.feedback = 'compile'
          out.showOnBoard = false
          break
        case 'published':
          out.feedback = self ? 'full' : 'score'
          out.showOnBoard = true
          break
      }
      break
    case 'ioi':
      if (self) out.feedback = 'full'
      out.showOnBoard = inp.ioiPublicBoard || inp.phase === 'published'
      if (inp.phase !== 'published' && !self && !inp.ioiPublicBoard) {
        out.showOnBoard = false
        out.feedback = 'none'
      }
      break
    case 'acm':
      if (self) out.feedback = 'full'
      out.showOnBoard = true
      if (frozen && !self) {
        out.showOnBoard = true
        out.feedback = 'none'
      }
      if (inp.phase === 'before') out.showOnBoard = false
      break
  }

  if (inp.phase !== 'published' && inp.granularity === 'case' && inp.rule === 'oi') {
    out.granularity = 'subtask'
  }
  if (inp.rule === 'oi' && (inp.phase === 'running' || inp.phase === 'ended_unpublished')) {
    out.granularity = 'subtask'
    if (self) out.feedback = 'compile'
  }
  return out
}

function sourceVisible(inp: VisibilityInput, admin: boolean, self: boolean): boolean {
  if (admin || self) return true
  if (inp.audience === 'anonymous') return false
  if (inp.sourcePublic && inp.published) {
    return inp.audience === 'logged_in' || inp.audience === 'other_contestant'
  }
  return false
}

function revisionNotEffective(inp: VisibilityInput): boolean {
  if (!inp.scoringRevision || !inp.submissionRevision) return false
  return inp.submissionRevision !== inp.scoringRevision
}

export function shouldEmit(view: Visibility): boolean {
  return view.feedback !== 'none'
}

/* ------------------------------------------------------------ projection */

export interface EmitMask {
  compileFailed: boolean
  verdict: boolean
  score: boolean
  subtasks: boolean
  cases: boolean
}

export function emitFields(view: Visibility): EmitMask {
  const m: EmitMask = {
    compileFailed: false,
    verdict: false,
    score: false,
    subtasks: false,
    cases: false,
  }
  switch (view.feedback) {
    case 'compile':
      m.compileFailed = true
      break
    case 'score':
      m.compileFailed = true
      m.verdict = true
      m.score = true
      break
    case 'full':
      m.compileFailed = true
      m.verdict = true
      m.score = true
      m.subtasks = true
      if (view.granularity === 'case') m.cases = true
      break
    case 'none':
      break
  }
  return m
}

export interface SubmissionMeta {
  id: string
  contestId: string
  entrantId: string
  submitterAccountId: string
  languageId: LanguageID
  receivedAt: Date
  receiveSeq: number
  revisionId: string
}

/** Port of `scoring.ProjectSubmission`: absent means "not visible to you". */
export function projectSubmission(
  meta: SubmissionMeta,
  view: Visibility,
  result: MockResult | null,
): Submission {
  const out: Submission = {
    id: meta.id,
    contest_id: meta.contestId,
    entrant_id: meta.entrantId,
    submitter_account_id: meta.submitterAccountId,
    language_id: meta.languageId,
    received_at: meta.receivedAt.toISOString(),
    receive_seq: meta.receiveSeq,
    revision_id: meta.revisionId,
    feedback: view.feedback,
  }
  if (view.notEffective) out.not_effective = true
  if (!result) return out
  if (result.systemFault && view.feedback !== 'full') return out

  const mask = emitFields(view)
  if (mask.compileFailed && !result.systemFault) {
    out.compile_failed = result.compileFailed || result.verdict === 'compilation_error'
  }
  if (mask.verdict) out.verdict = result.verdict
  if (mask.score && !result.systemFault) out.score = result.score
  if (mask.subtasks && !result.systemFault) {
    if (result.subtasks.length) out.subtasks = result.subtasks
  }
  if (mask.cases && !result.systemFault) {
    if (result.cases.length) out.cases = result.cases
  }
  return out
}

/* -------------------------------------------------------------- standings */

export interface ScoringProblem {
  id: string
  letter: string
  revisionId: string
}

export interface ScoringEntrant {
  id: string
  displayName: string
}

export interface ScoringSubmission {
  id: string
  entrantId: string
  problemId: string
  revisionId: string
  receiveSeq: number
  receivedAt: Date
  result: MockResult | null
}

export interface ScoringInput {
  rule: ContestRule
  start: Date
  penaltyMinutes: number
  problems: ScoringProblem[]
  entrants: ScoringEntrant[]
  submissions: ScoringSubmission[]
}

/** Port of `scoring.ScoreContest` — a pure function of receive order. */
export function scoreContest(inp: ScoringInput): StandingRow[] {
  const byEntrant = new Map<string, ScoringSubmission[]>()
  for (const s of inp.submissions) {
    const list = byEntrant.get(s.entrantId)
    if (list) list.push(s)
    else byEntrant.set(s.entrantId, [s])
  }
  for (const list of byEntrant.values()) {
    list.sort((a, b) =>
      a.receiveSeq !== b.receiveSeq
        ? a.receiveSeq - b.receiveSeq
        : a.id < b.id
          ? -1
          : a.id > b.id
            ? 1
            : 0,
    )
  }

  const rows: StandingRow[] = inp.entrants.map((e) => {
    const row: StandingRow = {
      entrant_id: e.id,
      display_name: e.displayName,
      rank: 0,
      solved: 0,
      total_score: '0.00',
      penalty_minutes: 0,
      problems: [],
      unsettled: false,
    }
    for (const p of inp.problems) {
      const subs = (byEntrant.get(e.id) ?? []).filter((s) => s.problemId === p.id)
      const ps = scoreProblem(inp, p, subs)
      row.problems.push(ps)
      if (ps.pending) row.unsettled = true
      if (ps.pending && inp.rule === 'oi') continue
      if (inp.rule === 'acm') {
        if (ps.solved) {
          row.solved += 1
          row.penalty_minutes += ps.penalty_minutes
        }
      } else {
        row.total_score = formatScore(scoreHundredths(row.total_score) + scoreHundredths(ps.score))
      }
    }
    return row
  })
  rank(inp.rule, rows)
  return rows
}

function emptyProblemScore(p: ScoringProblem): StandingProblem {
  return {
    problem_id: p.id,
    letter: p.letter,
    score: '0.00',
    solved: false,
    penalty_minutes: 0,
    failed_attempts_before_ac: 0,
    effective_submission_id: '',
    pending: false,
  }
}

function normalOn(s: ScoringSubmission, revisionId: string): boolean {
  if (s.revisionId !== revisionId || !s.result) return false
  if (s.result.systemFault) return false
  return isNormalTerminal(s.result.verdict)
}

function scoreProblem(
  inp: ScoringInput,
  p: ScoringProblem,
  subs: ScoringSubmission[],
): StandingProblem {
  switch (inp.rule) {
    case 'acm':
      return scoreACM(inp, p, subs)
    case 'oi':
      return scoreOI(p, subs)
    case 'ioi':
      return scoreIOI(p, subs)
    default:
      return emptyProblemScore(p)
  }
}

function scoreACM(
  inp: ScoringInput,
  p: ScoringProblem,
  subs: ScoringSubmission[],
): StandingProblem {
  const ps = emptyProblemScore(p)
  let fails = 0
  for (const s of subs) {
    if (!normalOn(s, p.revisionId)) {
      const result = s.result
      if (
        (result && result.systemFault && s.revisionId === p.revisionId) ||
        (!result && s.revisionId === p.revisionId)
      ) {
        ps.pending = true
      }
      continue
    }
    const result = s.result!
    if (result.verdict === 'accepted') {
      ps.solved = true
      ps.score = '1.00'
      ps.effective_submission_id = s.id
      ps.failed_attempts_before_ac = fails
      const elapsed = Math.max(0, s.receivedAt.getTime() - inp.start.getTime())
      ps.penalty_minutes = roundHalfUpMinutes(elapsed) + fails * inp.penaltyMinutes
      ps.pending = false
      return ps
    }
    if (isPlayerError(result.verdict)) fails += 1
  }
  ps.failed_attempts_before_ac = fails
  return ps
}

function scoreOI(p: ScoringProblem, subs: ScoringSubmission[]): StandingProblem {
  const ps = emptyProblemScore(p)
  if (subs.length === 0) return ps
  const last = subs[subs.length - 1]!
  if (!normalOn(last, p.revisionId)) {
    ps.pending = true
    ps.effective_submission_id = last.id
    return ps
  }
  ps.score = last.result!.score
  ps.effective_submission_id = last.id
  ps.solved = last.result!.verdict === 'accepted'
  return ps
}

function scoreIOI(p: ScoringProblem, subs: ScoringSubmission[]): StandingProblem {
  const ps = emptyProblemScore(p)
  let best: ScoringSubmission | null = null
  let pending = false
  for (const s of subs) {
    if (s.revisionId === p.revisionId && (!s.result || s.result.systemFault)) {
      pending = true
      continue
    }
    if (!normalOn(s, p.revisionId)) continue
    if (best === null || scoreHundredths(s.result!.score) > scoreHundredths(best.result!.score)) {
      best = s
    }
  }
  if (best === null) {
    ps.pending = pending
    return ps
  }
  ps.score = best.result!.score
  ps.effective_submission_id = best.id
  ps.solved = best.result!.verdict === 'accepted'
  ps.pending = pending
  return ps
}

/** `1:29` → 1, `1:30` → 2. */
export function roundHalfUpMinutes(elapsedMs: number): number {
  if (elapsedMs < 0) return 0
  return Math.floor((elapsedMs + 30_000) / 60_000)
}

function rank(rule: ContestRule, rows: StandingRow[]): void {
  rows.sort((a, b) => {
    if (rule === 'acm') {
      if (a.solved !== b.solved) return b.solved - a.solved
      if (a.penalty_minutes !== b.penalty_minutes) return a.penalty_minutes - b.penalty_minutes
    } else {
      const diff = scoreHundredths(b.total_score) - scoreHundredths(a.total_score)
      if (diff !== 0) return diff
    }
    return a.entrant_id < b.entrant_id ? -1 : a.entrant_id > b.entrant_id ? 1 : 0
  })
  for (let i = 0; i < rows.length; i++) {
    if (i === 0) {
      rows[i]!.rank = 1
      continue
    }
    const prev = rows[i - 1]!
    const cur = rows[i]!
    const same =
      rule === 'acm'
        ? cur.solved === prev.solved && cur.penalty_minutes === prev.penalty_minutes
        : scoreHundredths(cur.total_score) === scoreHundredths(prev.total_score)
    cur.rank = same ? prev.rank : i + 1
  }
}

/* ---------------------------------------------- frozen board filtering */

export interface FrozenInput {
  rule: ContestRule
  now: Date
  endsAt: Date
  freezeFor: number
  published: boolean
  phase: ContestPhase
}

/**
 * Port of `scoring.FilterFrozenStandings` for a non-admin viewer.
 *
 * `StandingProblem` deliberately omits the solve time (the backend marks it
 * `json:"-"`), so the caller passes a lookup for the effective submission's
 * receive time rather than the mock inventing a wire field the real API lacks.
 */
export function filterFrozenStandings(
  inp: FrozenInput,
  viewerEntrantId: string,
  rows: StandingRow[],
  receivedAt: (submissionId: string) => number | undefined,
): StandingRow[] {
  const freezeAt = inp.endsAt.getTime() - inp.freezeFor * 60_000
  const out = rows.map((row) => ({ ...row, problems: row.problems.map((p) => ({ ...p })) }))
  for (const row of out) {
    row.solved = 0
    row.penalty_minutes = 0
    const isViewer = row.entrant_id === viewerEntrantId
    for (const ps of row.problems) {
      if (!ps.effective_submission_id) continue
      const solvedAt = receivedAt(ps.effective_submission_id)
      const afterFreeze = solvedAt !== undefined && solvedAt >= freezeAt
      // The viewer keeps seeing their own post-freeze solves; everyone else's
      // are withheld until the contest is published.
      if (!isViewer && afterFreeze) {
        ps.solved = false
        ps.score = '0.00'
        ps.penalty_minutes = 0
        ps.effective_submission_id = ''
      }
      if (ps.solved) {
        row.solved += 1
        row.penalty_minutes += ps.penalty_minutes
      }
    }
  }
  rank('acm', out)
  return out
}

/* ------------------------------------------------------------- helpers */

export function contestInputFrom(
  contest: MockContest,
  contestProblems: MockContestProblem[],
  entrants: ScoringEntrant[],
  submissions: MockSubmission[],
): ScoringInput {
  const byId = new Map(contestProblems.map((cp) => [cp.id, cp]))
  return {
    rule: contest.rule,
    start: contest.startsAt,
    penaltyMinutes: contest.acmPenaltyMinutes,
    // Scoring keys problems by *bank* problem id, not contest-problem id,
    // matching `scoring.Module.input`.
    problems: contestProblems.map((cp) => ({
      id: cp.problemId,
      letter: cp.letter,
      revisionId: cp.scoringRevisionId,
    })),
    entrants,
    submissions: submissions.map((s) => {
      const cp = byId.get(s.contestProblemId)
      const problemId = cp?.problemId ?? s.contestProblemId
      // `scoring.Module.input` prefers the result on the contest problem's
      // scoring revision and falls back to the submission's own revision. Only
      // the first case counts as `NormalOn`.
      const onScoring = cp ? s.results[cp.scoringRevisionId] : undefined
      if (cp && onScoring) {
        return {
          id: s.id,
          entrantId: s.entrantId,
          problemId,
          revisionId: cp.scoringRevisionId,
          receiveSeq: s.receiveSeq,
          receivedAt: s.receivedAt,
          result: onScoring,
        }
      }
      return {
        id: s.id,
        entrantId: s.entrantId,
        problemId,
        revisionId: s.revisionId,
        receiveSeq: s.receiveSeq,
        receivedAt: s.receivedAt,
        result: s.results[s.revisionId] ?? null,
      }
    }),
  }
}
