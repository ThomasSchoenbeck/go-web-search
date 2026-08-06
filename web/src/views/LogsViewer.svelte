<script lang="ts">
  /**
   * The log database, newest line first — a tail with filters.
   *
   * The logs live in their own database file, separate from everything else the
   * UI shows, and the server only gained a read path onto it in T018.
   */
  import { logsResource } from '../lib/api'
  import { formatTimestamp, truncate } from '../lib/format'
  import PollControls from '../components/PollControls.svelte'

  const levels = ['error', 'warn', 'notice', 'info']
  const limit = 50

  let level = $state('')
  let runId = $state('')
  let source = $state('')
  let runDraft = $state('')
  let sourceDraft = $state('')
  let offset = $state(0)

  let logs = $derived(logsResource({ run_id: runId, level, source, limit, offset }))

  $effect(() => {
    void logs.reload()
  })

  function submitFilters(event: SubmitEvent): void {
    event.preventDefault()
    runId = runDraft.trim()
    source = sourceDraft.trim()
    offset = 0
  }

  function clearFilters(): void {
    runDraft = ''
    sourceDraft = ''
    runId = ''
    source = ''
    level = ''
    offset = 0
  }

  function changeLevel(event: Event): void {
    level = (event.currentTarget as HTMLSelectElement).value
    offset = 0
  }

  let rows = $derived($logs.data ?? [])
  let hasNextPage = $derived(rows.length === limit)
  let page = $derived(Math.floor(offset / limit) + 1)
  let filtered = $derived(Boolean(level || runId || source))
</script>

<section>
  <h1>Logs</h1>

  <PollControls task={() => logs.reload()} />

  <form onsubmit={submitFilters}>
    <label>
      Run id
      <input type="search" data-testid="logs-run" placeholder="exact run id" bind:value={runDraft} />
    </label>
    <label>
      Source
      <input type="search" data-testid="logs-source" placeholder="e.g. harvester" bind:value={sourceDraft} />
    </label>
    <button type="submit" data-testid="logs-submit">Filter</button>
    <button type="button" data-testid="logs-clear" onclick={clearFilters}>Clear</button>
  </form>

  <label>
    Level
    <select data-testid="logs-level" value={level} onchange={changeLevel}>
      <option value="">any</option>
      {#each levels as name (name)}
        <option value={name}>{name}</option>
      {/each}
    </select>
  </label>

  {#if $logs.loading && !$logs.data}
    <p data-testid="logs-loading">Loading logs…</p>
  {/if}
  {#if $logs.error}
    <p data-testid="logs-error">Could not load logs: {$logs.error.message}</p>
  {/if}

  {#if $logs.data}
    {#if rows.length === 0}
      <p data-testid="logs-empty">
        {filtered ? 'No log lines match this filter.' : 'Nothing has been logged yet.'}
      </p>
    {:else}
      <table data-testid="logs-table">
        <thead>
          <tr>
            <th>When</th>
            <th>Level</th>
            <th>Source</th>
            <th>Run</th>
            <th>Message</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as entry (entry.id)}
            <tr data-testid="log-row" class="level-{entry.level}">
              <td>{formatTimestamp(entry.created_at)}</td>
              <td data-testid="log-level">{entry.level}</td>
              <td>{entry.source ?? '—'}</td>
              <td>
                {#if entry.run_id}
                  <a href="/runs/{entry.run_id}" data-testid="log-run-link">{truncate(entry.run_id, 12)}</a>
                {:else}
                  —
                {/if}
              </td>
              <td data-testid="log-message">{entry.message}</td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p>
        <button
          type="button"
          data-testid="logs-prev"
          disabled={offset === 0}
          onclick={() => (offset = Math.max(0, offset - limit))}
        >
          Newer
        </button>
        <span data-testid="logs-page">Page {page}</span>
        <button
          type="button"
          data-testid="logs-next"
          disabled={!hasNextPage}
          onclick={() => (offset = offset + limit)}
        >
          Older
        </button>
      </p>
    {/if}
  {/if}
</section>

<style>
  /* Level is what the eye scans for, so it is the only thing coloured. */
  .level-error td {
    color: #b00020;
    font-weight: bold;
  }
  .level-warn td {
    color: #8a6100;
  }
  .level-notice td {
    color: #00548a;
  }
</style>
