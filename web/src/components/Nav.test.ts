import { describe, it, expect, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/svelte'
import Nav from './Nav.svelte'
import { navigate } from '../lib/router'
import { navEntries } from '../lib/routes'

beforeEach(() => window.history.replaceState({}, '', '/'))

afterEach(() => {
  cleanup()
  window.history.replaceState({}, '', '/')
})

describe('Nav', () => {
  it('renders an entry per registered destination', () => {
    render(Nav)

    for (const entry of navEntries) {
      const link = screen.getByTestId(`nav-${entry.section}`)
      expect(link.getAttribute('href')).toBe(entry.href)
      expect(link.textContent?.trim()).toBe(entry.label)
    }
  })

  it('marks the active entry for assistive technology, not just visually', () => {
    navigate('/runs')
    render(Nav)

    expect(screen.getByTestId('nav-runs').getAttribute('aria-current')).toBe('page')
    expect(screen.getByTestId('nav-provenance').getAttribute('aria-current')).toBeNull()
  })

  it('keeps the runs entry active on a nested run route', () => {
    navigate('/runs/abc/causality')
    render(Nav)

    expect(screen.getByTestId('nav-runs').getAttribute('aria-current')).toBe('page')
  })

  it('follows navigation without a remount', async () => {
    navigate('/runs')
    render(Nav)
    expect(screen.getByTestId('nav-runs').getAttribute('aria-current')).toBe('page')

    navigate('/provenance')
    await Promise.resolve()

    expect(screen.getByTestId('nav-provenance').getAttribute('aria-current')).toBe('page')
    expect(screen.getByTestId('nav-runs').getAttribute('aria-current')).toBeNull()
  })

  it('marks nothing when the route is unknown', () => {
    navigate('/nope')
    render(Nav)

    for (const entry of navEntries) {
      expect(screen.getByTestId(`nav-${entry.section}`).getAttribute('aria-current')).toBeNull()
    }
  })

  it('exposes a labelled navigation landmark', () => {
    const { container } = render(Nav)
    expect(container.querySelector('nav')?.getAttribute('aria-label')).toBe('Views')
  })
})
