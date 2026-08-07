<script lang="ts">
  import { runSearchesResource } from '../lib/api'
  import { formatDuration, formatTimestamp } from '../lib/format'

  interface Props {
    runId: string
  }

  const { runId }: Props = $props()

  let searches = $derived(runSearchesResource(runId))

  $effect(() => {
    void searches.reload()
  })
</script>

<section>
  <p><a href="/runs/{runId}" data-testid="back-to-run">← Run {runId}</a></p>
  <h1>Searches</h1>

  {#if $searches.loading && !$searches.data}
    <p data-testid="searches-loading">Loading searches…</p>
  {/if}
  {#if $searches.error}
    <p data-testid="searches-error">Could not load searches: {$searches.error.message}</p>
  {/if}

  {#if $searches.data}
    {#if $searches.data.length === 0}
      <p data-testid="searches-empty">This run ran no searches.</p>
    {:else}
      <table data-testid="searches-table">
        <thead>
          <tr>
            <th>Term</th>
            <th>Engine</th>
            <th>Mode</th>
            <th>Status</th>
            <th>Blocked</th>
            <th>Anchors</th>
            <th>Duration</th>
            <th>Started</th>
            <th>Error</th>
            <th>SERP</th>
          </tr>
        </thead>
        <tbody>
          {#each $searches.data as search (search.id)}
            <tr data-testid="search-row">
              <td>{search.term}</td>
              <td>{search.engine}</td>
              <td>{search.search_mode}</td>
              <td>{search.http_status ?? '—'}</td>
              <td>{search.blocked ? 'yes' : 'no'}</td>
              <td>{search.anchor_count}</td>
              <td>{formatDuration(search.duration_ms)}</td>
              <td>{formatTimestamp(search.created_at)}</td>
              <td data-testid="search-error">{search.error ?? ''}</td>
              <td><a href="/searches/{search.id}" data-testid="serp-link">View SERP</a></td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  {/if}
</section>
