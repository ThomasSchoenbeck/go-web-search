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
  import ProjectionScatter from './views/ProjectionScatter.svelte'

  onMount(() => startRouter())

  // routes.ts is the single source of truth: the nav renders from the same list.
  let route = $derived(resolveRoute($path))
  let params = $derived(new URLSearchParams($search))
  let pivotUrl = $derived(params.get('url') ?? '')
  let exploreQuery = $derived(params.get('q') ?? '')
  let exploreK = $derived(Number(params.get('k')) || 10)
</script>

<header class="shell">
  <a class="brand" href="/runs" data-testid="nav-home">
    <span class="mark" aria-hidden="true"><i></i><i></i><i></i></span>
    <span class="wordmark">Observability UI</span>
  </a>
  <Nav />
  <!-- Read-only by design; the badge says so rather than the docs alone. -->
  <span class="mode"><span class="dot" aria-hidden="true"></span>read-only</span>
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
  {:else if route?.name === 'projection'}
    <ProjectionScatter />
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

<style>
  .shell {
    display: flex;
    align-items: stretch;
    height: 52px;
    background: linear-gradient(var(--panel-2), oklch(0.205 0.014 265));
    border-bottom: 1px solid var(--line);
    position: sticky;
    top: 0;
    z-index: 10;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 20px;
    color: var(--text);
    text-decoration: none;
    border: none;
    border-right: 1px solid var(--line);
  }

  /* Three bars, not a logo: a level meter is what this thing actually is. */
  .mark {
    display: flex;
    align-items: flex-end;
    gap: 2px;
    height: 14px;
  }

  .mark i {
    display: block;
    width: 3px;
  }

  .mark i:nth-child(1) {
    height: 9px;
    background: var(--green);
  }

  .mark i:nth-child(2) {
    height: 14px;
    background: var(--cyan);
  }

  .mark i:nth-child(3) {
    height: 6px;
    background: var(--violet);
  }

  .wordmark {
    font-weight: 600;
    font-size: 12.5px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .mode {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 20px;
    margin-left: auto;
    border-left: 1px solid var(--line);
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--dim);
  }

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--green);
    box-shadow: 0 0 8px var(--green);
  }
</style>
