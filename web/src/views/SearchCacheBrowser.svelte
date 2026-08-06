<script lang="ts">
  /**
   * The search cache, row by row: which queries are stored, how durable they
   * are, and how often they have been reused. The stored result URLs are
   * summarised, not listed — this view is about cache health.
   */
  import { searchCacheResource } from '../lib/api'
  import { formatChars, formatTimestamp, truncate } from '../lib/format'

  const tiers = ['short', 'long', 'permanent']
  const limit = 25

  let tier = $state('')
  let q = $state('')
  let draft = $state('')
  let offset = $state(0)

  let entries = $derived(searchCacheResource({ tier, q, limit, offset }))

  $effect(() => {
    void entries.reload()
  })

  function submitFilter(event: SubmitEvent): void {
    event.preventDefault()
    q = draft.trim()
    offset = 0
  }

  function clearFilter(): void {
    draft = ''
    q = ''
    offset = 0
  }

  function changeTier(event: Event): void {
    tier = (event.currentTarget as HTMLSelectElement).value
    offset = 0
  }

  let rows = $derived($entries.data ?? [])
  let hasNextPage = $derived(rows.length === limit)
  let page = $derived(Math.floor(offset / limit) + 1)
</script>

<section>
  <h1>Search cache</h1>
  <p><a href="/cache/scrapes" data-testid="to-scrape-cache">Scrape cache →</a></p>

  <form onsubmit={submitFilter}>
    <label>
      Query
      <input type="search" data-testid="search-cache-filter" placeholder="substring match" bind:value={draft} />
    </label>
    <button type="submit" data-testid="search-cache-submit">Filter</button>
    <button type="button" data-testid="search-cache-clear" onclick={clearFilter}>Clear</button>
  </form>

  <label>
    Tier
    <select data-testid="search-cache-tier" value={tier} onchange={changeTier}>
      <option value="">any</option>
      {#each tiers as name (name)}
        <option value={name}>{name}</option>
      {/each}
    </select>
  </label>

  {#if $entries.loading && !$entries.data}
    <p data-testid="search-cache-loading">Loading the search cache…</p>
  {/if}
  {#if $entries.error}
    <p data-testid="search-cache-error">Could not load the search cache: {$entries.error.message}</p>
  {/if}

  {#if $entries.data}
    {#if rows.length === 0}
      <p data-testid="search-cache-empty">
        {tier || q ? 'No cached queries match this filter.' : 'Nothing has been cached yet.'}
      </p>
    {:else}
      <table data-testid="search-cache-table">
        <thead>
          <tr>
            <th>Query</th>
            <th>Tier</th>
            <th>Hits</th>
            <th>Results</th>
            <th>Size</th>
            <th>Fetched</th>
            <th>Expires</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as entry (entry.id)}
            <tr data-testid="search-cache-row">
              <td>{truncate(entry.query, 60)}</td>
              <td data-testid="search-cache-tier-cell">{entry.tier}</td>
              <td>{entry.hit_count}</td>
              <td>{entry.result_count}</td>
              <td>{formatChars(entry.results_chars)}</td>
              <td>{formatTimestamp(entry.fetched_at)}</td>
              <td>{entry.expires_at ? formatTimestamp(entry.expires_at) : 'never'}</td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p>
        <button
          type="button"
          data-testid="search-cache-prev"
          disabled={offset === 0}
          onclick={() => (offset = Math.max(0, offset - limit))}
        >
          Previous
        </button>
        <span data-testid="search-cache-page">Page {page}</span>
        <button
          type="button"
          data-testid="search-cache-next"
          disabled={!hasNextPage}
          onclick={() => (offset = offset + limit)}
        >
          Next
        </button>
      </p>
    {/if}
  {/if}
</section>
