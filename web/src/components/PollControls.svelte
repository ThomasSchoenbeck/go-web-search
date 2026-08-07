<script lang="ts">
  /**
   * The live-refresh controls for the two polling views (jobs T015, logs T019).
   *
   * The cadence and whether polling starts at all are session defaults from
   * `/api/ui-config` — config.toml owns them, nothing here is hardcoded. The
   * dropdown and the toggle override both for the moment only; neither is
   * written back. The poller stops when this component unmounts, so leaving a
   * view leaves no timer behind.
   */
  import { onMount } from 'svelte'
  import { createPoller, type Poller } from '../lib/poll'
  import { loadUIConfig } from '../lib/uiconfig'
  import { formatDuration } from '../lib/format'

  interface Props {
    /** Run on each tick. Usually the view's `reload`. */
    task: () => unknown | Promise<unknown>
  }
  let { task }: Props = $props()

  const intervals = [1000, 2000, 5000, 10_000, 30_000]

  let poller: Poller | null = null
  let ready = $state(false)
  let running = $state(false)
  let intervalMs = $state(0)
  let error = $state('')

  onMount(() => {
    let unmounted = false
    loadUIConfig()
      .then((config) => {
        if (unmounted) return
        poller = createPoller(task, {
          intervalMs: config.pollIntervalMs,
          enabled: config.pollEnabled,
          onError: (cause) => (error = cause instanceof Error ? cause.message : String(cause)),
        })
        intervalMs = poller.intervalMs
        running = poller.running
        ready = true
      })
      .catch((cause: unknown) => {
        // Without the defaults there is nothing to seed a poller from; the view
        // still works, it just does not refresh itself.
        error = cause instanceof Error ? cause.message : String(cause)
      })
    return () => {
      unmounted = true
      poller?.stop()
    }
  })

  function toggle(): void {
    poller?.toggle()
    running = poller?.running ?? false
  }

  function changeInterval(event: Event): void {
    intervalMs = Number((event.currentTarget as HTMLSelectElement).value)
    poller?.setIntervalMs(intervalMs)
  }
</script>

<div class="poll" data-testid="poll-controls">
  <button type="button" data-testid="poll-toggle" disabled={!ready} onclick={toggle}>
    {running ? 'Stop polling' : 'Start polling'}
  </button>

  <label>
    Every
    <select data-testid="poll-interval" value={intervalMs} disabled={!ready} onchange={changeInterval}>
      {#each intervals as ms (ms)}
        <option value={ms}>{formatDuration(ms)}</option>
      {/each}
    </select>
  </label>

  <span data-testid="poll-status">
    {#if !ready}
      polling unavailable
    {:else if running}
      refreshing every {formatDuration(intervalMs)}
    {:else}
      paused
    {/if}
  </span>

  {#if error}
    <span data-testid="poll-error">Polling problem: {error}</span>
  {/if}
</div>

<style>
  .poll {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
</style>
