import type { Submission } from '~/types/api'

interface Subscriber {
  accountId: string | null
  listener: (submission: Submission) => void
}

/**
 * Stand-in for the SSE stream at `GET /api/contests/:id/events`.
 *
 * `EventSource` cannot be intercepted, so `useContestEvents` subscribes here
 * instead when the mock is on.
 *
 * The projector runs once per subscriber rather than once per frame: the real
 * endpoint projects per connection, so an admin, the submission's owner and an
 * unrelated contestant receive three different payloads for the same submission.
 * Broadcasting one payload would quietly break the visibility rules the UI is
 * built around.
 */
export class MockEventBus {
  private listeners = new Map<string, Set<Subscriber>>()

  subscribe(
    contestId: string,
    accountId: string | null,
    listener: (submission: Submission) => void,
  ): () => void {
    let set = this.listeners.get(contestId)
    if (!set) {
      set = new Set()
      this.listeners.set(contestId, set)
    }
    const entry: Subscriber = { accountId, listener }
    set.add(entry)
    return () => {
      set.delete(entry)
      if (set.size === 0) this.listeners.delete(contestId)
    }
  }

  publish(contestId: string, project: (accountId: string | null) => Submission | null): void {
    const set = this.listeners.get(contestId)
    if (!set) return
    for (const entry of [...set]) {
      try {
        const payload = project(entry.accountId)
        if (payload) entry.listener(payload)
      } catch {
        // One bad subscriber must not stop the others.
      }
    }
  }

  clear(): void {
    this.listeners.clear()
  }
}
