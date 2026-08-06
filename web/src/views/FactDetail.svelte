<script lang="ts">
  import { factResource, ApiError } from '../lib/api'
  import { formatChars, formatTimestamp } from '../lib/format'

  interface Props {
    id: string
  }

  const { id }: Props = $props()

  let detail = $derived(factResource(id))

  $effect(() => {
    void detail.reload()
  })

  let notFound = $derived($detail.error instanceof ApiError && $detail.error.status === 404)
</script>

<section>
  <p><a href="/facts" data-testid="back-to-facts">← All facts</a></p>
  <h1>Fact</h1>

  {#if $detail.loading && !$detail.data}
    <p data-testid="fact-loading">Loading fact…</p>
  {/if}
  {#if notFound}
    <p data-testid="fact-missing">No fact with this id.</p>
  {:else if $detail.error}
    <p data-testid="fact-error">Could not load this fact: {$detail.error.message}</p>
  {/if}

  {#if $detail.data}
    {@const fact = $detail.data.fact}
    <blockquote data-testid="fact-text">{fact.text}</blockquote>

    <table data-testid="fact-metadata">
      <tbody>
        <tr><th scope="row">Length</th><td>{formatChars(fact.text_chars)} characters</td></tr>
        <tr><th scope="row">Volatility</th><td>{fact.volatility || '—'}</td></tr>
        <tr><th scope="row">Tier</th><td>{fact.tier ?? '—'}</td></tr>
        <tr><th scope="row">Hit count</th><td>{fact.hit_count}</td></tr>
        <tr><th scope="row">Expires</th><td>{fact.expires_at ? formatTimestamp(fact.expires_at) : 'never'}</td></tr>
        <tr>
          <th scope="row">Source</th>
          <td>
            {#if fact.source_url}
              <!-- The reverse fact→sources path: pivot on the URL to see which
                   searches surfaced the page this fact came from. -->
              <a
                href="/provenance?url={encodeURIComponent(fact.source_url)}"
                data-testid="fact-provenance-link"
              >
                {fact.source_url}
              </a>
            {:else}
              —
            {/if}
          </td>
        </tr>
      </tbody>
    </table>

    <h2>Source material</h2>
    {#if $detail.data.source}
      {@const source = $detail.data.source}
      <dl data-testid="fact-source">
        <dt>Scrape</dt>
        <dd>
          <a href="/scrapes/{source.scrape_id}" data-testid="fact-scrape-link">
            {source.title || source.scrape_id}
          </a>
        </dd>
        <dt>Status</dt>
        <dd>{source.http_status ?? '—'} · {source.fetched_with ?? '—'}</dd>
        <dt>Sizes</dt>
        <dd>
          {formatChars(source.text_chars)} text ·
          {formatChars(source.clean_html_chars)} clean ·
          {formatChars(source.raw_html_chars)} raw
        </dd>
        <dt>Scraped</dt>
        <dd>{formatTimestamp(source.created_at)}</dd>
      </dl>
      {#if $detail.data.read_raw}
        <p>
          <!-- An API path, not a client route: it streams the stored HTML. -->
          <a href={$detail.data.read_raw} data-testid="fact-read-raw" target="_blank" rel="noreferrer noopener">
            Read the raw source page →
          </a>
        </p>
      {/if}
    {:else}
      <p data-testid="fact-source-missing">
        {$detail.data.note || 'No cached source page for this fact.'}
      </p>
    {/if}
  {/if}
</section>
