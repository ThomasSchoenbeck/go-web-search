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

<section class="logs">
  <div class="head">
    <h1>Logs</h1>
    <PollControls task={() => logs.reload()} />
  </div>

  <!-- One filter bar: level, run, source and the actions read as a single control strip. -->
  <form class="filters" onsubmit={submitFilters}>
    <label>
      Level
      <select data-testid="logs-level" value={level} onchange={changeLevel}>
        <option value="">any</option>
        {#each levels as name (name)}
          <option value={name}>{name}</option>
        {/each}
      </select>
    </label>
    <span class="sep" aria-hidden="true"></span>
    <label>
      Run id
      <input type="search" data-testid="logs-run" placeholder="exact run id" bind:value={runDraft} />
    </label>
    <label>
      Source
      <input type="search" data-testid="logs-source" placeholder="e.g. harvester" bind:value={sourceDraft} />
    </label>
    <button class="primary" type="submit" data-testid="logs-submit">Filter</button>
    <button class="ghost" type="button" data-testid="logs-clear" onclick={clearFilters}>Clear</button>
    <span class="count">{rows.length} lines · newest first</span>
  </form>

  {#if $logs.loading && !$logs.data}
    <p data-testid="logs-loading">Loading logs…</p>
  {/if}
  {#if $logs.error}
    <p class="bad" data-testid="logs-error">Could not load logs: {$logs.error.message}</p>
  {/if}

  {#if $logs.data}
    {#if rows.length === 0}
      <p class="empty" data-testid="logs-empty">
        {filtered ? 'No log lines match this filter.' : 'Nothing has been logged yet.'}
      </p>
    {:else}
      <table data-testid="logs-table">
        <thead>
          <tr>
            <th class="c-when">When</th>
            <th class="c-level">Level</th>
            <th class="c-source">Source</th>
            <th class="c-run">Run</th>
            <th>Message</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as entry (entry.id)}
            <tr data-testid="log-row" class="level-{entry.level}">
              <td class="c-when">{formatTimestamp(entry.created_at)}</td>
              <td class="c-level" data-testid="log-level">{entry.level}</td>
              <td class="c-source">{entry.source ?? '—'}</td>
              <td class="c-run">
                {#if entry.run_id}
                  <a href="/runs/{entry.run_id}" data-testid="log-run-link">{truncate(entry.run_id, 12)}</a>
                {:else}
                  —
                {/if}
              </td>
              <td class="msg" data-testid="log-message">{entry.message}</td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p class="pager">
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
  .head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 24px;
    margin-bottom: 14px;
  }

  .head :global(h1) {
    margin: 0;
  }

  .filters {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 0;
    padding: 8px 12px;
    background: var(--panel);
    border: 1px solid var(--line);
  }

  .filters .sep {
    width: 1px;
    height: 22px;
    background: var(--line);
  }

  .filters input {
    width: 190px;
  }

  .filters .primary {
    background: var(--cyan);
    border-color: var(--cyan);
    color: var(--bg);
  }

  .filters .ghost {
    background: transparent;
    color: var(--muted);
  }

  .count {
    margin-left: auto;
    font-size: 11px;
    color: var(--dim);
  }

  table {
    margin-top: 0;
    table-layout: fixed;
  }

  /* Time is fixed-width and abbreviated; the message gets everything left. */
  .c-when {
    width: 96px;
  }
  .c-level {
    width: 74px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }
  .c-source {
    width: 100px;
  }
  .c-run {
    width: 110px;
  }

  tbody .c-when,
  tbody .c-source {
    color: var(--muted);
  }

  .msg {
    color: oklch(0.86 0.008 265);
    word-break: break-word;
  }

  thead th:first-child,
  tbody td:first-child {
    padding-left: 14px;
  }

  /* A 2px gutter on the row carries the level, so the eye scans one edge
     instead of reading a column. Level text is coloured to match. */
  tbody td:first-child {
    border-left: 2px solid var(--line-strong);
  }

  .level-error td:first-child {
    border-left-color: var(--red);
  }
  .level-error td {
    background: oklch(0.72 0.16 25 / 0.07);
  }
  .level-error .c-level,
  .level-error .msg {
    color: oklch(0.78 0.15 25);
  }

  .level-warn td:first-child {
    border-left-color: var(--amber);
  }
  .level-warn .c-level {
    color: oklch(0.84 0.14 90);
  }

  .level-notice td:first-child {
    border-left-color: var(--cyan);
  }
  .level-notice .c-level {
    color: oklch(0.84 0.14 190);
  }

  .level-info .c-level {
    color: var(--dim);
  }

  .pager {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 0 0;
  }

  .pager span {
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--dim);
  }

  .empty {
    color: var(--dim);
  }

  .bad {
    color: var(--red);
  }
</style>
