<script lang="ts">
  /**
   * Everything /api/stats knows, on one page: what has been collected, how
   * bloated the scraped material is, which embedding model is active, how the
   * two caches are aging, and what the job queue has been doing.
   */
  import { statsResource } from '../lib/api'
  import { formatChars, formatDuration, formatTimestamp } from '../lib/format'

  const tiers = ['short', 'long', 'permanent']

  let stats = $derived(statsResource())

  $effect(() => {
    void stats.reload()
  })

  let counts = $derived(
    $stats.data
      ? [
          { label: 'Runs', value: $stats.data.runs },
          { label: 'Searches', value: $stats.data.searches },
          { label: 'URLs', value: $stats.data.urls },
          { label: 'Scrapes', value: $stats.data.scrapes },
          { label: 'Memory facts', value: $stats.data.memory_facts },
          { label: 'Cached searches', value: $stats.data.search_cache },
          { label: 'Vectors', value: $stats.data.vectors },
          { label: 'Pending jobs', value: $stats.data.pending_jobs },
        ]
      : [],
  )
</script>

<section>
  <h1>Stats</h1>

  {#if $stats.loading && !$stats.data}
    <p data-testid="stats-loading">Loading stats…</p>
  {/if}
  {#if $stats.error}
    <p data-testid="stats-error">Could not load stats: {$stats.error.message}</p>
  {/if}

  {#if $stats.data}
    <ul data-testid="stats-counts">
      {#each counts as tile (tile.label)}
        <li data-testid="stats-count">{tile.label}: {tile.value}</li>
      {/each}
    </ul>

    <h2>Scraped material</h2>
    <ul data-testid="stats-sizes">
      <li>Text, average: {formatChars($stats.data.scrape_text_avg_chars)}</li>
      <li>Text, largest: {formatChars($stats.data.scrape_text_max_chars)}</li>
      <li>Raw HTML, average: {formatChars($stats.data.scrape_raw_avg_chars)}</li>
      <li>Raw HTML, largest: {formatChars($stats.data.scrape_raw_max_chars)}</li>
    </ul>

    <h2>Embeddings</h2>
    <ul data-testid="stats-embedding">
      <li>Model: {$stats.data.embed_model || 'unknown'}</li>
      <li>Dimensions: {$stats.data.embed_dim || 'unknown'}</li>
      <li>Active table: {$stats.data.vector_table || 'none yet'}</li>
    </ul>
    {#if $stats.data.vector_migration_in_progress}
      <p data-testid="stats-migrating">
        A re-embed migration is in progress. Semantic search is unavailable until it finishes, and the
        vector count above is the previous generation's.
      </p>
    {/if}

    <h2>Caches</h2>
    <!--
      Hit counts, not hit rates: the schema records hit_count on the rows that
      exist and nothing at all about lookups that missed, so a rate would have
      to be invented. Counting misses means writing new counters, which the
      read-only UI does not do.
    -->
    <p data-testid="stats-hit-note">
      Reuse is shown as hit counts, not a hit rate: misses leave no record to count.
    </p>
    <table data-testid="stats-caches">
      <thead>
        <tr>
          <th>Cache</th>
          <th>Rows</th>
          {#each tiers as tier (tier)}
            <th>{tier}</th>
          {/each}
          <th>Expired</th>
          <th>Rows reused</th>
          <th>Total hits</th>
        </tr>
      </thead>
      <tbody>
        {#each [{ label: 'Search', data: $stats.data.search_cache_stats }, { label: 'Scrape', data: $stats.data.scrape_cache_stats }] as row (row.label)}
          <tr data-testid="stats-cache-row">
            <td>{row.label}</td>
            <td>{row.data.rows}</td>
            {#each tiers as tier (tier)}
              <td>{row.data.tiers?.[tier] ?? 0}</td>
            {/each}
            <td>{row.data.expired}</td>
            <td>{row.data.rows_with_hits}</td>
            <td>{row.data.total_hits}</td>
          </tr>
        {/each}
      </tbody>
    </table>

    <h2>Job queue</h2>
    <ul data-testid="stats-jobs">
      {#each Object.entries($stats.data.jobs.by_status) as [status, count] (status)}
        <li data-testid="stats-job-status">{status}: {count}</li>
      {/each}
    </ul>
    <ul data-testid="stats-job-types">
      {#each Object.entries($stats.data.jobs.by_type) as [type, count] (type)}
        <li data-testid="stats-job-type">{type}: {count}</li>
      {:else}
        <li data-testid="stats-job-types-empty">No jobs have been enqueued.</li>
      {/each}
    </ul>
    <ul data-testid="stats-job-timing">
      <li>Retried more than once: {$stats.data.jobs.retried}</li>
      <li>Most attempts on one job: {$stats.data.jobs.max_attempts}</li>
      <li>
        Oldest pending job:
        {$stats.data.jobs.oldest_pending_at ? formatTimestamp($stats.data.jobs.oldest_pending_at) : 'none waiting'}
      </li>
      <li>
        Average time to finish: {$stats.data.jobs.completed_sampled > 0
          ? `${formatDuration($stats.data.jobs.avg_completion_ms)} over the last ${$stats.data.jobs.completed_sampled}`
          : 'nothing finished yet'}
      </li>
    </ul>
  {/if}
</section>
