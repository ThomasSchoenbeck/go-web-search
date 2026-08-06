import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPoller } from './poll'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('createPoller', () => {
  it('starts stopped when config disables polling', () => {
    const task = vi.fn()
    const poller = createPoller(task, { intervalMs: 1000, enabled: false })

    expect(poller.running).toBe(false)
    vi.advanceTimersByTime(5000)
    expect(task).not.toHaveBeenCalled()
  })

  it('starts running when config enables polling', async () => {
    const task = vi.fn()
    const poller = createPoller(task, { intervalMs: 1000, enabled: true })

    expect(poller.running).toBe(true)
    await vi.advanceTimersByTimeAsync(1000)
    expect(task).toHaveBeenCalledTimes(1)
  })

  it('re-runs on the interval and stops cleanly', async () => {
    const task = vi.fn()
    const poller = createPoller(task, { intervalMs: 1000, enabled: false })

    poller.start()
    await vi.advanceTimersByTimeAsync(3000)
    expect(task).toHaveBeenCalledTimes(3)

    poller.stop()
    expect(poller.running).toBe(false)
    await vi.advanceTimersByTimeAsync(5000)
    expect(task).toHaveBeenCalledTimes(3)
  })

  it('honours a runtime interval override immediately', async () => {
    const task = vi.fn()
    const poller = createPoller(task, { intervalMs: 10_000, enabled: true })

    poller.setIntervalMs(500)
    expect(poller.intervalMs).toBe(500)
    expect(poller.running).toBe(true)

    await vi.advanceTimersByTimeAsync(1500)
    expect(task).toHaveBeenCalledTimes(3)
  })

  it('remembers a new interval while stopped', async () => {
    const task = vi.fn()
    const poller = createPoller(task, { intervalMs: 1000, enabled: false })

    poller.setIntervalMs(250)
    expect(poller.running).toBe(false)

    poller.start()
    await vi.advanceTimersByTimeAsync(500)
    expect(task).toHaveBeenCalledTimes(2)
  })

  it('toggles on and off', async () => {
    const task = vi.fn()
    const poller = createPoller(task, { intervalMs: 100, enabled: false })

    poller.toggle()
    expect(poller.running).toBe(true)
    poller.toggle()
    expect(poller.running).toBe(false)

    await vi.advanceTimersByTimeAsync(1000)
    expect(task).not.toHaveBeenCalled()
  })

  it('reports errors instead of breaking the loop', async () => {
    const onError = vi.fn()
    const task = vi.fn().mockRejectedValue(new Error('endpoint down'))
    createPoller(task, { intervalMs: 100, enabled: true, onError })

    await vi.advanceTimersByTimeAsync(300)
    expect(onError).toHaveBeenCalled()
    expect(task.mock.calls.length).toBeGreaterThan(1)
  })

  it('does not overlap runs when the task is slower than the interval', async () => {
    let running = 0
    let maxConcurrent = 0
    const task = vi.fn(async () => {
      running += 1
      maxConcurrent = Math.max(maxConcurrent, running)
      await new Promise((resolve) => setTimeout(resolve, 500))
      running -= 1
    })
    createPoller(task, { intervalMs: 100, enabled: true })

    await vi.advanceTimersByTimeAsync(2000)
    expect(maxConcurrent).toBe(1)
  })
})
