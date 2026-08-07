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

  // `tone` is presentation only: hue carries which subsystem a number belongs
  // to (collection / memory / queue), so the eye can group without reading.
  let counts = $derived(
    $stats.data
      ? [
          { label: 'Runs', value: $stats.data.runs, tone: 'cyan' },
          { label: 'Searches', value: $stats.data.searches, tone: 'cyan' },
          { label: 'URLs', value: $stats.data.urls, tone: 'cyan' },
          { label: 'Scrapes', value: $stats.data.scrapes, tone: 'cyan' },
          { label: 'Memory facts', value: $stats.data.memory_facts, tone: 'violet' },
          { label: 'Cached searches', value: $stats.data.search_cache, tone: 'violet' },
          { label: 'Vectors', value: $stats.data.vectors, tone: 'violet' },
          { label: 'Pending jobs', value: $stats.data.pending_jobs, tone: 'amber' },
        ]
      : [],
  )

  // Sizes share one scale so the four bars are comparable to each other; the
  // largest raw document is almost always the ceiling.
  let sizes = $derived.by(() => {
    const data = $stats.data
    if (!data) return []
    const rows = [
      { label: 'Text, average', chars: data.scrape_text_avg_chars, tone: 'cyan' },
      { label: 'Text, largest', chars: data.scrape_text_max_chars, tone: 'cyan' },
      { label: 'Raw HTML, average', chars: data.scrape_raw_avg_chars, tone: 'violet' },
      { label: 'Raw HTML, largest', chars: data.scrape_raw_max_chars, tone: 'violet' },
    ]
    const ceiling = Math.max(...rows.map((row) => row.chars || 0), 1)
    return rows.map((row) => ({ ...row, pct: Math.round(((row.chars || 0) / ceiling) * 100) }))
  })

  function tierPct(row: { rows: number; tiers?: Record<string, number> }, tier: string): number {
    if (!row.rows) return 0
    return ((row.tiers?.[tier] ?? 0) / row.rows) * 100
  }
</script>

