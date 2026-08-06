/**
 * Interval re-fetching for the live views (jobs T015, logs T019).
 *
 * Seeded from `/api/ui-config` — which may say polling starts off — and
 * overridable at runtime by the UI's interval dropdown and on/off toggle. An
 * override lasts for the moment only; nothing is written back to config.
 *
 * Polling, not SSE, is the v1 live-update mechanism.
 */

export interface PollerOptions {
  /** Milliseconds between runs. Seed from UIConfig.pollIntervalMs. */
  intervalMs: number
  /** Start polling immediately. Seed from UIConfig.pollEnabled. */
  enabled: boolean
  /** Called when a run throws, so views can surface it. */
  onError?: (error: unknown) => void
}

export interface Poller {
  readonly running: boolean
  readonly intervalMs: number
  start(): void
  stop(): void
  /** Change the cadence; a running poller re-arms on the new interval. */
  setIntervalMs(ms: number): void
  /** Convenience for a toggle button. */
  toggle(): void
}

/**
 * Runs `task` every `intervalMs` while started.
 *
 * The next run is scheduled only after the previous one settles, so a slow
 * endpoint can never pile up overlapping requests the way setInterval would.
 * The first run happens one interval in — the caller does the initial load, so
 * enabling polling does not duplicate it.
 */
export function createPoller(task: () => unknown | Promise<unknown>, options: PollerOptions): Poller {
  let intervalMs = options.intervalMs
  let running = false
  let timer: ReturnType<typeof setTimeout> | null = null
  // Guards against a run that was in flight when stop() was called resuming the
  // loop after the fact.
  let generation = 0

  function schedule(forGeneration: number): void {
    timer = setTimeout(() => {
      void run(forGeneration)
    }, intervalMs)
  }

  async function run(forGeneration: number): Promise<void> {
    try {
      await task()
    } catch (error) {
      options.onError?.(error)
    }
    if (running && forGeneration === generation) schedule(forGeneration)
  }

  const poller: Poller = {
    get running() {
      return running
    },
    get intervalMs() {
      return intervalMs
    },
    start() {
      if (running) return
      running = true
      generation += 1
      schedule(generation)
    },
    stop() {
      running = false
      generation += 1
      if (timer !== null) {
        clearTimeout(timer)
        timer = null
      }
    },
    setIntervalMs(ms: number) {
      intervalMs = ms
      if (running) {
        // Re-arm so the new cadence takes effect now rather than after the
        // pending tick, which is what a user expects from the dropdown.
        poller.stop()
        poller.start()
      }
    },
    toggle() {
      if (running) poller.stop()
      else poller.start()
    },
  }

  if (options.enabled) poller.start()
  return poller
}
