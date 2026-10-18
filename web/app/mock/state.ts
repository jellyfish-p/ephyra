import type { MockState } from './types'
import { seedState } from './seed'

export function emptyState(fixedNow: Date | null): MockState {
  return {
    fixedNow,
    accounts: [],
    orgs: [],
    orgMembers: [],
    joinRequests: [],
    contests: [],
    problems: [],
    revisions: [],
    contestProblems: [],
    entrants: [],
    entrantMembers: [],
    submissions: [],
    pools: [],
    nodes: [],
    routing: [],
    tasks: [],
    attempts: [],
    rejudgeBatches: [],
    tokens: [],
    receiveSeq: 0,
    idCounters: {},
  }
}

/**
 * Builds a fully seeded world. Contest times are laid out relative to `now`, so
 * the demo always has a contest that is running, one that has not started, and
 * one that is finished and published.
 */
export function createMockState(now: Date = new Date(), frozen = false): MockState {
  const state = emptyState(frozen ? now : null)
  seedState(state, now)
  return state
}

/** The mock's clock. Frozen in tests, the wall clock otherwise. */
export function stateNow(state: MockState): Date {
  return state.fixedNow ?? new Date()
}
