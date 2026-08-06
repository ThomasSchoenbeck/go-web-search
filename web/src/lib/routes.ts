/**
 * The one place routes are defined.
 *
 * Both the shell's view selection and the navigation render from this list, so a
 * route cannot exist without a way to reach it, and a nav entry cannot point at
 * a pattern nothing matches. Adding a view means adding one entry here.
 *
 * Order matters: resolveRoute takes the first match, so more specific patterns
 * come first (`/runs/:id/searches` before `/runs/:id`).
 */

import { matchRoute } from './router'

/** Every view the shell can render. `name` is what App switches on. */
export type ViewName =
  | 'runs'
  | 'run-detail'
  | 'run-searches'
  | 'run-causality'
  | 'serp'
  | 'scrape'
  | 'provenance'
  | 'facts'
  | 'fact-detail'
  | 'explore'

export interface RouteDef {
  pattern: string
  name: ViewName
  /** Which nav entry is highlighted while this route is active. */
  section: string
}

export const routes: RouteDef[] = [
  { pattern: '/runs/:id/searches', name: 'run-searches', section: 'runs' },
  { pattern: '/runs/:id/causality', name: 'run-causality', section: 'runs' },
  { pattern: '/runs/:id', name: 'run-detail', section: 'runs' },
  { pattern: '/runs', name: 'runs', section: 'runs' },
  { pattern: '/searches/:id', name: 'serp', section: 'runs' },
  { pattern: '/scrapes/:id', name: 'scrape', section: 'runs' },
  { pattern: '/provenance', name: 'provenance', section: 'provenance' },
  { pattern: '/facts/:id', name: 'fact-detail', section: 'facts' },
  { pattern: '/facts', name: 'facts', section: 'facts' },
  { pattern: '/explore', name: 'explore', section: 'explore' },
  { pattern: '/', name: 'runs', section: 'runs' },
]

/** Top-level destinations, in the order they appear in the navigation. */
export interface NavEntry {
  section: string
  label: string
  href: string
}

export const navEntries: NavEntry[] = [
  { section: 'runs', label: 'Runs', href: '/runs' },
  { section: 'provenance', label: 'Provenance', href: '/provenance' },
  { section: 'facts', label: 'Memory', href: '/facts' },
  { section: 'explore', label: 'Explorer', href: '/explore' },
]

export interface ResolvedRoute {
  name: ViewName
  section: string
  params: Record<string, string>
}

export function resolveRoute(path: string): ResolvedRoute | null {
  for (const route of routes) {
    const params = matchRoute(route.pattern, path)
    if (params) return { name: route.name, section: route.section, params }
  }
  return null
}

/** Which nav entry to highlight for a path. Null when nothing matches. */
export function activeSection(path: string): string | null {
  return resolveRoute(path)?.section ?? null
}
