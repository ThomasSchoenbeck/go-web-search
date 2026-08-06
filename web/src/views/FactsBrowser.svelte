<script lang="ts">
  import { factsResource } from '../lib/api'
  import { formatTimestamp, truncate } from '../lib/format'

  const pageSizes = [25, 50, 100]

  let q = $state('')
  let draft = $state('')
  let limit = $state(25)
  let offset = $state(0)

  let facts = $derived(factsResource({ q, limit, offset }))

  $effect(() => {
    void facts.reload()
  })

  function submitSearch(event: SubmitEvent): void {
    event.preventDefault()
    q = draft.trim()
    offset = 0 // a new filter starts at the first page
  }

  function clearSearch(): void {
    draft = ''
    q = ''
    offset = 0
  }

  function changeLimit(event: Event): void {
    limit = Number((event.currentTarget as HTMLSelectElement).value)
    offset = 0
  }

  // The endpoint returns a page, not a total, so "there is a next page" is
  // inferred from getting a full one.
  let hasNextPage = $derived(($facts.data?.length ?? 0) === limit)
  let page = $derived(Math.floor(offset / limit) + 1)
</script>

<section>
  <h1>Memory facts</h1>

  <form onsubmit={submitSearch}>
    <label>
      Search
      <input type="search" data-testid="facts-search" placeholder="substring match" bind:value={draft} />
    </label>
    <button type="submit" data-testid="facts-search-submit">Search</button>
    <button type="button" data-testid="facts-search-clear" onclick={clearSearch}>Clear</button>
  </form>

  <label>
    Per page
    <select data-testid="facts-limit" value={limit} onchange={changeLimit}>
      {#each pageSizes as size (size)}
        <option value={size}>{size}</option>
      {/each}
    </select>
  </label>

  {#if $facts.loading && !$facts.data}
    <p data-testid="facts-loading">Loading facts…</p>
  {/if}
  {#if $facts.error}
    <p data-testid="facts-error">Could not load facts: {$facts.error.message}</p>
  {/if}

  {#if $facts.data}
    {#if $facts.data.length === 0}
      <p data-testid="facts-empty">
        {q ? `No facts match “${q}”.` : 'Nothing has been distilled into memory yet.'}
      </p>
    {:else}
      <table data-testid="facts-table">
        <thead>
          <tr>
            <th>Fact</th>
            <th>Source</th>
            <th>Volatility</th>
            <th>Tier</th>
            <th>Hits</th>
            <th>Expires</th>
          </tr>
        </thead>
        <tbody>
          {#each $facts.data as fact (fact.id)}
            <tr>
              <td><a href="/facts/{fact.id}" data-testid="fact-link">{truncate(fact.text, 90)}</a></td>
              <td>
                {#if fact.source_url}
                  <a
                    href="/provenance?url={encodeURIComponent(fact.source_url)}"
                    data-testid="fact-source-link"
                  >
                    {truncate(fact.source_url, 40)}
                  </a>
                {:else}
                  —
                {/if}
              </td>
              <td>{fact.volatility || '—'}</td>
              <td>{fact.tier ?? '—'}</td>
              <td>{fact.hit_count}</td>
              <td>{fact.expires_at ? formatTimestamp(fact.expires_at) : 'never'}</td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p>
        <button
          type="button"
          data-testid="facts-prev"
          disabled={offset === 0}
          onclick={() => (offset = Math.max(0, offset - limit))}
        >
          Previous
        </button>
        <span data-testid="facts-page">Page {page}</span>
        <button
          type="button"
          data-testid="facts-next"
          disabled={!hasNextPage}
          onclick={() => (offset = offset + limit)}
        >
          Next
        </button>
      </p>
    {/if}
  {/if}
</section>
