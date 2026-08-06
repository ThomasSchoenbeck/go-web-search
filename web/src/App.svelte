<script lang="ts">
  import { onMount } from 'svelte'
  import { path, search, startRouter } from './lib/router'
  import { resolveRoute } from './lib/routes'
  import Nav from './components/Nav.svelte'
  import RunsList from './views/RunsList.svelte'
  import RunDetail from './views/RunDetail.svelte'
  import RunCausality from './views/RunCausality.svelte'
  import SearchesList from './views/SearchesList.svelte'
  import SerpViewer from './views/SerpViewer.svelte'
  import ScrapeDetail from './views/ScrapeDetail.svelte'
  import Provenance from './views/Provenance.svelte'
  import FactsBrowser from './views/FactsBrowser.svelte'
  import FactDetail from './views/FactDetail.svelte'
  import SemanticExplorer from './views/SemanticExplorer.svelte'
  import JobsMonitor from './views/JobsMonitor.svelte'
  import SearchCacheBrowser from './views/SearchCacheBrowser.svelte'
  import ScrapeCacheBrowser from './views/ScrapeCacheBrowser.svelte'
  import LogsViewer from './views/LogsViewer.svelte'
  import StatsDashboard from './views/StatsDashboard.svelte'

  onMount(() => startRouter())

  // routes.ts is the single source of truth: the nav renders from the same list.
  let route = $derived(resolveRoute($path))
  let params = $derived(new URLSearchParams($search))
  let pivotUrl = $derived(params.get('url') ?? '')
  let exploreQuery = $derived(params.get('q') ?? '')
  let exploreK = $derived(Number(params.get('k')) || 10)
</script>

<header>
  <a href="/runs" data-testid="nav-home">Observability UI</a>
  <Nav />
</header>

<main>
  {#if route?.name === 'run-searches'}
    <SearchesList runId={route.params.id} />
  {:else if route?.name === 'run-causality'}
    <RunCausality runId={route.params.id} />
  {:else if route?.name === 'run-detail'}
    <RunDetail id={route.params.id} />
  {:else if route?.name === 'serp'}
    <SerpViewer id={route.params.id} />
  {:else if route?.name === 'scrape'}
    <ScrapeDetail id={route.params.id} />
  {:else if route?.name === 'provenance'}
    <Provenance url={pivotUrl} />
  {:else if route?.name === 'fact-detail'}
    <FactDetail id={route.params.id} />
  {:else if route?.name === 'facts'}
    <FactsBrowser />
  {:else if route?.name === 'explore'}
    <SemanticExplorer query={exploreQuery} k={exploreK} />
  {:else if route?.name === 'jobs'}
    <JobsMonitor />
  {:else if route?.name === 'search-cache'}
    <SearchCacheBrowser />
  {:else if route?.name === 'scrape-cache'}
    <ScrapeCacheBrowser />
  {:else if route?.name === 'logs'}
    <LogsViewer />
  {:else if route?.name === 'stats'}
    <StatsDashboard />
  {:else if route?.name === 'runs'}
    <RunsList />
  {:else}
    <section data-testid="not-found">
      <h1>Not found</h1>
      <p>No view is registered for {$path}.</p>
      <p><a href="/runs" data-testid="not-found-home">Back to runs</a></p>
    </section>
  {/if}
</main>
