<script lang="ts">
  import { searchRawResource, ApiError } from '../lib/api'
  import { formatChars } from '../lib/format'

  interface Props {
    id: string
  }

  const { id }: Props = $props()

  type View = 'rendered' | 'source'
  let view: View = $state('rendered')

  let serp = $derived(searchRawResource(id))

  $effect(() => {
    void serp.reload()
  })

  // A search that stored no SERP is an expected state, not a failure: the
  // endpoint 404s and the view says so rather than showing an error.
  let notFound = $derived($serp.error instanceof ApiError && $serp.error.status === 404)
</script>

<section>
  <h1>Raw SERP</h1>
  <p>Search {id}</p>

  {#if $serp.loading && !$serp.data}
    <p data-testid="serp-loading">Loading SERP…</p>
  {/if}

  {#if notFound}
    <p data-testid="serp-missing">No raw HTML was stored for this search.</p>
  {:else if $serp.error}
    <p data-testid="serp-error">Could not load the SERP: {$serp.error.message}</p>
  {/if}

  {#if $serp.data}
    <p data-testid="serp-size">{formatChars($serp.data.length)} characters</p>

    <button
      type="button"
      data-testid="serp-view-rendered"
      aria-pressed={view === 'rendered'}
      onclick={() => (view = 'rendered')}
    >
      Rendered
    </button>
    <button
      type="button"
      data-testid="serp-view-source"
      aria-pressed={view === 'source'}
      onclick={() => (view = 'source')}
    >
      Source
    </button>

    {#if view === 'rendered'}
      <!--
        A stored SERP is a full untrusted document. srcdoc inside a sandbox with
        no allow-* tokens gives it a unique opaque origin with scripts, forms and
        top-level navigation all disabled, so it can render without reaching the
        app DOM or the API it is served next to.
      -->
      <iframe
        data-testid="serp-frame"
        title="Raw SERP for search {id}"
        sandbox=""
        referrerpolicy="no-referrer"
        srcdoc={$serp.data}
      ></iframe>
    {:else}
      <pre data-testid="serp-source">{$serp.data}</pre>
    {/if}
  {/if}
</section>

<style>
  iframe {
    width: 100%;
    height: 60vh;
    border: 1px solid currentColor;
  }
  pre {
    max-height: 60vh;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-word;
  }
</style>
