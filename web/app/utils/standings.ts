import type { StandingProblem, StandingRow } from '~/types/api'

/** Per-problem score of a standing, keyed by contest problem letter. */
export function problemScoresByLetter(standing: StandingRow): Map<string, StandingProblem> {
  return new Map(standing.problems.map((p) => [p.letter, p]))
}

/** Highest score obtainable, used for the score progress bar. */
export function maxScoreOf(rows: StandingRow[]): number {
  let max = 0
  for (const row of rows) {
    const value = Number.parseFloat(row.total_score)
    if (Number.isFinite(value) && value > max) max = value
  }
  return max
}
