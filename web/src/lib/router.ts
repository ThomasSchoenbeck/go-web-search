/**
 * Minimal History-API router.
 *
 * Svelte ships no router, so this is a deliberate choice rather than a default:
 * clean URLs (`/runs/123`) with the Go handler serving index.html for unknown
 * non-API paths, so deep links resolve. Hash routing was the simpler
 * alternative and is not used.
 *
 * Views render plain <a href="/..."> links; startRouter intercepts same-origin
 * clicks and turns them into pushState navigations. That keeps links real links
 * — middle-click, open-in-new-tab and keyboard activation all still work, and
 * there is no custom Link component for views to remember to use.
 */

import { writable, type Readable } from 'svelte/store'

function currentPath(): string {
  return window.location.pathname
}

const store = writable<string>(typeof window === 'undefined' ? '/' : currentPath())

/** The active path, as a Svelte store. */
export const path: Readable<string> = { subscribe: store.subscribe }

export function navigate(to: string, replace = false): void {
  if (to === currentPath()) return
  if (replace) window.history.replaceState({}, '', to)
  else window.history.pushState({}, '', to)
  store.set(currentPath())
}

/** Should this click be handled by the router rather than the browser? */
export function isInternalNavigation(event: MouseEvent, anchor: HTMLAnchorElement): boolean {
  // Leave modified clicks and non-primary buttons to the browser so
  // open-in-new-tab keeps working.
  if (event.defaultPrevented || event.button !== 0) return false
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return false
  if (anchor.target && anchor.target !== '_self') return false
  if (anchor.hasAttribute('download')) return false
  if (anchor.origin !== window.location.origin) return false
  // Anything the Go server owns must do a real request.
  if (anchor.pathname.startsWith('/api/') || anchor.pathname.startsWith('/mcp')) return false
  return true
}

/** Install the popstate and click listeners. Returns a teardown function. */
export function startRouter(): () => void {
  const onPopState = (): void => store.set(currentPath())
  const onClick = (event: MouseEvent): void => {
    const anchor = (event.target as Element | null)?.closest('a')
    if (!anchor) return
    if (!isInternalNavigation(event, anchor)) return
    event.preventDefault()
    navigate(anchor.pathname + anchor.search)
  }

  window.addEventListener('popstate', onPopState)
  document.addEventListener('click', onClick)
  store.set(currentPath())

  return () => {
    window.removeEventListener('popstate', onPopState)
    document.removeEventListener('click', onClick)
  }
}

/**
 * Match a path against a pattern with `:name` segments, returning the captured
 * params or null. Exact segment count — `/runs/:id` does not match `/runs`.
 */
export function matchRoute(pattern: string, actual: string): Record<string, string> | null {
  const patternParts = pattern.split('/').filter(Boolean)
  const actualParts = actual.split('/').filter(Boolean)
  if (patternParts.length !== actualParts.length) return null

  const params: Record<string, string> = {}
  for (let i = 0; i < patternParts.length; i += 1) {
    const expected = patternParts[i]
    const got = actualParts[i]
    if (expected.startsWith(':')) {
      params[expected.slice(1)] = decodeURIComponent(got)
      continue
    }
    if (expected !== got) return null
  }
  return params
}
