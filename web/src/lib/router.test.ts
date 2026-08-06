import { describe, it, expect, vi, afterEach } from 'vitest'
import { matchRoute, isInternalNavigation, navigate, startRouter, path } from './router'
import { get } from 'svelte/store'

afterEach(() => {
  window.history.replaceState({}, '', '/')
})

describe('matchRoute', () => {
  it('matches a literal path', () => {
    expect(matchRoute('/runs', '/runs')).toEqual({})
  })

  it('captures named params', () => {
    expect(matchRoute('/runs/:id', '/runs/abc')).toEqual({ id: 'abc' })
    expect(matchRoute('/runs/:id/searches', '/runs/abc/searches')).toEqual({ id: 'abc' })
  })

  it('requires an exact segment count', () => {
    expect(matchRoute('/runs/:id', '/runs')).toBeNull()
    expect(matchRoute('/runs/:id', '/runs/abc/searches')).toBeNull()
  })

  it('does not match a different literal segment', () => {
    expect(matchRoute('/runs/:id/searches', '/runs/abc/scrapes')).toBeNull()
  })

  it('decodes percent-encoded params', () => {
    expect(matchRoute('/scrapes/:id', '/scrapes/a%2Fb')).toEqual({ id: 'a/b' })
  })

  it('treats trailing slashes as equivalent', () => {
    expect(matchRoute('/runs', '/runs/')).toEqual({})
  })
})

function anchorFor(href: string, attrs: Record<string, string> = {}): HTMLAnchorElement {
  const anchor = document.createElement('a')
  anchor.href = href
  for (const [key, value] of Object.entries(attrs)) anchor.setAttribute(key, value)
  return anchor
}

describe('isInternalNavigation', () => {
  const plainClick = (): MouseEvent => new MouseEvent('click', { button: 0 })

  it('accepts a same-origin app link', () => {
    expect(isInternalNavigation(plainClick(), anchorFor('/runs/1'))).toBe(true)
  })

  it('leaves modified clicks to the browser', () => {
    for (const modifier of ['metaKey', 'ctrlKey', 'shiftKey', 'altKey'] as const) {
      const event = new MouseEvent('click', { button: 0, [modifier]: true })
      expect(isInternalNavigation(event, anchorFor('/runs/1'))).toBe(false)
    }
  })

  it('leaves non-primary buttons to the browser', () => {
    expect(isInternalNavigation(new MouseEvent('click', { button: 1 }), anchorFor('/runs/1'))).toBe(false)
  })

  it('ignores cross-origin links', () => {
    expect(isInternalNavigation(plainClick(), anchorFor('https://example.com/page'))).toBe(false)
  })

  it('ignores targeted and download links', () => {
    expect(isInternalNavigation(plainClick(), anchorFor('/runs/1', { target: '_blank' }))).toBe(false)
    expect(isInternalNavigation(plainClick(), anchorFor('/runs/1', { download: '' }))).toBe(false)
  })

  it('lets backend paths do a real request', () => {
    expect(isInternalNavigation(plainClick(), anchorFor('/api/runs'))).toBe(false)
    expect(isInternalNavigation(plainClick(), anchorFor('/mcp'))).toBe(false)
  })
})

describe('navigate', () => {
  it('pushes a history entry and updates the path store', () => {
    navigate('/runs/42')
    expect(window.location.pathname).toBe('/runs/42')
    expect(get(path)).toBe('/runs/42')
  })

  it('is a no-op when already on the path', () => {
    navigate('/runs/42')
    const push = vi.spyOn(window.history, 'pushState')
    navigate('/runs/42')
    expect(push).not.toHaveBeenCalled()
  })
})

describe('startRouter', () => {
  it('intercepts internal anchor clicks and stops after teardown', () => {
    const teardown = startRouter()
    const anchor = anchorFor('/runs/7')
    document.body.appendChild(anchor)

    anchor.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }))
    expect(get(path)).toBe('/runs/7')

    teardown()
    const after = anchorFor('/runs/8')
    document.body.appendChild(after)
    after.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }))
    expect(get(path)).toBe('/runs/7')

    anchor.remove()
    after.remove()
  })
})
