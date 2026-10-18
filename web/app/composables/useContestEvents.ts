import { mockSubscribe } from '~/mock'
import type { Submission } from '~/types/api'

/**
 * Live contest submission feed.
 *
 * `GET /api/contests/:id/events` emits a `submission` event whose payload is the
 * same projection as `GET /api/submissions/:id`, so it is already filtered by
 * the viewer's visibility rules. The browser reconnects automatically; the
 * backend only replays a frame when a submission's fingerprint changes.
 *
 * In mock mode `EventSource` cannot be intercepted, so the mock's event bus
 * stands in — with the same per-viewer projection, so the visibility rules still
 * hold.
 */
export function useContestEvents(contestId: MaybeRefOrGetter<string>) {
  const api = useApi()
  const mock = useMockEnabled()
  const submissions = ref<Map<string, Submission>>(new Map())
  const connected = ref(false)
  let source: EventSource | null = null
  let unsubscribe: (() => void) | null = null

  function apply(payload: Submission) {
    submissions.value = new Map(submissions.value).set(payload.id, payload)
  }

  function start() {
    stop()
    const id = toValue(contestId)
    if (!id) return

    if (mock) {
      unsubscribe = mockSubscribe(id, apply)
      connected.value = true
      return
    }

    source = new EventSource(api.apiUrl(`/api/contests/${id}/events`), {
      withCredentials: true,
    })

    source.addEventListener('open', () => {
      connected.value = true
    })
    source.addEventListener('error', () => {
      connected.value = false
    })
    source.addEventListener('submission', (event) => {
      try {
        apply(JSON.parse((event as MessageEvent).data) as Submission)
      } catch {
        // Ignore a malformed frame rather than tearing down the stream.
      }
    })
  }

  function stop() {
    source?.close()
    source = null
    unsubscribe?.()
    unsubscribe = null
    connected.value = false
  }

  onScopeDispose(stop)

  return { submissions, connected, start, stop }
}
