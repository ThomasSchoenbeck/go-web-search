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
</script>

<section>
  <h1>Job queue</h1>

  <PollControls task={() => jobs.reload()} />

  {#if $jobs.data}
    <ul data-testid="jobs-breakdown">
      {#each statuses as name (name)}
        <li data-testid="jobs-count-{name}">{name}: {$jobs.data.counts[name] ?? 0}</li>
      {/each}
    </ul>
  {/if}

  <label>
    Status
    <select data-testid="jobs-status" value={status} onchange={changeStatus}>
      <option value="">any</option>
      {#each statuses as name (name)}
        <option value={name}>{name}</option>
      {/each}
    </select>
  </label>

  <label>
    Type
    <select data-testid="jobs-type" value={type} onchange={changeType}>
      <option value="">any</option>
      {#each types as name (name)}
        <option value={name}>{name}</option>
      {/each}
    </select>
  </label>

  {#if $jobs.loading && !$jobs.data}
    <p data-testid="jobs-loading">Loading the queue…</p>
  {/if}
  {#if $jobs.error}
    <p data-testid="jobs-error">Could not load the queue: {$jobs.error.message}</p>
  {/if}

  {#if $jobs.data}
    {#if rows.length === 0}
      <p data-testid="jobs-empty">
        {status || type ? 'No jobs match this filter.' : 'The queue is empty.'}
      </p>
    {:else}
      <table data-testid="jobs-table">
        <thead>
          <tr>
            <th>Type</th>
            <th>Status</th>
            <th>Attempts</th>
            <th>Runnable at</th>
            <th>Locked at</th>
            <th>Created</th>
            <th>Updated</th>
            <th>Payload</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as job (job.id)}
            <tr data-testid="job-row">
              <td>{job.type}</td>
              <td data-testid="job-status">{job.status}</td>
              <td>{job.attempts}</td>
              <td>{formatTimestamp(job.run_after)}</td>
              <td>{job.locked_at ? formatTimestamp(job.locked_at) : '—'}</td>
              <td>{formatTimestamp(job.created_at)}</td>
              <td>{formatTimestamp(job.updated_at)}</td>
              <!-- Stored JSON, rendered as text: it is data, not markup. -->
              <td data-testid="job-payload">{job.payload ? truncate(job.payload, 60) : '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p>
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
