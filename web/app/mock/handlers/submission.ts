import { projectFor, requireAccount, requireContest, visibilityFor } from '../access'
import { conflict, forbidden, invalid, notFound, unverified } from '../errors'
import { nextId } from '../ids'
import { dispatchTask, judgeDepsOf, PROFILE_FOR_LANGUAGE } from '../judging'
import { shouldEmit } from '../scoring'
import { json, text } from '../types'
import type { LanguageID, Submission } from '~/types/api'
import type { MockContext, MockRoute, MockSubmission } from '../types'

const LANGUAGES: LanguageID[] = ['cpp23', 'c17', 'c23']

function requireSubmission(ctx: MockContext, id: string): MockSubmission {
  const submission = ctx.state.submissions.find((s) => s.id === id)
  if (!submission) throw notFound('submission not found')
  return submission
}

/** Reads the multipart body of a submission or an import. */
async function formFields(ctx: MockContext): Promise<{ language: string; source: string }> {
  if (typeof FormData === 'undefined' || !(ctx.body instanceof FormData)) {
    return { language: '', source: '' }
  }
  const language = ctx.body.get('language')
  const file = ctx.body.get('source')
  let source = ''
  if (file && typeof file === 'object' && 'text' in file) {
    source = await (file as File).text()
  } else if (typeof file === 'string') {
    source = file
  }
  return { language: typeof language === 'string' ? language : '', source }
}

function numberQuery(value: unknown): number | null {
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : null
  }
  if (typeof value === 'number') return value
  return null
}

export const submissionRoutes: MockRoute[] = [
  {
    method: 'POST',
    pattern: '/api/contests/:id/problems/:letter/submissions',
    handler: async (ctx) => {
      const account = requireAccount(ctx)
      const contest = requireContest(ctx.state, ctx.params.id!)
      const { language, source } = await formFields(ctx)
      if (!source) throw invalid('source is empty or too large')
      if (!LANGUAGES.includes(language as LanguageID)) throw invalid('unsupported language')
      if (!account.emailVerified) throw unverified()

      const membership = ctx.state.entrantMembers.find(
        (m) => m.contestId === contest.id && m.accountId === account.id,
      )
      if (!membership) throw forbidden('not registered')
      const entrant = ctx.state.entrants.find((e) => e.id === membership.entrantId)
      if (!entrant) throw forbidden('not registered')
      if (entrant.status !== 'accepted') throw forbidden('entrant is not accepted')
      const running =
        ctx.now.getTime() >= contest.startsAt.getTime() &&
        ctx.now.getTime() < contest.endsAt.getTime()
      if (!running) throw conflict('contest is not running')

      const cp = ctx.state.contestProblems.find(
        (c) => c.contestId === contest.id && c.letter === ctx.params.letter,
      )
      if (!cp) throw notFound('contest problem not found')

      ctx.state.receiveSeq += 1
      const submission: MockSubmission = {
        id: nextId(ctx.state.idCounters, 'subm'),
        contestId: contest.id,
        contestProblemId: cp.id,
        entrantId: entrant.id,
        submitterAccountId: account.id,
        languageId: language as LanguageID,
        receivedAt: ctx.now,
        receiveSeq: ctx.state.receiveSeq,
        // Submissions are judged on the contest problem's *target* revision.
        revisionId: cp.targetRevisionId,
        source,
        results: {},
      }
      ctx.state.submissions.push(submission)

      const task = {
        id: nextId(ctx.state.idCounters, 'task'),
        submissionId: submission.id,
        contestId: contest.id,
        revisionId: cp.targetRevisionId,
        profile: PROFILE_FOR_LANGUAGE[language] ?? PROFILE_FOR_LANGUAGE.cpp23!,
        purpose: 'initial',
        state: 'pending_schedule' as const,
        createdAt: ctx.now,
        batchId: null,
      }
      ctx.state.tasks.push(task)
      dispatchTask(judgeDepsOf(ctx), task)

      return json(
        {
          submission_id: submission.id,
          receive_seq: submission.receiveSeq,
          received_at: submission.receivedAt.toISOString(),
          task_id: task.id,
        },
        202,
      )
    },
  },
  {
    method: 'GET',
    pattern: '/api/contests/:id/submissions',
    handler: (ctx) => {
      const account = requireAccount(ctx)
      const contest = requireContest(ctx.state, ctx.params.id!)

      let contestProblemId: string | null = null
      const letter = typeof ctx.req.query.letter === 'string' ? ctx.req.query.letter.trim() : ''
      if (letter) {
        const cp = ctx.state.contestProblems.find(
          (c) => c.contestId === contest.id && c.letter === letter,
        )
        if (!cp) throw notFound('contest problem not found')
        contestProblemId = cp.id
      }
      const entrantId =
        typeof ctx.req.query.entrant_id === 'string' ? ctx.req.query.entrant_id : null
      const rawLimit = numberQuery(ctx.req.query.limit)
      const limit = rawLimit === null ? 50 : Math.min(200, Math.max(1, rawLimit))
      let before = numberQuery(ctx.req.query.before_seq)

      const names = new Map(
        ctx.state.entrants
          .filter((e) => e.contestId === contest.id)
          .map((e) => [e.id, e.displayName]),
      )

      // The limit applies to *visible* rows, not candidates: hidden newer
      // submissions must not stop a viewer paging through their own older work.
      const batchSize = Math.max(50, limit)
      const out: Submission[] = []
      for (;;) {
        let candidates = ctx.state.submissions.filter((s) => s.contestId === contest.id)
        if (entrantId) candidates = candidates.filter((s) => s.entrantId === entrantId)
        if (contestProblemId) {
          candidates = candidates.filter((s) => s.contestProblemId === contestProblemId)
        }
        if (before !== null) candidates = candidates.filter((s) => s.receiveSeq < before!)
        candidates = candidates.sort((a, b) => b.receiveSeq - a.receiveSeq).slice(0, batchSize)

        for (const candidate of candidates) {
          const { submission, view } = projectFor(ctx.state, contest, candidate, account, ctx.now)
          if (!shouldEmit(view)) continue
          submission.entrant_display_name = names.get(submission.entrant_id)
          out.push(submission)
          if (out.length === limit) return json({ submissions: out })
        }
        if (candidates.length < batchSize) return json({ submissions: out })
        before = candidates[candidates.length - 1]!.receiveSeq
      }
    },
  },
  {
    method: 'GET',
    pattern: '/api/submissions/:id',
    handler: (ctx) => {
      const submission = requireSubmission(ctx, ctx.params.id!)
      const contest = requireContest(ctx.state, submission.contestId)
      // The single-submission view does not resolve entrant names.
      return json(projectFor(ctx.state, contest, submission, ctx.account, ctx.now).submission)
    },
  },
  {
    method: 'GET',
    pattern: '/api/submissions/:id/source',
    handler: (ctx) => {
      const submission = requireSubmission(ctx, ctx.params.id!)
      const contest = requireContest(ctx.state, submission.contestId)
      const view = visibilityFor(ctx.state, contest, submission, ctx.account, ctx.now)
      if (!view.showSource) {
        throw forbidden('source is not visible')
      }
      return text(submission.source)
    },
  },
]
