<script lang="ts">
  import { scrapeResource, ApiError, type ScrapeDetail } from '../lib/api'
  import { formatChars, formatDuration, formatTimestamp, truncate } from '../lib/format'

  interface Props {
    id: string
  }

  const { id }: Props = $props()

  type Tab = 'text' | 'clean' | 'raw'
  let tab: Tab = $state('text')

  // The default load omits raw_html, which can be megabytes, so the raw tab has
  // its own resource. Creating a resource issues no request — the ?raw=1 fetch
  // happens only when the raw tab is actually opened.
  let scrape = $derived(scrapeResource(id, false))
  let rawScrape = $derived(scrapeResource(id, true))

  $effect(() => {
    void scrape.reload()
  })

  function show(next: Tab): void {
    tab = next
    if (next === 'raw' && $rawScrape.data === null && !$rawScrape.loading) {
      void rawScrape.reload()
    }
  }

  let notFound = $derived($scrape.error instanceof ApiError && $scrape.error.status === 404)

  function metadata(detail: ScrapeDetail): Array<[string, string]> {
    return [
      ['URL', detail.url],
      ['HTTP status', String(detail.http_status ?? '—')],
      ['Content type', detail.content_type ?? '—'],
      ['Fetched with', detail.fetched_with ?? '—'],
      ['Title', detail.title ?? '—'],
      ['Robots allowed', detail.robots_allowed ? 'yes' : 'no'],
      ['Content hash', detail.content_hash ?? '—'],
      ['ETag', detail.etag ?? '—'],
      ['Last modified', detail.last_modified ?? '—'],
      ['Tier', detail.tier ?? '—'],
      ['Hit count', String(detail.hit_count ?? 0)],
      ['Expires at', detail.expires_at ? formatTimestamp(detail.expires_at) : 'never'],
      ['Fetched at', formatTimestamp(detail.fetched_at)],
      ['Duration', formatDuration(detail.duration_ms)],
      ['Error', detail.error ?? '—'],
    ]
  }
</script>

<section>
  <h1>Scrape</h1>
  <p>{id}</p>

  {#if $scrape.loading && !$scrape.data}
    <p data-testid="scrape-loading">Loading scrape…</p>
  {/if}
  {#if notFound}
    <p data-testid="scrape-missing">No scrape with this id.</p>
  {:else if $scrape.error}
    <p data-testid="scrape-error">Could not load this scrape: {$scrape.error.message}</p>
  {/if}

  {#if $scrape.data}
    {#if $scrape.data.run_id}
      <p><a href="/runs/{$scrape.data.run_id}" data-testid="scrape-run-link">← Run {$scrape.data.run_id}</a></p>
    {/if}

    <h2>Fetch metadata</h2>
    <table data-testid="scrape-metadata">
      <tbody>
        {#each metadata($scrape.data) as [label, value] (label)}
          <tr>
            <th scope="row">{label}</th>
            <td>{value}</td>
          </tr>
        {/each}
      </tbody>
    </table>

    <h2>Content</h2>
    <button type="button" data-testid="tab-text" aria-pressed={tab === 'text'} onclick={() => show('text')}>
      Text
    </button>
    <button type="button" data-testid="tab-clean" aria-pressed={tab === 'clean'} onclick={() => show('clean')}>
      Clean HTML
    </button>
    <button type="button" data-testid="tab-raw" aria-pressed={tab === 'raw'} onclick={() => show('raw')}>
      Raw HTML
    </button>

    {#if tab === 'text'}
      {#if $scrape.data.text}
        <p data-testid="text-size">{formatChars($scrape.data.text.length)} characters</p>
        <pre data-testid="scrape-text">{$scrape.data.text}</pre>
      {:else}
        <p data-testid="text-empty">No text content was extracted.</p>
      {/if}
    {:else if tab === 'clean'}
      {#if $scrape.data.clean_html}
        <!-- Scraped markup is untrusted; sandboxed with no allow-* tokens. -->
        <iframe
          data-testid="clean-frame"
          title="Cleaned HTML for scrape {id}"
          sandbox=""
          referrerpolicy="no-referrer"
          srcdoc={$scrape.data.clean_html}
        ></iframe>
      {:else}
        <p data-testid="clean-empty">No cleaned HTML was stored.</p>
      {/if}
    {:else}
      {#if $rawScrape.loading && !$rawScrape.data}
        <p data-testid="raw-loading">Loading raw HTML…</p>
      {:else if $rawScrape.error}
        <p data-testid="raw-error">{$rawScrape.error.message}</p>
      {:else if $rawScrape.data?.raw_html}
        <p data-testid="raw-size">{formatChars($rawScrape.data.raw_html.length)} characters</p>
        <iframe
          data-testid="raw-frame"
          title="Raw HTML for scrape {id}"
          sandbox=""
          referrerpolicy="no-referrer"
          srcdoc={$rawScrape.data.raw_html}
        ></iframe>
      {:else}
        <p data-testid="raw-empty">No raw HTML was stored.</p>
      {/if}
    {/if}

    <h2>Images</h2>
    {#if !$scrape.data.images || $scrape.data.images.length === 0}
      <p data-testid="images-empty">This scrape recorded no images.</p>
    {:else}
      <ul data-testid="images-list">
        {#each $scrape.data.images as image (image.url)}
          <li>
            <!-- URLs only were stored, so this is a live remote fetch; no
                 referrer is sent and a failure degrades to the alt text. -->
            <img src={image.url} alt={image.alt ?? ''} loading="lazy" referrerpolicy="no-referrer" />
            <a href={image.url} target="_blank" rel="noreferrer noopener">{truncate(image.url, 60)}</a>
            <span>{image.width ?? '?'}×{image.height ?? '?'}</span>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</section>

<style>
  iframe {
    width: 100%;
    height: 50vh;
    border: 1px solid currentColor;
  }
  pre {
    max-height: 50vh;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-word;
  }
  img {
    max-width: 8rem;
    max-height: 6rem;
  }
</style>