<section class="stats">
  <h1>Stats</h1>

  {#if $stats.loading && !$stats.data}
    <p data-testid="stats-loading">Loading stats…</p>
  {/if}
  {#if $stats.error}
    <p class="bad" data-testid="stats-error">Could not load stats: {$stats.error.message}</p>
  {/if}

  {#if $stats.data}
    <ul class="tiles" data-testid="stats-counts">
      {#each counts as tile (tile.label)}
        <li class="tile tone-{tile.tone}" data-testid="stats-count">
          <span class="k">{tile.label}</span><span class="vh">: </span><span class="n">{tile.value}</span>
        </li>
      {/each}
    </ul>

    <div class="cols">
      <div class="panel">
        <h2>Scraped material</h2>
        <ul class="meters" data-testid="stats-sizes">
          {#each sizes as row (row.label)}
            <li>
              <span class="k">{row.label}</span><span class="vh">: </span><span class="n">{formatChars(row.chars)}</span>
              <span class="bar tone-{row.tone}" aria-hidden="true"><i style="width:{row.pct}%"></i></span>
            </li>
          {/each}
        </ul>
      </div>

      <div class="panel">
        <h2>Embeddings</h2>
        <ul class="pairs" data-testid="stats-embedding">
          <li><span class="k">Model</span><span class="vh">: </span><span class="n">{$stats.data.embed_model || 'unknown'}</span></li>
          <li><span class="k">Dimensions</span><span class="vh">: </span><span class="n">{$stats.data.embed_dim || 'unknown'}</span></li>
          <li><span class="k">Active table</span><span class="vh">: </span><span class="n">{$stats.data.vector_table || 'none yet'}</span></li>
          <li><span class="k">Similarity search</span><span class="vh">: </span><span class="n">exact linear scan</span></li>
        </ul>
        {#if $stats.data.vector_migration_in_progress}
          <p class="warn" data-testid="stats-migrating">
            A re-embed migration is in progress. Semantic search is unavailable until it finishes, and the
            vector count above is the previous generation's.
          </p>
        {/if}
      </div>
    </div>

    <h2>Caches</h2>
    <!--
      Hit counts, not hit rates: the schema records hit_count on the rows that
      exist and nothing at all about lookups that missed, so a rate would have
      to be invented. Counting misses means writing new counters, which the
      read-only UI does not do.
    -->
    <p class="note" data-testid="stats-hit-note">
      Reuse is shown as hit counts, not a hit rate: misses leave no record to count.
    </p>
    <table data-testid="stats-caches">
      <thead>
        <tr>
          <th>Cache</th>
          <th class="num">Rows</th>
          <th class="spread">Tiers</th>
          {#each tiers as tier (tier)}
            <th class="num">{tier}</th>
          {/each}
          <th class="num">Expired</th>
          <th class="num">Rows reused</th>
          <th class="num">Total hits</th>
        </tr>
      </thead>
      <tbody>
        {#each [{ label: 'Search', data: $stats.data.search_cache_stats }, { label: 'Scrape', data: $stats.data.scrape_cache_stats }] as row (row.label)}
          <tr data-testid="stats-cache-row">
            <td>{row.label}</td>
            <td class="num">{row.data.rows}</td>
            <td class="spread">
              <span class="stack" aria-hidden="true">
                <i class="tone-cyan" style="width:{tierPct(row.data, 'short')}%"></i>
                <i class="tone-green" style="width:{tierPct(row.data, 'long')}%"></i>
                <i class="tone-violet" style="width:{tierPct(row.data, 'permanent')}%"></i>
              </span>
            </td>
            {#each tiers as tier (tier)}
              <td class="num">{row.data.tiers?.[tier] ?? 0}</td>
            {/each}
            <td class="num expired">{row.data.expired}</td>
            <td class="num">{row.data.rows_with_hits}</td>
            <td class="num hits">{row.data.total_hits}</td>
          </tr>
        {/each}
      </tbody>
    </table>

    <h2>Job queue</h2>
    <ul class="chips" data-testid="stats-jobs">
      {#each Object.entries($stats.data.jobs.by_status) as [status, count] (status)}
        <li class="chip s-{status}" data-testid="stats-job-status">
          <span class="k">{status}</span><span class="vh">: </span><span class="n">{count}</span>
        </li>
      {/each}
    </ul>

    <div class="cols">
      <div class="panel">
        <ul class="pairs" data-testid="stats-job-types">
          {#each Object.entries($stats.data.jobs.by_type) as [type, count] (type)}
            <li data-testid="stats-job-type"><span class="k">{type}</span><span class="vh">: </span><span class="n">{count}</span></li>
          {:else}
            <li class="empty" data-testid="stats-job-types-empty">No jobs have been enqueued.</li>
          {/each}
        </ul>
      </div>

      <div class="panel">
        <ul class="pairs" data-testid="stats-job-timing">
          <li><span class="k">Retried more than once</span><span class="vh">: </span><span class="n">{$stats.data.jobs.retried}</span></li>
          <li><span class="k">Most attempts on one job</span><span class="vh">: </span><span class="n">{$stats.data.jobs.max_attempts}</span></li>
          <li>
            <span class="k">Oldest pending job</span><span class="vh">: </span>
            <span class="n">
              {$stats.data.jobs.oldest_pending_at ? formatTimestamp($stats.data.jobs.oldest_pending_at) : 'none waiting'}
            </span>
          </li>
          <li>
            <span class="k">Average time to finish</span><span class="vh">: </span>
            <span class="n">
              {$stats.data.jobs.completed_sampled > 0
                ? `${formatDuration($stats.data.jobs.avg_completion_ms)} over the last ${$stats.data.jobs.completed_sampled}`
                : 'nothing finished yet'}
            </span>
          </li>
        </ul>
      </div>
    </div>
  {/if}
</section>

<style>
  /* Tone is hue only — same lightness and chroma across all four signals. */
  .tone-cyan {
    --tone: var(--cyan);
  }
  .tone-violet {
    --tone: var(--violet);
  }
  .tone-amber {
    --tone: var(--amber);
  }
  .tone-green {
    --tone: var(--green);
  }

  .stats :global(h2) {
    margin-top: 28px;
  }

  /* --- headline counters ------------------------------------------------ */

  .tiles {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1px;
    background: var(--line);
    border: 1px solid var(--line);
    margin: 4px 0 0;
  }

  .tile {
    background: var(--panel);
    padding: 16px 20px 14px;
    border: none;
    position: relative;
    display: block;
  }

  .tile::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: 2px;
    background: var(--tone);
  }

  .tile .k {
    display: block;
    font-size: 10.5px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--dim);
  }

  .tile .n {
    display: block;
    font-size: 34px;
    line-height: 1.15;
    letter-spacing: -0.04em;
    color: var(--text);
  }

  .tile.tone-amber .n {
    color: var(--amber);
  }

  /* --- panels ------------------------------------------------------------ */

  .cols {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-top: 16px;
  }

  .panel {
    background: var(--panel);
    border: 1px solid var(--line);
    padding: 4px 18px 16px;
  }

  .panel :global(h2) {
    margin-top: 16px;
  }

  .pairs,
  .meters {
    margin: 8px 0 0;
  }

  .pairs > li {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 16px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
    color: var(--muted);
  }

  .pairs > li:last-child {
    border-bottom: none;
  }

  .pairs .n {
    color: var(--text);
    text-align: right;
  }

  .pairs .empty {
    color: var(--dim);
  }

  .meters > li {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 4px 12px;
    padding: 9px 0;
    border-bottom: 1px solid var(--line);
    color: var(--muted);
  }

  .meters > li:last-child {
    border-bottom: none;
  }

  .meters .n {
    color: var(--text);
  }

  .bar {
    grid-column: 1 / -1;
    display: block;
    height: 8px;
    background: var(--panel-3);
  }

  .bar i {
    display: block;
    height: 100%;
    background: var(--tone);
  }

  /* --- caches ------------------------------------------------------------ */

  .note {
    margin: 0 0 4px;
    color: var(--dim);
    font-size: 11.5px;
  }

  .spread {
    width: 180px;
  }

  .stack {
    display: flex;
    height: 8px;
    background: var(--panel-3);
    margin-top: 4px;
  }

  .stack i {
    display: block;
    height: 100%;
    background: var(--tone);
  }

  .expired {
    color: oklch(0.76 0.12 40);
  }

  .hits {
    color: var(--green);
  }

  /* --- job queue --------------------------------------------------------- */

  .chips {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 10px;
    margin: 8px 0 0;
  }

  .chip {
    border: 1px solid var(--line-strong);
    border-left: 2px solid var(--tone, var(--line-strong));
    background: var(--panel);
    padding: 11px 14px;
    display: block;
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
    color: var(--text);
  }

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
  .s-failed .n {
    color: var(--red);
  }

  .warn {
    color: var(--amber);
    font-size: 11.5px;
  }

  .bad {
    color: var(--red);
  }
</style>
