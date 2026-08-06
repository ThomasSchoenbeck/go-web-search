import { describe, it, expect } from 'vitest'
import { routes, navEntries, resolveRoute, activeSection } from './routes'
import { matchRoute } from './router'

describe('resolveRoute', () => {
  it('resolves the list routes', () => {
    expect(resolveRoute('/')?.name).toBe('runs')
    expect(resolveRoute('/runs')?.name).toBe('runs')
  })

  it('prefers the more specific run routes over run detail', () => {
    expect(resolveRoute('/runs/abc')?.name).toBe('run-detail')
    expect(resolveRoute('/runs/abc/searches')?.name).toBe('run-searches')
    expect(resolveRoute('/runs/abc/causality')?.name).toBe('run-causality')
  })

  it('captures params', () => {
    expect(resolveRoute('/scrapes/xyz')?.params).toEqual({ id: 'xyz' })
    expect(resolveRoute('/searches/s1')?.params).toEqual({ id: 's1' })
  })

  it('returns null for an unknown path', () => {
    expect(resolveRoute('/nope/deep/path')).toBeNull()
  })
})

describe('activeSection', () => {
  it('keeps the runs entry active across every run-derived view', () => {
    for (const path of ['/', '/runs', '/runs/1', '/runs/1/searches', '/runs/1/causality', '/searches/s', '/scrapes/x']) {
      expect(activeSection(path)).toBe('runs')
    }
  })

  it('marks provenance on its own route', () => {
    expect(activeSection('/provenance')).toBe('provenance')
  })

  it('is null when nothing matches', () => {
    expect(activeSection('/nope')).toBeNull()
  })
})

// The point of routes.ts is that these two lists cannot drift apart.
describe('route and nav consistency', () => {
  it('every nav entry points at a path that resolves', () => {
    for (const entry of navEntries) {
      const resolved = resolveRoute(entry.href)
      expect(resolved, `nav entry ${entry.label} → ${entry.href}`).not.toBeNull()
      expect(resolved?.section).toBe(entry.section)
    }
  })

  it('every route section has a nav entry to reach it', () => {
    const sections = new Set(navEntries.map((entry) => entry.section))
    for (const route of routes) {
      expect(sections.has(route.section), `route ${route.pattern} has section ${route.section}`).toBe(true)
    }
  })

  it('lists more specific patterns before their prefixes', () => {
    // resolveRoute takes the first match, so ordering is load-bearing.
    const index = (pattern: string): number => routes.findIndex((r) => r.pattern === pattern)
    expect(index('/runs/:id/searches')).toBeLessThan(index('/runs/:id'))
    expect(index('/runs/:id/causality')).toBeLessThan(index('/runs/:id'))
    expect(index('/runs/:id')).toBeLessThan(index('/runs'))
  })

  it('has no pattern that matchRoute cannot parse', () => {
    for (const route of routes) {
      const sample = route.pattern.replace(/:[^/]+/g, 'sample')
      expect(matchRoute(route.pattern, sample), route.pattern).not.toBeNull()
    }
  })
})
