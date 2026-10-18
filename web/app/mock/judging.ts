import type { FeedbackGranularity, Verdict } from '~/types/api'
import { projectFor } from './access'
import { nextId } from './ids'
import { formatScore, shouldEmit } from './scoring'
import type {
  MockAttempt,
  MockContext,
  MockNode,
  MockRevision,
  MockResult,
  MockRouting,
  MockState,
  MockTask,
} from './types'

/**
 * The judge simulator.
 *
 * A submission is not judged instantly: the mock creates a task and an attempt,
 * then completes them a moment later, exactly like the real pipeline. That delay
 * is what makes the UI's pending states observable — the "评测中" badge, the
 * `…` cell on an OI scoreboard, and the SSE frame that replaces them.
 *
 * The verdict is *controllable*. Put a directive in the submitted source:
 *
 *     // @verdict=tle
 *
 * and the mock will return that verdict. Without a directive the verdict is
 * derived from a hash of the source, so the same program always judges the same
 * way and demo data does not shuffle between reloads.
 */

const DIRECTIVES: Record<string, Verdict> = {
  ac: 'accepted',
  accepted: 'accepted',
  wa: 'wrong_answer',
  wrong: 'wrong_answer',
  wrong_answer: 'wrong_answer',
  tle: 'time_limit_exceeded',
  time_limit_exceeded: 'time_limit_exceeded',
  mle: 'memory_limit_exceeded',
  memory_limit_exceeded: 'memory_limit_exceeded',
  rte: 'runtime_error',
  runtime_error: 'runtime_error',
  ce: 'compilation_error',
  compile_error: 'compilation_error',
  compilation_error: 'compilation_error',
  sys: 'system_error',
  system_error: 'system_error',
}

/** FNV-1a, so the same source always produces the same result. */
export function fnv1a(text: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i)
    h = Math.imul(h, 0x01000193) >>> 0
  }
  return h >>> 0
}

/** Reads `@verdict=<name>` from the source, if present. */
export function verdictDirective(source: string): Verdict | null {
  const match = /@verdict\s*[=:]\s*([a-z_]+)/i.exec(source)
  if (!match?.[1]) return null
  return DIRECTIVES[match[1].toLowerCase()] ?? null
}

function fallbackVerdict(hash: number): Verdict {
  const roll = hash % 100
  if (roll < 55) return 'accepted'
  if (roll < 75) return 'wrong_answer'
  if (roll < 85) return 'time_limit_exceeded'
  if (roll < 92) return 'memory_limit_exceeded'
  if (roll < 97) return 'runtime_error'
  return 'compilation_error'
}

/**
 * Builds a result for a submission. Scores are hundredths in decimal strings,
 * like `exact.Score`.
 */
export function deriveResult(
  source: string,
  revision: MockRevision,
  granularity: FeedbackGranularity,
): MockResult {
  // The revision id is part of the hash, so rejudging on different judge data
  // can produce a different verdict — which is the whole point of a rejudge.
  const hash = fnv1a(`${source}\u0000${revision.id}`)
  const verdict = verdictDirective(source) ?? fallbackVerdict(hash)
  if (verdict === 'compilation_error') {
    return {
      verdict,
      score: '0.00',
      compileFailed: true,
      systemFault: false,
      subtasks: [],
      cases: [],
    }
  }
  if (verdict === 'system_error') {
    return {
      verdict,
      score: '0.00',
      compileFailed: false,
      systemFault: true,
      subtasks: [],
      cases: [],
    }
  }

  const subtasks = revision.subtasks.length ? revision.subtasks : [{ id: '1', score: '100.00' }]
  const failAt = verdict === 'accepted' ? -1 : hash % subtasks.length

  let total = 0
  const subtaskOut = subtasks.map((s, i) => {
    const passed = failAt === -1 || i < failAt
    if (passed) total += Math.round(Number.parseFloat(s.score) * 100)
    return {
      id: s.id,
      score: passed ? s.score : '0.00',
      verdict: passed ? 'accepted' : verdict,
    }
  })

  const cases =
    granularity !== 'case'
      ? []
      : Array.from({ length: 3 + (hash % 4) }, (_, i) => ({
          case_id: String(i + 1),
          verdict: failAt === -1 || i < failAt ? 'accepted' : verdict,
        }))

  return {
    verdict,
    score: formatScore(total),
    compileFailed: false,
    systemFault: false,
    subtasks: subtaskOut,
    cases,
  }
}

/* -------------------------------------------------------------- dispatch */

/** `execution` profile ids; a node must advertise the exact id. */
export const PROFILE_FOR_LANGUAGE: Record<string, string> = {
  cpp23: 'cpp23.gcc.linux-amd64.v0',
  c17: 'c17.gcc.linux-amd64.v0',
  c23: 'c23.gcc.linux-amd64.v0',
}

