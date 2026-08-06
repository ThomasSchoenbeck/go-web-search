<script lang="ts">
  /**
   * The scrape cache, row by row: which pages are stored, how large they are,
   * and how the sliding expiry has treated them. Bodies stay behind the scrape
   * detail view — this lists sizes, not content.
   */
  import { scrapeCacheResource } from '../lib/api'
  import { formatChars, formatTimestamp, truncate } from '../lib/format'

  const tiers = ['short', 'long', 'permanent']
  const limit = 25

  let tier = $state('')
  let q = $state('')
  let draft = $state('')
  let offset = $state(0)

  let entries = $derived(scrapeCacheResource({ tier, q, limit, offset }))

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
  <h1>Scrape cache</h1>
  <p><a href="/cache/searches" data-testid="to-search-cache">← Search cache</a></p>

  <form onsubmit={submitFilter}>
    <label>
      URL or domain
      <input type="search" data-testid="scrape-cache-filter" placeholder="substring match" bind:value={draft} />
    </label>
    <button type="submit" data-testid="scrape-cache-submit">Filter</button>
    <button type="button" data-testid="scrape-cache-clear" onclick={clearFilter}>Clear</button>
  </form>

  <label>
    Tier
    <select data-testid="scrape-cache-tier" value={tier} onchange={changeTier}>
      <option value="">any</option>
      {#each tiers as name (name)}
        <option value={name}>{name}</option>
      {/each}
    </select>
  </label>

  {#if $entries.loading && !$entries.data}
    <p data-testid="scrape-cache-loading">Loading the scrape cache…</p>
  {/if}
  {#if $entries.error}
    <p data-testid="scrape-cache-error">Could not load the scrape cache: {$entries.error.message}</p>
  {/if}

  {#if $entries.data}
    {#if rows.length === 0}
      <p data-testid="scrape-cache-empty">
        {tier || q ? 'No cached pages match this filter.' : 'Nothing has been cached yet.'}
      </p>
    {:else}
      <table data-testid="scrape-cache-table">
        <thead>
          <tr>
            <th>URL</th>
            <th>Status</th>
            <th>Type</th>
            <th>Title</th>
            <th>Tier</th>
            <th>Hits</th>
            <th>Text</th>
            <th>Clean</th>
            <th>Raw</th>
            <th>Robots</th>
            <th>Fetched</th>
            <th>Expires</th>
            <th>Trace</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as entry (entry.id)}
            <tr data-testid="scrape-cache-row">
              <td>
                <a href="/scrapes/{entry.id}" data-testid="scrape-cache-detail-link">{truncate(entry.url, 50)}</a>
              </td>
              <td>{entry.http_status ?? '—'}{entry.error ? ` · ${entry.error}` : ''}</td>
              <td>{entry.content_type ?? '—'}</td>
              <td>{entry.title ? truncate(entry.title, 30) : '—'}</td>
              <td data-testid="scrape-cache-tier-cell">{entry.tier}</td>
              <td>{entry.hit_count}</td>
              <td>{formatChars(entry.text_chars)}</td>
              <td>{formatChars(entry.clean_html_chars)}</td>
              <td>{formatChars(entry.raw_html_chars)}</td>
              <td>{entry.robots_allowed ? 'allowed' : 'blocked'}</td>
              <td>{formatTimestamp(entry.fetched_at)}</td>
              <td>{entry.expires_at ? formatTimestamp(entry.expires_at) : 'never'}</td>
              <td>
                <a href="/provenance?url={encodeURIComponent(entry.url)}" data-testid="scrape-cache-provenance-link">
                  provenance
                </a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p>
        <button
          type="button"
          data-testid="scrape-cache-prev"
          disabled={offset === 0}
          onclick={() => (offset = Math.max(0, offset - limit))}
        >
          Previous
        </button>
        <span data-testid="scrape-cache-page">Page {page}</span>
        <button
          type="button"
          data-testid="scrape-cache-next"
          disabled={!hasNextPage}
          onclick={() => (offset = offset + limit)}
        >
          Next
        </button>
      </p>
    {/if}
  {/if}
</section>
