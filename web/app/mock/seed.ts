import { nextId } from './ids'
import { ALL_PROFILES, PROFILE_FOR_LANGUAGE } from './judging'
import type { Verdict } from '~/types/api'
import type {
  MockAccount,
  MockContest,
  MockContestProblem,
  MockEntrant,
  MockOrg,
  MockProblem,
  MockResult,
  MockRevision,
  MockState,
  MockSubmission,
} from './types'

const HOUR = 3600_000
const DAY = 24 * HOUR
const MINUTE = 60_000

/**
 * The demo world.
 *
 * Everything is laid out relative to `now` so the fixture always contains a
 * running contest, one that has not opened, one that is finished and published,
 * and one sitting inside its ACM freeze window. Each of those drives a different
 * branch of the UI, and a fixed fixture would only ever exercise one.
 */
export function seedState(state: MockState, now: Date): void {
  const mk = (kind: string) => nextId(state.idCounters, kind)
  const at = (base: Date, minutes: number) => new Date(base.getTime() + minutes * MINUTE)

  /* ------------------------------------------------------------ accounts */

  const account = (
    email: string,
    displayName: string,
    opts: { verified?: boolean; siteAdmin?: boolean } = {},
  ): MockAccount => {
    const row: MockAccount = {
      id: mk('acct'),
      email,
      password: 'password123',
      displayName,
      emailVerified: opts.verified ?? true,
      siteAdmin: opts.siteAdmin ?? false,
    }
    state.accounts.push(row)
    return row
  }

  const alice = account('alice@example.com', 'Alice Chen', { siteAdmin: true })
  const bob = account('bob@example.com', 'Bob Liu')
  const carol = account('carol@example.com', 'Carol Wang', { verified: false })
  const dave = account('dave@example.com', 'Dave Zhang')
  const erin = account('erin@example.com', 'Erin Zhao')
  // Not a member of any organization: the only account that sees contest
  // problems as letters-only, and the only external registrant.
  const frank = account('frank@example.com', 'Frank Wu')

  /* ------------------------------------------------------- organizations */

  const org = (slug: string, name: string): MockOrg => {
    const row: MockOrg = { id: mk('org'), slug, name }
    state.orgs.push(row)
    return row
  }

  const labs = org('ephyra-labs', 'Ephyra Labs')
  const acme = org('acme', 'ACME University')

  state.orgMembers.push(
    { orgId: labs.id, accountId: alice.id, role: 'owner' },
    { orgId: labs.id, accountId: bob.id, role: 'admin' },
    { orgId: labs.id, accountId: carol.id, role: 'member' },
    { orgId: acme.id, accountId: dave.id, role: 'owner' },
    { orgId: acme.id, accountId: erin.id, role: 'member' },
  )

  // A pending request, so the members page has something to decide.
  state.joinRequests.push({
    id: mk('jreq'),
    orgId: labs.id,
    accountId: erin.id,
    status: 'pending',
    createdAt: new Date(now.getTime() - 2 * DAY),
  })

  /* ------------------------------------------------------------ problems */

  const problem = (orgId: string, slug: string, title: string): MockProblem => {
    const row: MockProblem = { id: mk('pbm'), orgId, slug, title }
    state.problems.push(row)
    return row
  }

  const sumOfTwo = problem(labs.id, 'sum-of-two', 'A + B')
  const maxSubarray = problem(labs.id, 'max-subarray', 'Maximum Subarray')
  const shortestPath = problem(labs.id, 'shortest-path', 'Shortest Path')
  const helloWorld = problem(acme.id, 'hello-world', 'Hello, World')

  const revision = (
    problemId: string,
    n: number,
    statement: string | null,
    subtasks = [
      { id: '1', score: '30.00' },
      { id: '2', score: '30.00' },
      { id: '3', score: '40.00' },
    ],
  ): MockRevision => {
    const row: MockRevision = {
      id: mk('rev'),
      problemId,
      n,
      createdAt: new Date(now.getTime() - (30 - n) * DAY),
      statement,
      kind: 'batch',
      checkerKind: 'token',
      timeLimitMs: 2000,
      memoryLimitBytes: 268_435_456,
      subtasks,
    }
    state.revisions.push(row)
    return row
  }

  const sumStatement = `# A + B

Given two integers $a$ and $b$, print their sum.

## Input

A single line with two integers $a$ and $b$ ($-10^9 \\le a, b \\le 10^9$).

## Output

Print $a + b$ on one line.

## Subtasks

| Subtask | Points | Constraints |
| --- | --- | --- |
| 1 | 30 | $0 \\le a, b \\le 10^3$ |
| 2 | 30 | $a, b \\ge 0$ |
| 3 | 40 | no additional constraints |

## Example

\`\`\`
3 4
\`\`\`

\`\`\`
7
\`\`\`

Beware of overflow: the sum does not fit in a 32-bit signed integer for the
largest inputs.
`

  const maxSubarrayStatement = `# Maximum Subarray

Find the largest possible sum of a **contiguous** subarray.

## Input

- Line 1: $n$ ($1 \\le n \\le 2 \\cdot 10^5$)
- Line 2: $n$ integers $a_i$ ($-10^4 \\le a_i \\le 10^4$)

## Output

A single integer — the maximum subarray sum. The subarray must be non-empty.

## Example

\`\`\`
5
-2 1 -3 4 -1
\`\`\`

\`\`\`
4
\`\`\`

The subarray $[4]$ scores 4, and $[4, -1]$ also scores 3, so 4 is the answer.
`

  const shortestPathStatement = `# Shortest Path

Given a weighted directed graph, answer $q$ shortest-path queries from vertex 1.

## Input

- Line 1: $n$, $m$, $q$
- Next $m$ lines: $u\\ v\\ w$ — an edge with weight $w \\ge 0$
- Next $q$ lines: a target vertex

## Output

For each query, print the distance from vertex 1, or \`-1\` if unreachable.

Weights are non-negative, so Dijkstra's algorithm applies. The intended solution
runs in $O((n + m) \\log n)$ per test.
`

  const sumR1 = revision(sumOfTwo.id, 1, sumStatement)
  const sumR2 = revision(
    sumOfTwo.id,
    2,
    `${sumStatement}\n## Changelog\n\nRevision 2 clarifies the overflow warning.\n`,
  )
  const maxR1 = revision(maxSubarray.id, 1, maxSubarrayStatement)
  const maxR2 = revision(maxSubarray.id, 2, maxSubarrayStatement)
  // A revision with no statement exercises the "题面缺失" branch.
  const shortR1 = revision(shortestPath.id, 1, null)
  const shortR2 = revision(shortestPath.id, 2, shortestPathStatement)
  const helloR1 = revision(helloWorld.id, 1, null, [{ id: '1', score: '100.00' }])

  /* ------------------------------------------------------------ contests */

  const contest = (
    orgId: string,
    slug: string,
    title: string,
    opts: Partial<Omit<MockContest, 'id' | 'orgId' | 'slug' | 'title'>>,
  ): MockContest => {
    const row: MockContest = {
      id: mk('ctst'),
      orgId,
      slug,
      title,
      rule: 'acm',
      entryMode: 'individual',
      startsAt: at(now, -120),
      endsAt: at(now, 240),
      registerStartsAt: at(now, -7 * 24 * 60),
      allowExternal: false,
      registrationRequiresReview: false,
      maxTeamSize: 1,
      acmPenaltyMinutes: 20,
      freezeMinutesBeforeEnd: null,
      ioiPublicScoreboard: false,
      caseFeedback: 'subtask',
      sourcePublicAfter: false,
      publishedAt: null,
      sourceVisibilityLocked: true,
      ...opts,
    }
    state.contests.push(row)
    return row
  }

  // Running ACM, comfortably inside its window: the main demo contest.
  const winter = contest(labs.id, 'winter-2026', 'Winter Contest 2026', {
    startsAt: at(now, -120),
    endsAt: at(now, 240),
    allowExternal: true,
    sourcePublicAfter: true,
  })

  // Registration open, teams, review required: drives the review queue.
  const spring = contest(labs.id, 'spring-oi', 'Spring OI', {
    rule: 'oi',
    entryMode: 'team',
    startsAt: at(now, 5 * 24 * 60),
    endsAt: at(now, 5 * 24 * 60 + 300),
    registerStartsAt: at(now, -24 * 60),
    registrationRequiresReview: true,
    maxTeamSize: 3,
    caseFeedback: 'case',
    sourcePublicAfter: false,
  })

  // Finished and published: drives the IOI board and public source access.
  const summer = contest(labs.id, 'summer-ioi', 'Summer IOI', {
    rule: 'ioi',
    startsAt: at(now, -30 * 24 * 60),
    endsAt: at(now, -30 * 24 * 60 + 300),
    ioiPublicScoreboard: true,
    sourcePublicAfter: true,
    publishedAt: at(now, -29 * 24 * 60),
  })

  // Inside its ACM freeze window: non-admins must not see others' late solves.
  const intro = contest(acme.id, 'intro-2026', 'Intro to Programming', {
    startsAt: at(now, -300),
    endsAt: at(now, 25),
    freezeMinutesBeforeEnd: 60,
    allowExternal: true,
  })

  // Registration open, individual, external entrants welcome: the one contest
  // where a stranger can still sign up.
  const autumn = contest(labs.id, 'autumn-acm', 'Autumn ACM', {
    startsAt: at(now, 2 * 24 * 60),
    endsAt: at(now, 2 * 24 * 60 + 300),
    registerStartsAt: at(now, -24 * 60),
    allowExternal: true,
  })

  /* --------------------------------------------------- contest problems */

  const contestProblem = (
    contestId: string,
    problemId: string,
    letter: string,
    rev: MockRevision,
  ): MockContestProblem => {
    const row: MockContestProblem = {
      id: mk('cpbm'),
      contestId,
      problemId,
      letter,
      scoringRevisionId: rev.id,
      targetRevisionId: rev.id,
    }
    state.contestProblems.push(row)
    return row
  }

  const winterA = contestProblem(winter.id, sumOfTwo.id, 'A', sumR1)
  const winterB = contestProblem(winter.id, maxSubarray.id, 'B', maxR1)
  const winterC = contestProblem(winter.id, shortestPath.id, 'C', shortR1)
  const springA = contestProblem(spring.id, sumOfTwo.id, 'A', sumR1)
  const springB = contestProblem(spring.id, maxSubarray.id, 'B', maxR1)
  const summerA = contestProblem(summer.id, sumOfTwo.id, 'A', sumR2)
  const summerB = contestProblem(summer.id, maxSubarray.id, 'B', maxR2)
  const introA = contestProblem(intro.id, helloWorld.id, 'A', helloR1)
  const autumnA = contestProblem(autumn.id, sumOfTwo.id, 'A', sumR1)
  const autumnB = contestProblem(autumn.id, shortestPath.id, 'B', shortR2)

  /* ------------------------------------------------------------ entrants */

  const entrant = (
    contestId: string,
    kind: 'individual' | 'team',
    displayName: string,
    status: MockEntrant['status'],
    registeredAt: Date,
  ): MockEntrant => {
    const row: MockEntrant = {
      id: mk('entr'),
      contestId,
      kind,
      displayName,
      status,
      registeredAt,
      acceptedAt: status === 'accepted' ? registeredAt : null,
      rosterFrozenAt: status === 'accepted' ? registeredAt : null,
    }
    state.entrants.push(row)
    return row
  }

  const member = (
    en: MockEntrant,
    accountId: string,
    role: 'captain' | 'member',
    confirmed = true,
  ) => {
    state.entrantMembers.push({
      contestId: en.contestId,
      entrantId: en.id,
      accountId,
      role,
      confirmedAt: confirmed ? en.registeredAt : null,
    })
  }

  const winterAlice = entrant(winter.id, 'individual', 'Alice Chen', 'accepted', at(now, -150))
  const winterBob = entrant(winter.id, 'individual', 'Bob Liu', 'accepted', at(now, -145))
  const winterCarol = entrant(winter.id, 'individual', 'Carol Wang', 'accepted', at(now, -140))
  const winterFrank = entrant(winter.id, 'individual', 'Frank Wu', 'accepted', at(now, -130))
  member(winterAlice, alice.id, 'captain')
  member(winterBob, bob.id, 'captain')
  member(winterCarol, carol.id, 'captain')
  member(winterFrank, frank.id, 'captain')

  const springRocket = entrant(spring.id, 'team', 'Team Rocket', 'pending', at(now, -300))
  const springNull = entrant(spring.id, 'team', 'Null Pointer', 'accepted', at(now, -400))
  const springSegv = entrant(spring.id, 'team', 'Segfault Squad', 'pending', at(now, -200))
  member(springRocket, alice.id, 'captain')
  member(springNull, bob.id, 'captain')
  member(springNull, carol.id, 'member')
  member(springSegv, dave.id, 'captain')
  // An unconfirmed invitation, so the "待确认" state is reachable.
  member(springRocket, erin.id, 'member', false)

  const summerAlice = entrant(
    summer.id,
    'individual',
    'Alice Chen',
    'accepted',
    at(now, -31 * 24 * 60),
  )
  const summerBob = entrant(summer.id, 'individual', 'Bob Liu', 'accepted', at(now, -31 * 24 * 60))
  member(summerAlice, alice.id, 'captain')
  member(summerBob, bob.id, 'captain')

  const introDave = entrant(intro.id, 'individual', 'Dave Zhang', 'accepted', at(now, -320))
  const introErin = entrant(intro.id, 'individual', 'Erin Zhao', 'accepted', at(now, -310))
  member(introDave, dave.id, 'captain')
  member(introErin, erin.id, 'captain')

  /* -------------------------------------------------- judge infrastructure */

  const systemPool = {
    id: mk('pool'),
    kind: 'system' as const,
    orgId: null,
    name: 'system',
    enabled: true,
  }
  const labsPool = {
    id: mk('pool'),
    kind: 'organization' as const,
    orgId: labs.id,
    name: 'ephyra-judge',
    enabled: true,
  }
  const acmePool = {
    id: mk('pool'),
    kind: 'organization' as const,
    orgId: acme.id,
    name: 'acme-lab',
    enabled: true,
  }
  state.pools.push(systemPool, labsPool, acmePool)

  const node = (
    poolId: string,
    name: string,
    opts: Partial<{
      enabled: boolean
      connected: boolean
      revoked: boolean
      maxSlots: number
    }> = {},
  ) => {
    const row = {
      id: mk('node'),
      poolId,
      name,
      enabled: opts.enabled ?? true,
      connected: opts.connected ?? true,
      revoked: opts.revoked ?? false,
      maxSlots: opts.maxSlots ?? 4,
      lastHeartbeatAt: (opts.connected ?? true) ? new Date(now.getTime() - 5_000) : null,
      advertisedProfiles: [...ALL_PROFILES],
      credential: `node-${name}-secret`,
    }
    state.nodes.push(row)
    return row
  }

  node(systemPool.id, 'sys-1')
  node(systemPool.id, 'sys-2', { maxSlots: 8 })
  const labsNode = node(labsPool.id, 'ephyra-1')
  node(labsPool.id, 'ephyra-2', { connected: false })
  node(labsPool.id, 'ephyra-3', { revoked: true, enabled: false })
  node(acmePool.id, 'acme-1', { maxSlots: 2 })

  state.routing.push(
    { orgId: labs.id, poolId: labsPool.id, priority: 1 },
    { orgId: labs.id, poolId: systemPool.id, priority: 10 },
    { orgId: acme.id, poolId: acmePool.id, priority: 1 },
    { orgId: acme.id, poolId: systemPool.id, priority: 5 },
  )

  /* --------------------------------------------------------- submissions */

  const result = (verdict: Verdict, score: string, opts: Partial<MockResult> = {}): MockResult => ({
    verdict,
    score,
    compileFailed: verdict === 'compilation_error',
    systemFault: verdict === 'system_error',
    subtasks: [],
    cases: [],
    ...opts,
  })

  /** Subtasks that pass the first `passed` entries and fail the rest. */
  const partial = (rev: MockRevision, passed: number, verdict: Verdict): MockResult => {
    let total = 0
    const subtasks = rev.subtasks.map((s, i) => {
      const ok = i < passed
      if (ok) total += Math.round(Number.parseFloat(s.score) * 100)
      return { id: s.id, score: ok ? s.score : '0.00', verdict: ok ? 'accepted' : verdict }
    })
    return result(verdict, `${Math.floor(total / 100)}.${String(total % 100).padStart(2, '0')}`, {
      subtasks,
    })
  }

  const acceptedWith = (rev: MockRevision): MockResult =>
    partial(rev, rev.subtasks.length, 'accepted')

  const cpp = (body: string) =>
    `#include <bits/stdc++.h>\nusing namespace std;\nint main() {\n${body}\n}\n`

  interface Spec {
    cp: MockContestProblem
    en: MockEntrant
    account: MockAccount
    minutes: number
    result: MockResult | null
    source?: string
  }

  const specs: Spec[] = [
    // Winter: Alice solves A after one failed attempt, then B.
    {
      cp: winterA,
      en: winterAlice,
      account: alice,
      minutes: 5,
      result: result('wrong_answer', '0.00'),
      source: cpp('  long long a, b; cin >> a >> b; cout << a + b;'),
    },
    {
      cp: winterA,
      en: winterAlice,
      account: alice,
      minutes: 12,
      result: acceptedWith(sumR1),
      source: cpp('  long long a, b; cin >> a >> b; cout << a + b << "\\n";'),
    },
    {
      cp: winterB,
      en: winterAlice,
      account: alice,
      minutes: 25,
      result: acceptedWith(maxR1),
      source: cpp(
        '  int n; cin >> n; long long best = LLONG_MIN, cur = 0;\n  for (int i = 0; i < n; i++) { long long x; cin >> x; cur = max(x, cur + x); best = max(best, cur); }\n  cout << best << "\\n";',
      ),
    },

    // Winter: Bob grinds B down through three failures.
    {
      cp: winterA,
      en: winterBob,
      account: bob,
      minutes: 8,
      result: acceptedWith(sumR1),
      source: cpp('  int a, b; cin >> a >> b; cout << a + b << "\\n";'),
    },
    {
      cp: winterB,
      en: winterBob,
      account: bob,
      minutes: 20,
      result: result('wrong_answer', '0.00'),
      source: cpp('  // only the positive prefix\n  cout << 0 << "\\n";'),
    },
    {
      cp: winterB,
      en: winterBob,
      account: bob,
      minutes: 30,
      result: result('time_limit_exceeded', '0.00'),
      source: cpp('  // O(n^2) brute force\n  for (int i = 0; i < 1e9; i++) {}'),
    },
    {
      cp: winterB,
      en: winterBob,
      account: bob,
      minutes: 40,
      result: acceptedWith(maxR1),
      source: cpp('  // Kadane\n  long long best = LLONG_MIN, cur = 0; (void)best; (void)cur;'),
    },
    // A system fault: reduced to a bare verdict for its submitter, and withheld
    // entirely from anyone whose feedback is below full. It leaves C pending.
    {
      cp: winterC,
      en: winterBob,
      account: bob,
      minutes: 50,
      result: result('system_error', '0.00'),
      source: cpp('  // dijkstra\n  return 0;'),
    },
    {
      cp: winterC,
      en: winterFrank,
      account: frank,
      minutes: 52,
      result: result('system_error', '0.00'),
      source: cpp('  // dijkstra, take two\n  return 0;'),
    },

    // Winter: Carol solves A, fails B on a compile error (which is not a penalty),
    // then leaves B pending.
    {
      cp: winterA,
      en: winterCarol,
      account: carol,
      minutes: 30,
      result: acceptedWith(sumR1),
      source: cpp('  long long a, b; cin >> a >> b; cout << a + b;'),
    },
    {
      cp: winterB,
      en: winterCarol,
      account: carol,
      minutes: 35,
      result: result('compilation_error', '0.00'),
      source: '#include <bits/stdc++.h>\nint main() { int x = "not an int"; }\n',
    },
    {
      cp: winterB,
      en: winterCarol,
      account: carol,
      minutes: 55,
      result: null,
      source: cpp('  // still judging\n  return 0;'),
    },

    // Summer (IOI, published): partial credit shows up as a score board.
    {
      cp: summerA,
      en: summerAlice,
      account: alice,
      minutes: 20,
      result: acceptedWith(sumR2),
      source: cpp('  long long a, b; cin >> a >> b; cout << a + b;'),
    },
    {
      cp: summerB,
      en: summerAlice,
      account: alice,
      minutes: 60,
      result: partial(maxR2, 2, 'wrong_answer'),
      source: cpp('  // passes the small subtasks only\n  return 0;'),
    },
    {
      cp: summerA,
      en: summerBob,
      account: bob,
      minutes: 35,
      result: acceptedWith(sumR2),
      source: cpp('  long long a, b; cin >> a >> b; cout << a + b;'),
    },
    {
      cp: summerB,
      en: summerBob,
      account: bob,
      minutes: 90,
      result: partial(maxR2, 1, 'time_limit_exceeded'),
      source: cpp('  // O(n^2)\n  return 0;'),
    },

    // Intro (ACM, currently frozen): Erin solved before the freeze, Dave after it.
    {
      cp: introA,
      en: introErin,
      account: erin,
      minutes: 250,
      result: result('wrong_answer', '0.00'),
      source: cpp('  cout << "hello";'),
    },
    {
      cp: introA,
      en: introErin,
      account: erin,
      minutes: 255,
      result: acceptedWith(helloR1),
      source: cpp('  cout << "Hello, World!" << "\\n";'),
    },
    {
      cp: introA,
      en: introDave,
      account: dave,
      minutes: 290,
      result: acceptedWith(helloR1),
      source: cpp('  cout << "Hello, World!" << "\\n";'),
    },
  ]

  for (const spec of specs) {
    state.submissions.push({
      id: mk('subm'),
      contestId: spec.cp.contestId,
      contestProblemId: spec.cp.id,
      entrantId: spec.en.id,
      submitterAccountId: spec.account.id,
      languageId: 'cpp23',
      receivedAt: at(
        state.contests.find((c) => c.id === spec.cp.contestId)!.startsAt,
        spec.minutes,
      ),
      receiveSeq: 0,
      revisionId: spec.cp.targetRevisionId,
      source: spec.source ?? cpp('  return 0;'),
      results: spec.result ? { [spec.cp.targetRevisionId]: spec.result } : {},
    })
  }

  // `receive_seq` is global and follows receive order, like the real sequence.
  const ordered = [...state.submissions].sort(
    (a, b) => a.receivedAt.getTime() - b.receivedAt.getTime(),
  )
  ordered.forEach((s, i) => {
    s.receiveSeq = i + 1
  })
  state.receiveSeq = ordered.length

  /* ------------------------------------------- tasks and attempts per submission */

  const profile = PROFILE_FOR_LANGUAGE.cpp23!
  for (const sub of state.submissions) {
    const taskId = mk('task')
    const seeded = Object.values(sub.results)[0]
    const awaitingAdmin = seeded?.systemFault === true
    state.tasks.push({
      id: taskId,
      submissionId: sub.id,
      contestId: sub.contestId,
      revisionId: sub.revisionId,
      profile,
      purpose: 'initial',
      state: seeded === undefined ? 'dispatched' : awaitingAdmin ? 'awaiting_admin' : 'completed',
      createdAt: sub.receivedAt,
      batchId: null,
    })
    const attemptId = mk('atmp')
    state.attempts.push({
      id: attemptId,
      taskId,
      generation: 1,
      nodeId: labsNode.id,
      poolId: labsPool.id,
      state: seeded === undefined ? 'dispatched' : awaitingAdmin ? 'failed' : 'succeeded',
      dispatchedAt: sub.receivedAt,
      leaseUntil: new Date(sub.receivedAt.getTime() + 5 * MINUTE),
      finishedAt: seeded === undefined ? null : new Date(sub.receivedAt.getTime() + MINUTE),
      systemFault: awaitingAdmin,
    })
  }

  /* -------------------------------------------------------------- tokens */

  // The backend mails a bare token, so the mock hands them out through
  // `GET /api/_mock/tokens` instead of pretending an email was sent.
  state.tokens.push(
    {
      token: 'mock-verify-carol',
      accountId: carol.id,
      kind: 'verify',
      expiresAt: new Date(now.getTime() + 7 * DAY),
    },
    {
      token: 'mock-reset-bob',
      accountId: bob.id,
      kind: 'reset',
      expiresAt: new Date(now.getTime() + DAY),
    },
  )
}
