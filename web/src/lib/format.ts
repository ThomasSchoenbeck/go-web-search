/** Display helpers shared by the views. Pure functions, no DOM. */

/** Milliseconds as a compact human duration. */
export function formatDuration(ms: number | undefined | null): string {
  if (ms === undefined || ms === null || Number.isNaN(ms)) return '—'
  if (ms < 1000) return `${Math.round(ms)}ms`
  const seconds = ms / 1000
  if (seconds < 60) return `${seconds.toFixed(seconds < 10 ? 1 : 0)}s`
  const minutes = Math.floor(seconds / 60)
  return `${minutes}m ${Math.round(seconds % 60)}s`
}

/**
 * RFC3339 timestamps as `YYYY-MM-DD HH:MM:SS` in local time. Returns the input
 * unchanged when it is not parseable, so a stored oddity stays visible rather
 * than becoming "Invalid Date".
 */
export function formatTimestamp(value: string | undefined | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number): string => String(n).padStart(2, '0')
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  )
}

/** Middle-truncate long values (URLs, ids) so a table row stays readable. */
export function truncate(value: string, max = 80): string {
  if (value.length <= max) return value
  const head = Math.ceil((max - 1) / 2)
  const tail = Math.floor((max - 1) / 2)
  return `${value.slice(0, head)}…${value.slice(value.length - tail)}`
}

/** Character counts as a compact size. Content here is text, not bytes. */
export function formatChars(count: number | undefined | null): string {
  if (!count) return '0'
  if (count < 1000) return String(count)
  if (count < 1_000_000) return `${(count / 1000).toFixed(1)}k`
  return `${(count / 1_000_000).toFixed(1)}M`
}
