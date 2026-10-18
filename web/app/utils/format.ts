import type { Contest, ContestPhase } from '~/types/api'

const dateTimeFormat = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return dateTimeFormat.format(date)
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return dateTimeFormat.format(date).slice(0, 10)
}

/** "3 天后" / "2 小时前" style relative time. */
export function formatRelative(value: string | null | undefined): string {
  if (!value) return '—'
  const then = new Date(value).getTime()
  if (Number.isNaN(then)) return '—'
  const diffMs = then - Date.now()
  const abs = Math.abs(diffMs)
  const units: [Intl.RelativeTimeFormatUnit, number][] = [
    ['second', 1000],
    ['minute', 60_000],
    ['hour', 3_600_000],
    ['day', 86_400_000],
    ['month', 2_592_000_000],
    ['year', 31_536_000_000],
  ]
  const formatter = new Intl.RelativeTimeFormat('zh-CN', { numeric: 'auto' })
  let chosen: [Intl.RelativeTimeFormatUnit, number] = units[0]!
  for (const unit of units) {
    if (abs >= unit[1]) chosen = unit
  }
  return formatter.format(Math.round(diffMs / chosen[1]), chosen[0])
}

export function formatBytes(bytes: number | null | undefined): string {
  if (bytes === null || bytes === undefined) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KiB', 'MiB', 'GiB']
  let value = bytes / 1024
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[index]}`
}

/** Milliseconds → `1000 ms` or `2.5 s`. */
export function formatDuration(ms: number | null | undefined): string {
  if (ms === null || ms === undefined) return '—'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(ms % 1000 === 0 ? 0 : 2)} s`
}

/** ACM penalty is counted in whole minutes. */
export function formatPenalty(minutes: number | null | undefined): string {
  if (minutes === null || minutes === undefined) return '—'
  return `${minutes} min`
}

/** Mirrors `domain.Phase` in the Go backend. */
export function contestPhase(
  contest: Pick<Contest, 'starts_at' | 'ends_at' | 'published_at'>,
): ContestPhase {
  if (contest.published_at) return 'published'
  const now = Date.now()
  const start = new Date(contest.starts_at).getTime()
  const end = new Date(contest.ends_at).getTime()
  if (now < start) return 'before'
  if (now < end) return 'running'
  return 'ended_unpublished'
}

export function registrationOpen(
  contest: Pick<Contest, 'register_starts_at' | 'ends_at'>,
): boolean {
  const now = Date.now()
  return (
    now >= new Date(contest.register_starts_at).getTime() &&
    now <= new Date(contest.ends_at).getTime()
  )
}

export function percent(part: number, whole: number): number {
  if (!whole) return 0
  return Math.round((part / whole) * 100)
}

/** `Date` → value accepted by `<input type="datetime-local">`. */
export function toLocalInput(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` +
    `T${pad(date.getHours())}:${pad(date.getMinutes())}`
  )
}

/** `<input type="datetime-local">` value → RFC3339, which Go's `time.Time` requires. */
export function localInputToIso(value: string): string | null {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return null
  return date.toISOString()
}
