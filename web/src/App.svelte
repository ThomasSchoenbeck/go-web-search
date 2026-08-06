<script lang="ts">
  import { onMount } from 'svelte'
  import { path, startRouter, matchRoute } from './lib/router'
  import RunsList from './views/RunsList.svelte'
  import RunDetail from './views/RunDetail.svelte'
  import SearchesList from './views/SearchesList.svelte'
  import SerpViewer from './views/SerpViewer.svelte'
  import ScrapeDetail from './views/ScrapeDetail.svelte'

  onMount(() => startRouter())

  // First match wins, so the more specific patterns come first.
  let runSearches = $derived(matchRoute('/runs/:id/searches', $path))
  let runDetail = $derived(matchRoute('/runs/:id', $path))
  let serp = $derived(matchRoute('/searches/:id', $path))
  let scrape = $derived(matchRoute('/scrapes/:id', $path))
  let runsList = $derived(matchRoute('/runs', $path) ?? matchRoute('/', $path))
</script>

<header>
  <a href="/runs" data-testid="nav-runs">Observability UI</a>
</header>

<main>
  {#if runSearches}
    <SearchesList runId={runSearches.id} />
  {:else if runDetail}
    <RunDetail id={runDetail.id} />
  {:else if serp}
    <SerpViewer id={serp.id} />
  {:else if scrape}
    <ScrapeDetail id={scrape.id} />
  {:else if runsList}
    <RunsList />
  {:else}
    <section data-testid="not-found">
      <h1>Not found</h1>
      <p>No view is registered for {$path}.</p>
      <p><a href="/runs" data-testid="not-found-home">Back to runs</a></p>
    </section>
  {/if}
</main>
