/**
 * Deterministic, readable identifiers.
 *
 * The first eight characters name the entity kind, so the admin UI's
 * `id.slice(0, 8)` labels stay legible (`task0001`, not `00000000`). These are
 * deliberately *not* valid UUIDs — a mock id should never be mistaken for a real
 * one, and nothing on the mock path parses them as UUIDs.
 */
export function makeId(kind: string, n: number): string {
  const tag = `${kind}${n.toString(36)}`.padEnd(8, '0').slice(0, 8)
  return `${tag}-0000-4000-8000-${n.toString(16).padStart(12, '0')}`
}

/** Allocates ids for one entity kind, in order. */
export function idSeries(kind: string) {
  let n = 0
  return () => makeId(kind, ++n)
}

/**
 * Per-kind counters live on the state so ids stay unique across mutations and
 * reset together with everything else.
 */
export function nextId(counters: Record<string, number>, kind: string): string {
  const n = (counters[kind] ?? 0) + 1
  counters[kind] = n
  return makeId(kind, n)
}
