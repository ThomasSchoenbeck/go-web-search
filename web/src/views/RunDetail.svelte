<script lang="ts">
  import { runResource, runUrlsResource, runSearchesResource, runScrapeIdsResource } from '../lib/api'
  import { formatTimestamp, formatDuration, truncate } from '../lib/format'

  interface Props {
    id: string
  }

  const { id }: Props = $props()

  // Rebuilt whenever the route id changes, so navigating between runs refetches
  // rather than showing the previous run's children.
  let run = $derived(runResource(id))
  let urls = $derived(runUrlsResource(id))
  let searches = $derived(runSearchesResource(id))
  let scrapeIds = $derived(runScrapeIdsResource(id))

  $effect(() => {
    void run.reload()
    void urls.reload()
    void searches.reload()
    void scrapeIds.reload()
  })
</script>

<section>
  <p><a href="/runs" data-testid="back-to-runs">← All runs</a></p>
  <h1>Run {id}</h1>
  <p>
    <a href="/runs/{id}/causality" data-testid="run-causality-link">
      See what caused what in this run →
    </a>
  </p>

  {#if $run.loading && !$run.data}
    <p data-testid="run-loading">Loading run…</p>
  {/if}
  {#if $run.error}
    <p data-testid="run-error">Could not load this run: {$run.error.message}</p>
  {/if}

  {#if $run.data}
    <dl data-testid="run-summary">
      <dt>Mode</dt>
      <dd>{$run.data.mode}</dd>
      <dt>Started</dt>
      <dd>{formatTimestamp($run.data.started_at)}</dd>
      <dt>Finished</dt>
      <dd>{$run.data.finished_at ? formatTimestamp($run.data.finished_at) : 'still running'}</dd>
      <dt>Counts</dt>
      <dd>{$run.data.searches} searches · {$run.data.urls} URLs · {$run.data.scrapes} scrapes</dd>
    </dl>
  {/if}

  <h2>Searches</h2>
  <p><a href="/runs/{id}/searches" data-testid="all-searches-link">Open the searches view →</a></p>
  {#if $searches.error}
    <p data-testid="searches-error">{$searches.error.message}</p>
  {:else if $searches.data}
    {#if $searches.data.length === 0}
      <p data-testid="searches-empty">This run ran no searches.</p>
    {:else}
      <ul data-testid="searches-list">
        {#each $searches.data as search (search.id)}
          <li>
            <a href="/searches/{search.id}" data-testid="search-link">
              {search.engine} · {search.term}
            </a>
            <span>{search.anchor_count} anchors · {formatDuration(search.duration_ms)}</span>
            {#if search.blocked}<strong data-testid="search-blocked">blocked</strong>{/if}
          </li>
        {/each}
      </ul>
    {/if}
  {/if}

  <h2>URLs</h2>
  {#if $urls.error}
    <p data-testid="urls-error">{$urls.error.message}</p>
  {:else if $urls.data}
    {#if $urls.data.length === 0}
      <p data-testid="urls-empty">This run found no URLs.</p>
    {:else}
      <table data-testid="urls-table">
        <thead>
          <tr><th>Rank</th><th>URL</th><th>Domain</th></tr>
        </thead>
        <tbody>
          {#each $urls.data as row (row.id)}
            <tr>
              <td>{row.rank ?? '—'}</td>
              <td><a href={row.url} target="_blank" rel="noreferrer noopener">{truncate(row.url)}</a></td>
              <td>{row.domain}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  {/if}

  <h2>Scrapes</h2>
  {#if $scrapeIds.error}
    <p data-testid="scrapes-error">{$scrapeIds.error.message}</p>
  {:else if $scrapeIds.data}
    {#if $scrapeIds.data.length === 0}
      <p data-testid="scrapes-empty">This run produced no scrapes.</p>
    {:else}
      <ul data-testid="scrapes-list">
        {#each $scrapeIds.data as scrapeId (scrapeId)}
          <li><a href="/scrapes/{scrapeId}" data-testid="scrape-link">{scrapeId}</a></li>
        {/each}
      </ul>
    {/if}
  {/if}
</section>
