<script lang="ts">
  import { provenanceResource } from '../lib/api'
  import { navigate } from '../lib/router'
  import { formatChars, formatTimestamp, truncate } from '../lib/format'

  interface Props {
    /** The URL to pivot on, from the ?url= query parameter. */
    url: string
  }

  const { url }: Props = $props()

  // Seeded from the prop by the effect below, so the box reflects the pivot in
  // the URL on load and whenever it changes.
  let draft = $state('')

  // Rebuilt when the query parameter changes, so a new pivot refetches.
  let chain = $derived(url ? provenanceResource(url) : null)

  $effect(() => {
    void chain?.reload()
  })

  $effect(() => {
    draft = url
  })

  function submit(event: SubmitEvent): void {
    event.preventDefault()
    const next = draft.trim()
    // The pivot lives in the URL so a chain can be linked to and reloaded.
    navigate(next ? `/provenance?url=${encodeURIComponent(next)}` : '/provenance')
  }
</script>

<section>
  <h1>Provenance</h1>
  <p>Pivot on a URL to see which searches found it and what came out of it.</p>

  <form onsubmit={submit}>
    <label>
      URL
      <input
        type="url"
        name="url"
        data-testid="provenance-input"
        placeholder="https://example.com/page"
        bind:value={draft}
        size="60"
      />
    </label>
    <button type="submit" data-testid="provenance-submit">Trace</button>
  </form>

  {#if !url}
    <p data-testid="provenance-prompt">Enter a URL above, or follow a URL link from a run.</p>
  {:else}
    {#if $chain?.loading && !$chain?.data}
      <p data-testid="provenance-loading">Tracing {truncate(url)}…</p>
    {/if}
    {#if $chain?.error}
      <p data-testid="provenance-error">Could not trace this URL: {$chain.error.message}</p>
    {/if}

    {#if $chain?.data}
      {@const data = $chain.data}
      <h2>{truncate(data.url)}</h2>

      {#if !data.known}
        <p data-testid="provenance-unknown">
          This URL is not in the registry and has no cached scrape. Nothing points at it yet.
        </p>
      {/if}

      <h3>Backward — searches that found it</h3>
      {#if data.found_by.length === 0}
        <p data-testid="found-by-empty">No search in the database returned this URL.</p>
      {:else}
        <table data-testid="found-by-table">
          <thead>
            <tr><th>Rank</th><th>Engine</th><th>Term</th><th>Mode</th><th>Run</th><th>Search</th></tr>
          </thead>
          <tbody>
            {#each data.found_by as found (found.search_id)}
              <tr>
                <td>{found.rank}</td>
                <td>{found.engine}</td>
                <td>{found.term}</td>
                <td>{found.search_mode}</td>
                <td><a href="/runs/{found.run_id}" data-testid="found-run-link">{found.run_id}</a></td>
                <td><a href="/searches/{found.search_id}" data-testid="found-search-link">SERP</a></td>
              </tr>
            {/each}
          </tbody>
        </table>
        <p>
          <a href="/runs/{data.found_by[0].run_id}/causality" data-testid="causality-link">
            See the whole run's causality graph →
          </a>
        </p>
      {/if}

      <h3>Forward — what came out of it</h3>
      {#if !data.scrape}
        <p data-testid="scrape-none">This URL has no cached scrape.</p>
      {:else}
        <dl data-testid="provenance-scrape">
          <dt>Scrape</dt>
          <dd>
            <a href="/scrapes/{data.scrape.scrape_id}" data-testid="provenance-scrape-link">
              {data.scrape.title || data.scrape.scrape_id}
            </a>
          </dd>
          <dt>Status</dt>
          <dd>{data.scrape.http_status ?? '—'} · {data.scrape.fetched_with ?? '—'}</dd>
          <dt>Sizes</dt>
          <dd>
            {formatChars(data.scrape.text_chars)} text ·
            {formatChars(data.scrape.clean_html_chars)} clean ·
            {formatChars(data.scrape.raw_html_chars)} raw
          </dd>
          <dt>Scraped</dt>
          <dd>{formatTimestamp(data.scrape.created_at)}</dd>
        </dl>
      {/if}

      {#if !data.vectors_available}
        <p data-testid="vectors-unavailable">
          {data.note || 'Vector store unavailable; vector presence is not reported.'}
        </p>
      {/if}

      {#if data.facts.length === 0}
        <p data-testid="facts-empty">Nothing has been distilled from this URL.</p>
      {:else}
        <ul data-testid="facts-list">
          {#each data.facts as fact (fact.id)}
            <li>
              <a href="/facts/{fact.id}" data-testid="fact-link">{fact.text}</a>
              <span>{fact.tier ?? '—'} · {fact.volatility || 'unlabelled'}</span>
              {#if data.vectors_available}
                <span data-testid="fact-vector">{fact.has_vector ? 'embedded' : 'not embedded'}</span>
              {:else}
                <span data-testid="fact-vector-unknown">vector unknown</span>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  {/if}
</section>
