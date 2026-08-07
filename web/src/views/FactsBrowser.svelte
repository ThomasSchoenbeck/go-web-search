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

  /** Host only: the full URL is the link target, not the label. */
  function sourceLabel(url: string): string {
    try {
      const parsed = new URL(url)
      return truncate(`${parsed.host}${parsed.pathname === '/' ? '' : parsed.pathname}`, 52)
    } catch {
      return truncate(url, 52)
    }
  }
</script>

<section class="facts">
  <div class="head">
    <h1>Memory facts</h1>
    <span class="sub">atomic claims distilled from scraped pages, each embedded and tiered</span>
  </div>

  <!-- Search, clear and page size read as one strip; the field takes the room. -->
  <form class="searchbar" onsubmit={submitSearch}>
    <input
      type="search"
      data-testid="facts-search"
      placeholder="substring match across every stored fact"
      bind:value={draft}
    />
    <button class="primary" type="submit" data-testid="facts-search-submit">Search</button>
    <button class="ghost" type="button" data-testid="facts-search-clear" onclick={clearSearch}>Clear</button>
    <label>
      Per page
      <select data-testid="facts-limit" value={limit} onchange={changeLimit}>
        {#each pageSizes as size (size)}
          <option value={size}>{size}</option>
        {/each}
      </select>
    </label>
  </form>

  {#if $facts.loading && !$facts.data}
    <p data-testid="facts-loading">Loading facts…</p>
  {/if}
  {#if $facts.error}
    <p class="bad" data-testid="facts-error">Could not load facts: {$facts.error.message}</p>
  {/if}

  {#if $facts.data}
    {#if $facts.data.length === 0}
      <p class="empty" data-testid="facts-empty">
        {q ? `No facts match “${q}”.` : 'Nothing has been distilled into memory yet.'}
      </p>
    {:else}
      <table data-testid="facts-table">
        <thead>
          <tr>
            <th>Fact</th>
            <th class="c-vol">Volatility</th>
            <th class="c-tier">Tier</th>
            <th class="c-hits num">Hits</th>
            <th class="c-exp">Expires</th>
          </tr>
        </thead>
        <tbody>
          {#each $facts.data as fact (fact.id)}
            <tr class="t-{fact.tier ?? 'none'}">
              <!-- The claim is the content; its source rides underneath it. -->
              <td class="fact">
                <a class="claim" href="/facts/{fact.id}" data-testid="fact-link">{truncate(fact.text, 180)}</a>
                <span class="source">
                  {#if fact.source_url}
                    <a href="/provenance?url={encodeURIComponent(fact.source_url)}" data-testid="fact-source-link">
                      {sourceLabel(fact.source_url)}
                    </a>
                  {:else}
                    stored by hand · no source url
                  {/if}
                </span>
              </td>
              <td class="c-vol" class:volatile={fact.volatility === 'volatile'}>{fact.volatility || '—'}</td>
              <td class="c-tier"><span class="tier">{fact.tier ?? '—'}</span></td>
              <td class="c-hits num" class:used={fact.hit_count > 0}>{fact.hit_count}</td>
              <td class="c-exp">{fact.expires_at ? formatTimestamp(fact.expires_at) : 'never'}</td>
            </tr>
          {/each}
        </tbody>
      </table>

      <p class="pager">
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

<style>
  .head {
    display: flex;
    align-items: baseline;
    gap: 14px;
    margin-bottom: 14px;
  }

  .head :global(h1) {
    margin: 0;
  }

  .sub {
    font-size: 11.5px;
    color: var(--dim);
  }

  .searchbar {
    display: flex;
    align-items: stretch;
    gap: 0;
    margin: 0;
    background: var(--panel);
    border: 1px solid var(--line);
  }

  .searchbar input {
    flex: 1;
    background: transparent;
    border: none;
    padding: 9px 14px;
  }

  .searchbar button {
    border: none;
    padding: 9px 16px;
  }

  .searchbar .primary {
    background: var(--cyan);
    color: var(--bg);
  }

  .searchbar .ghost {
    background: transparent;
    color: var(--muted);
    border-left: 1px solid var(--line);
  }

  .searchbar label {
    margin: 0;
    padding: 0 14px;
    border-left: 1px solid var(--line);
  }

  .searchbar select {
    padding: 5px 8px;
  }

  table {
    margin-top: 0;
    table-layout: fixed;
  }

  .c-vol {
    width: 96px;
  }
  .c-tier {
    width: 104px;
  }
  .c-hits {
    width: 64px;
  }
  .c-exp {
    width: 150px;
  }

  thead th:first-child,
  tbody td:first-child {
    padding-left: 14px;
  }

  /* Tier is the row's hue: gutter and chip come from one variable. */
  .t-short {
    --tone: var(--cyan);
  }
  .t-long {
    --tone: var(--green);
  }
  .t-permanent {
    --tone: var(--violet);
  }

  tbody td:first-child {
    border-left: 2px solid var(--tone, var(--line-strong));
  }

  tbody td {
    padding-top: 11px;
    padding-bottom: 11px;
  }

  .claim {
    display: block;
    color: oklch(0.93 0.008 265);
    font-size: 13px;
    line-height: 1.5;
    border: none;
  }

  .claim:hover {
    color: #fff;
  }

  .source {
    display: block;
    margin-top: 4px;
    font-size: 11px;
    color: var(--dim);
  }

  .source a {
    color: oklch(0.72 0.09 190);
    border-bottom-color: oklch(0.72 0.09 190 / 0.3);
  }

  .tier {
    display: inline-block;
    padding: 2px 8px;
    font-size: 11px;
    letter-spacing: 0.06em;
    color: var(--tone, var(--muted));
    border: 1px solid color-mix(in oklch, var(--tone, var(--line-strong)) 45%, transparent);
  }

  .c-vol {
    color: var(--muted);
  }

  .c-vol.volatile {
    color: var(--amber);
  }

  .c-hits {
    color: var(--dim);
  }

  .c-hits.used {
    color: var(--green);
  }

  .c-exp {
    color: var(--muted);
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
