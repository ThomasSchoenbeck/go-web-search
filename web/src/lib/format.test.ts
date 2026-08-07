import { describe, it, expect } from 'vitest'
import { formatDuration, formatTimestamp, truncate, formatChars } from './format'

describe('formatDuration', () => {
  it('renders sub-second values in milliseconds', () => {
    expect(formatDuration(0)).toBe('0ms')
    expect(formatDuration(42)).toBe('42ms')
    expect(formatDuration(999)).toBe('999ms')
  })

  it('renders seconds and minutes', () => {
    expect(formatDuration(1500)).toBe('1.5s')
    expect(formatDuration(45_000)).toBe('45s')
    expect(formatDuration(90_000)).toBe('1m 30s')
  })

  it('renders missing values as a dash', () => {
    expect(formatDuration(null)).toBe('—')
    expect(formatDuration(undefined)).toBe('—')
    expect(formatDuration(Number.NaN)).toBe('—')
  })
})

describe('formatTimestamp', () => {
  it('formats an RFC3339 timestamp', () => {
    // Built from a local-time Date so the assertion holds in any timezone.
    const when = new Date(2026, 7, 5, 14, 3, 9)
    expect(formatTimestamp(when.toISOString())).toBe('2026-08-05 14:03:09')
  })

  it('returns an unparseable value unchanged rather than "Invalid Date"', () => {
    expect(formatTimestamp('not a date')).toBe('not a date')
  })

  it('renders missing values as a dash', () => {
    expect(formatTimestamp('')).toBe('—')
    expect(formatTimestamp(null)).toBe('—')
  })
})

describe('truncate', () => {
  it('leaves short values alone', () => {
    expect(truncate('https://example.com', 80)).toBe('https://example.com')
  })

  it('middle-truncates long values to the requested length', () => {
    const long = 'https://example.com/' + 'x'.repeat(200)
    const result = truncate(long, 40)
    expect(result).toHaveLength(40)
    expect(result).toContain('…')
    expect(result.startsWith('https://')).toBe(true)
    expect(result.endsWith('x')).toBe(true)
  })
})

describe('formatChars', () => {
  it('formats counts by magnitude', () => {
    expect(formatChars(0)).toBe('0')
    expect(formatChars(999)).toBe('999')
    expect(formatChars(1500)).toBe('1.5k')
    expect(formatChars(2_500_000)).toBe('2.5M')
  })

  it('treats missing counts as zero', () => {
    expect(formatChars(null)).toBe('0')
    expect(formatChars(undefined)).toBe('0')
  })
})
