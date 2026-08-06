import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import PollControls from './PollControls.svelte'
import { stubApi } from '../lib/apiStub'
import { resetUIConfig } from '../lib/uiconfig'

beforeEach(() => {
  resetUIConfig()
  vi.useFakeTimers({ shouldAdvanceTime: true })
})

afterEach(() => {
  vi.useRealTimers()
  cleanup()
  vi.unstubAllGlobals()
})

function stubConfig(overrides: Record<string, unknown> = {}): void {
  stubApi({
    '/api/ui-config': {
      json: { poll_interval_ms: 1000, poll_enabled: false, projection_sample_cap: 10, ...overrides },
    },
  })
}

describe('PollControls', () => {
  it('seeds the cadence and the on/off state from ui-config', async () => {
    stubConfig({ poll_interval_ms: 2000, poll_enabled: true })
    const task = vi.fn()
    render(PollControls, { props: { task } })

    await waitFor(() => expect(screen.getByTestId('poll-status').textContent).toContain('refreshing'))
    expect((screen.getByTestId('poll-interval') as HTMLSelectElement).value).toBe('2000')
    expect(screen.getByTestId('poll-toggle').textContent).toContain('Stop')
  })

  it('starts paused when config says polling is off', async () => {
    stubConfig({ poll_enabled: false })
    const task = vi.fn()
    render(PollControls, { props: { task } })

    await waitFor(() => expect(screen.getByTestId('poll-toggle')).toHaveProperty('disabled', false))
    expect(screen.getByTestId('poll-status').textContent).toContain('paused')

    await vi.advanceTimersByTimeAsync(5000)
    expect(task).not.toHaveBeenCalled()
  })

  it('the toggle button starts and stops the poll', async () => {
    stubConfig({ poll_interval_ms: 1000, poll_enabled: false })
    const task = vi.fn()
    render(PollControls, { props: { task } })
    await waitFor(() => expect(screen.getByTestId('poll-toggle')).toHaveProperty('disabled', false))

    await fireEvent.click(screen.getByTestId('poll-toggle'))
    await vi.advanceTimersByTimeAsync(1000)
    expect(task).toHaveBeenCalledTimes(1)

    await fireEvent.click(screen.getByTestId('poll-toggle'))
    await vi.advanceTimersByTimeAsync(5000)
    expect(task).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId('poll-status').textContent).toContain('paused')
  })

  it('the dropdown changes the cadence of a running poll', async () => {
    stubConfig({ poll_interval_ms: 10_000, poll_enabled: true })
    const task = vi.fn()
    render(PollControls, { props: { task } })
    await waitFor(() => expect(screen.getByTestId('poll-status').textContent).toContain('refreshing'))

    await fireEvent.change(screen.getByTestId('poll-interval'), { target: { value: '1000' } })
    await vi.advanceTimersByTimeAsync(1000)

    expect(task).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId('poll-status').textContent).toContain('1.0s')
  })

  it('stops the poll when the view unmounts, leaving no timer behind', async () => {
    stubConfig({ poll_interval_ms: 1000, poll_enabled: true })
    const task = vi.fn()
    const { unmount } = render(PollControls, { props: { task } })
    await waitFor(() => expect(screen.getByTestId('poll-status').textContent).toContain('refreshing'))

    unmount()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(task).not.toHaveBeenCalled()
  })

  it('leaves the controls unavailable when the defaults cannot be loaded', async () => {
    stubApi({ '/api/ui-config': { status: 500, json: { error: 'boom' } } })
    render(PollControls, { props: { task: vi.fn() } })

    await waitFor(() => expect(screen.getByTestId('poll-error')).toBeTruthy())
    expect(screen.getByTestId('poll-toggle')).toHaveProperty('disabled', true)
    expect(screen.getByTestId('poll-status').textContent).toContain('unavailable')
  })
})