export const ALL_PROFILES: string[] = Object.values(PROFILE_FOR_LANGUAGE)

/** First enabled, connected node in the organization's routing order. */
export function pickNode(
  state: MockState,
  orgId: string,
  profile: string,
): { node: MockNode; poolId: string } | null {
  const entries: MockRouting[] = state.routing
    .filter((r) => r.orgId === orgId)
    .sort((a, b) => a.priority - b.priority || (a.poolId < b.poolId ? -1 : 1))
  for (const entry of entries) {
    const pool = state.pools.find((p) => p.id === entry.poolId)
    if (!pool || !pool.enabled) continue
    const node = state.nodes.find(
      (n) =>
        n.poolId === pool.id &&
        n.enabled &&
        n.connected &&
        !n.revoked &&
        n.advertisedProfiles.includes(profile),
    )
    if (node) return { node, poolId: pool.id }
  }
  return null
}

/**
 * Pending judge work, so a reset (or a test) can drop it instead of leaking
 * timers across runs.
 */
export class JudgeScheduler {
  private timers = new Set<ReturnType<typeof setTimeout>>()

  schedule(delayMs: number, run: () => void): void {
    const timer = setTimeout(() => {
      this.timers.delete(timer)
      run()
    }, delayMs)
    this.timers.add(timer)
  }

  cancelAll(): void {
    for (const timer of this.timers) clearTimeout(timer)
    this.timers.clear()
  }
}

/* ------------------------------------------------------------ completion */

/** How long a submission stays "评测中" before the result lands. */
export const JUDGE_DELAY_MS = 1200

export interface JudgeDeps {
  state: MockState
  now: () => Date
  schedule: (delayMs: number, run: () => void) => void
  publish: (
    contestId: string,
    project: (accountId: string | null) => import('~/types/api').Submission | null,
  ) => void
}

export function judgeDepsOf(ctx: MockContext): JudgeDeps {
  return {
    state: ctx.state,
    now: ctx.runtime.now,
    schedule: ctx.runtime.schedule,
    publish: ctx.runtime.publish,
  }
}

/**
 * Sends a task to a node and books the completion for later.
 *
 * Nothing is judged synchronously: the delay is what makes the pending badge,
 * the `…` scoreboard cell and the SSE update observable in the UI.
 */
export function dispatchTask(
  deps: JudgeDeps,
  task: MockTask,
  delayMs: number = JUDGE_DELAY_MS,
): void {
  const contest = deps.state.contests.find((c) => c.id === task.contestId)
  if (!contest) return
  const pick = pickNode(deps.state, contest.orgId, task.profile)
  if (!pick) {
    // No node advertises the profile: exactly the state an admin has to fix.
    task.state = 'scheduling_error'
    return
  }
  const now = deps.now()
  const generation = deps.state.attempts.filter((a) => a.taskId === task.id).length + 1
  task.state = 'dispatched'
  const attempt: MockAttempt = {
    id: nextId(deps.state.idCounters, 'atmp'),
    taskId: task.id,
    generation,
    nodeId: pick.node.id,
    poolId: pick.poolId,
    state: 'dispatched',
    dispatchedAt: now,
    leaseUntil: new Date(now.getTime() + 5 * 60_000),
    finishedAt: null,
    systemFault: false,
  }
  deps.state.attempts.push(attempt)
  deps.schedule(delayMs, () => completeTask(deps, task, attempt))
}

/**
 * Lands a result on the task's revision and pushes one frame per viewer.
 *
 * The result is stored *per revision* and the submission's accepted revision is
 * left alone, so an open rejudge does not disturb the standing scoreboard.
 */
export function completeTask(deps: JudgeDeps, task: MockTask, attempt: MockAttempt): void {
  // Already settled: a timer that outlived a forced completion is a no-op.
  if (task.state === 'completed') return
  const submission = deps.state.submissions.find((s) => s.id === task.submissionId)
  const revision = deps.state.revisions.find((r) => r.id === task.revisionId)
  const contest = deps.state.contests.find((c) => c.id === task.contestId)
  if (!submission || !revision || !contest) return

  const result = deriveResult(submission.source, revision, contest.caseFeedback)
  submission.results[task.revisionId] = result

  const now = deps.now()
  attempt.finishedAt = now
  attempt.systemFault = result.systemFault
  attempt.state = result.systemFault ? 'failed' : 'succeeded'
  // A system fault is an administrator's problem, never an automatic retry.
  task.state = result.systemFault ? 'awaiting_admin' : 'completed'

  deps.publish(contest.id, (accountId) => {
    const account = accountId ? (deps.state.accounts.find((a) => a.id === accountId) ?? null) : null
    const { submission: projected, view } = projectFor(
      deps.state,
      contest,
      submission,
      account,
      now,
    )
    return shouldEmit(view) ? projected : null
  })
}
