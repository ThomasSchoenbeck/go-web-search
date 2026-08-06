<script lang="ts">
  // Smoke usage of the shared read layer (T005): load the UI settings, fetch
  // /api/stats through the resource wrapper, and drive a poller with the
  // interval dropdown and on/off toggle the real views will reuse. Replaced by
  // the actual views in later tasks.
  import { onMount, onDestroy } from 'svelte'
  import { statsResource } from './lib/api'
  import { loadUIConfig, type UIConfig } from './lib/uiconfig'
  import { createPoller, type Poller } from './lib/poll'

  const stats = statsResource()

  let settings: UIConfig | null = $state(null)
  let settingsError: string | null = $state(null)
  let poller: Poller | null = $state(null)
  let polling = $state(false)
  let intervalMs = $state(0)

  const intervalChoices = [1000, 2000, 5000, 10000, 30000]

  onMount(async () => {
    await stats.reload()
    try {
      const config = await loadUIConfig()
      settings = config
      intervalMs = config.pollIntervalMs
      poller = createPoller(() => stats.reload(), {
        intervalMs: config.pollIntervalMs,
        enabled: config.pollEnabled,
      })
      polling = poller.running
    } catch (error) {
      settingsError = error instanceof Error ? error.message : String(error)
    }
  })

  onDestroy(() => poller?.stop())

  function togglePolling(): void {
    poller?.toggle()
    polling = poller?.running ?? false
  }

  function changeInterval(event: Event): void {
    const ms = Number((event.currentTarget as HTMLSelectElement).value)
    intervalMs = ms
    poller?.setIntervalMs(ms)
  }
</script>

<main>
  <h1>Observability UI</h1>
  <p>Read-only inspection of runs, scrapes, memory, jobs, caches and logs.</p>

  <section>
    <h2>Settings</h2>
    {#if settingsError}
      <p data-testid="settings-error">Could not load settings: {settingsError}</p>
    {:else if settings}
      <p data-testid="settings">
        poll interval {settings.pollIntervalMs}ms · polling
        {settings.pollEnabled ? 'on' : 'off'} by default · projection cap
        {settings.projectionSampleCap}
      </p>
    {:else}
      <p>Loading settings…</p>
    {/if}
  </section>

  <section>
    <h2>Live refresh</h2>
    <button type="button" data-testid="toggle-polling" onclick={togglePolling} disabled={!poller}>
      {polling ? 'Stop polling' : 'Start polling'}
    </button>
    <label>
      Interval
      <select data-testid="interval" value={intervalMs} onchange={changeInterval} disabled={!poller}>
        {#each intervalChoices as choice (choice)}
          <option value={choice}>{choice}ms</option>
        {/each}
      </select>
    </label>
    <button type="button" data-testid="reload" onclick={() => stats.reload()}>Reload now</button>
  </section>

  <section>
    <h2>/api/stats</h2>
    {#if $stats.loading && !$stats.data}
      <p data-testid="stats-loading">Loading…</p>
    {/if}
    {#if $stats.error}
      <p data-testid="stats-error">{$stats.error.message}</p>
    {/if}
    {#if $stats.data}
      <pre data-testid="stats">{JSON.stringify($stats.data, null, 2)}</pre>
    {/if}
  </section>
</main>
