<script lang="ts">
  /**
   * The background queue, as it stands right now.
   *
   * Read-only: this shows what the job system is doing, it never requeues,
   * cancels or retries anything.
   */
  import { jobsResource } from '../lib/api'
  import { formatTimestamp, truncate } from '../lib/format'
  import PollControls from '../components/PollControls.svelte'

  const statuses = ['pending', 'running', 'done', 'failed']
  const types = ['embed', 'distill', 'cleanup', 'reembed']
  const limit = 25

  // Five is the retry ceiling the reaper works to; the wear meter is drawn
  // against it so a job about to be given up on looks like one.
  const maxAttempts = 5

  let status = $state('')
  let type = $state('')
  let offset = $state(0)

  let jobs = $derived(jobsResource({ status, type, limit, offset }))

  $effect(() => {
    void jobs.reload()
  })

  function changeStatus(event: Event): void {
    status = (event.currentTarget as HTMLSelectElement).value
    offset = 0
  }

  function changeType(event: Event): void {
    type = (event.currentTarget as HTMLSelectElement).value
    offset = 0
  }

  // The endpoint returns a page, not a total, so a full page is the only signal
  // that there may be another.
  let rows = $derived($jobs.data?.jobs ?? [])
  let hasNextPage = $derived(rows.length === limit)
  let page = $derived(Math.floor(offset / limit) + 1)

  const pips = Array.from({ length: maxAttempts }, (_, i) => i)
</script>

<section class="jobs">
  <div class="head">
    <div class="title">
      <h1>Job queue</h1>
      <span class="sub">read-only · nothing here requeues or cancels</span>
    </div>
    <PollControls task={() => jobs.reload()} />
  </div>

  {#if $jobs.data}
    <ul class="chips" data-testid="jobs-breakdown">
      {#each statuses as name (name)}
        <li class="chip s-{name}" data-testid="jobs-count-{name}">
          <span class="k">{name}</span><span class="vh">: </span><span class="n">{$jobs.data.counts[name] ?? 0}</span>
        </li>
      {/each}
    </ul>
  {/if}

  <div class="filters">
    <label>
      Status
      <select data-testid="jobs-status" value={status} onchange={changeStatus}>
        <option value="">any</option>
        {#each statuses as name (name)}
          <option value={name}>{name}</option>
        {/each}
      </select>
    </label>
    <span class="sep" aria-hidden="true"></span>
    <label>
      Type
      <select data-testid="jobs-type" value={type} onchange={changeType}>
        <option value="">any</option>
        {#each types as name (name)}
          <option value={name}>{name}</option>
        {/each}
      </select>
    </label>
    <span class="count">{limit} per page</span>
  </div>

  {#if $jobs.loading && !$jobs.data}
    <p data-testid="jobs-loading">Loading the queue…</p>
  {/if}
  {#if $jobs.error}
    <p class="bad" data-testid="jobs-error">Could not load the queue: {$jobs.error.message}</p>
  {/if}

  {#if $jobs.data}
    {#if rows.length === 0}
      <p class="empty" data-testid="jobs-empty">
        {status || type ? 'No jobs match this filter.' : 'The queue is empty.'}
      </p>
    {:else}
      <table data-testid="jobs-table">
        <thead>
          <tr>
            <th class="c-type">Type</th>
            <th class="c-status">Status</th>
            <th class="c-attempts">Attempts</th>
            <th class="c-time">Runnable at</th>
            <th class="c-time">Locked at</th>
            <th class="c-time">Created</th>
            <th class="c-time">Updated</th>
            <th>Payload</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as job (job.id)}
            <tr data-testid="job-row" class="s-{job.status}">
              <td class="c-type">{job.type}</td>
              <td class="c-status" data-testid="job-status">
                <span class="dot" aria-hidden="true"></span>{job.status}
              </td>
              <td class="c-attempts">
                {job.attempts}
                <!-- The number is the data; the pips just make wear scannable. -->
                <span class="wear" aria-hidden="true">
                  {#each pips as pip (pip)}
                    <i class:on={pip < job.attempts}></i>
                  {/each}
                </span>
              </td>
              <td class="c-time">{formatTimestamp(job.run_after)}</td>
              <td class="c-time locked">{job.locked_at ? formatTimestamp(job.locked_at) : '—'}</td>
              <td class="c-time">{formatTimestamp(job.created_at)}</td>
              <td class="c-time">{formatTimestamp(job.updated_at)}</td>
              <!-- Stored JSON, rendered as text: it is data, not markup. -->
              <td class="payload" data-testid="job-payload">{job.payload ? truncate(job.payload, 60) : '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p class="pager">
        <button
          type="button"
          data-testid="jobs-prev"
          disabled={offset === 0}
          onclick={() => (offset = Math.max(0, offset - limit))}
        >
          Previous
        </button>
        <span data-testid="jobs-page">Page {page}</span>
        <button
          type="button"
          data-testid="jobs-next"
          disabled={!hasNextPage}
          onclick={() => (offset = offset + limit)}
        >
          Next
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

  .title {
    display: flex;
    align-items: baseline;
    gap: 14px;
  }

  .title :global(h1) {
    margin: 0;
  }

  .sub {
    font-size: 11.5px;
    color: var(--dim);
  }

  /* --- whole-queue breakdown -------------------------------------------- */

  .chips {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 10px;
    margin: 0 0 14px;
  }

  .chip {
    display: block;
    padding: 11px 14px;
    background: var(--panel);
    border: 1px solid var(--line-strong);
    border-left: 2px solid var(--tone, var(--line-strong));
  }

  .chip .k {
    display: block;
    font-size: 10.5px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--dim);
  }

  .chip .n {
    display: block;
    font-size: 20px;
  }

  /* --- filters ----------------------------------------------------------- */

  .filters {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: var(--panel);
    border: 1px solid var(--line);
  }

  .filters .sep {
    width: 1px;
    height: 22px;
    background: var(--line);
  }

  .count {
    margin-left: auto;
    font-size: 11px;
    color: var(--dim);
  }

  /* --- table ------------------------------------------------------------- */

  table {
    margin-top: 0;
    table-layout: fixed;
  }

  .c-type {
    width: 92px;
  }
  .c-status {
    width: 92px;
  }
  .c-attempts {
    width: 96px;
  }
  .c-time {
    width: 132px;
  }

  tbody .c-time {
    color: var(--muted);
  }

  thead th:first-child,
  tbody td:first-child {
    padding-left: 14px;
  }

  tbody td:first-child {
    border-left: 2px solid var(--tone, var(--line-strong));
  }

  tbody .c-status {
    color: var(--tone, var(--muted));
    white-space: nowrap;
  }

  .dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    margin-right: 6px;
    background: var(--tone, var(--dim));
    vertical-align: middle;
  }

  .wear {
    display: inline-flex;
    gap: 2px;
    margin-left: 7px;
    vertical-align: middle;
  }

  .wear i {
    display: block;
    width: 4px;
    height: 9px;
    background: var(--line-strong);
  }

  .wear i.on {
    background: var(--tone, var(--cyan));
  }

  .payload {
    color: var(--violet);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Status is hue: one variable drives the gutter, dot, label and pips. */
  .s-pending {
    --tone: var(--amber);
  }
  .s-running {
    --tone: var(--cyan);
  }
  .s-done {
    --tone: var(--green);
  }
  .s-failed {
    --tone: var(--red);
  }

  tbody tr.s-running td {
    background: oklch(0.8 0.14 190 / 0.06);
  }

  tbody tr.s-failed td {
    background: oklch(0.72 0.16 25 / 0.07);
  }

  tbody tr.s-done .c-status {
    color: var(--muted);
  }

  /* A claimed row says so in the one column that means it. */
  tbody tr.s-running .locked {
    color: var(--cyan);
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
