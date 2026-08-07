<script lang="ts">
  import { runCausalityResource } from '../lib/api'
  import { buildTree, countNodes } from '../lib/graph'
  import { truncate } from '../lib/format'

  interface Props {
    runId: string
  }

  const { runId }: Props = $props()

  let graph = $derived(runCausalityResource(runId))

  $effect(() => {
    void graph.reload()
  })

  let tree = $derived($graph.data ? buildTree($graph.data) : null)
  let counts = $derived($graph.data ? countNodes($graph.data) : null)
</script>

<section>
  <p><a href="/runs/{runId}" data-testid="back-to-run">← Run {runId}</a></p>
  <h1>Causality</h1>

  {#if $graph.loading && !$graph.data}
    <p data-testid="causality-loading">Assembling the graph…</p>
  {/if}
  {#if $graph.error}
    <p data-testid="causality-error">Could not load the graph: {$graph.error.message}</p>
  {/if}

  {#if $graph.data && counts}
    <p data-testid="causality-counts">
      {counts.searches} searches · {counts.urls} URLs · {counts.scrapes} scrapes · {counts.facts} facts
    </p>

    {#if $graph.data.truncated}
      <p data-testid="causality-truncated">
        Showing the first {$graph.data.limit} URLs of this run. Raise
        <code>observability.causality_max_urls</code> in config.toml to widen the graph.
      </p>
    {/if}

    {#if !$graph.data.vectors_available}
      <p data-testid="causality-vectors-unavailable">
        {$graph.data.note || 'Vector store unavailable; vector presence is not reported.'}
      </p>
    {/if}

    {#if counts.searches === 0 && counts.urls === 0}
      <p data-testid="causality-empty">This run produced no searches or URLs.</p>
    {/if}
  {/if}

  {#if tree}
    {#each tree.searches as branch (branch.search.id)}
      <article data-testid="causality-search">
        <h2>
          <a href="/searches/{branch.search.ref_id}" data-testid="causality-search-link">
            {branch.search.label}
          </a>
          <small>{branch.search.detail ?? ''}</small>
        </h2>

        {#if branch.urls.length === 0}
          <p data-testid="causality-search-empty">This search found no URLs.</p>
        {:else}
          <ul>
            {#each branch.urls as urlBranch (urlBranch.url.id)}
              <li data-testid="causality-url">
                <span data-testid="causality-rank">#{urlBranch.rank ?? '—'}</span>
                <a
                  href="/provenance?url={encodeURIComponent(urlBranch.url.url ?? '')}"
                  data-testid="causality-url-link"
                >
                  {truncate(urlBranch.url.label, 70)}
                </a>

                {#if urlBranch.scrapes.length === 0}
                  <span data-testid="causality-no-scrape">not scraped</span>
                {:else}
                  <ul>
                    {#each urlBranch.scrapes as scrape (scrape.id)}
                      <li>
                        <a href="/scrapes/{scrape.ref_id}" data-testid="causality-scrape-link">
                          {truncate(scrape.label, 60)}
                        </a>
                        <small>{scrape.detail ?? ''}</small>
                      </li>
                    {/each}
                  </ul>
                {/if}

                {#if urlBranch.facts.length > 0}
                  <ul>
                    {#each urlBranch.facts as fact (fact.id)}
                      <li>
                        <a href="/facts/{fact.ref_id}" data-testid="causality-fact-link">
                          {truncate(fact.label, 70)}
                        </a>
                        {#if $graph.data?.vectors_available}
                          <span data-testid="causality-fact-vector">
                            {fact.has_vector ? 'embedded' : 'not embedded'}
                          </span>
                        {/if}
                      </li>
                    {/each}
                  </ul>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      </article>
    {/each}

    {#if tree.orphanUrls.length > 0}
      <article data-testid="causality-orphans">
        <h2>URLs with no finding search</h2>
        <ul>
          {#each tree.orphanUrls as urlBranch (urlBranch.url.id)}
            <li>
              <a
                href="/provenance?url={encodeURIComponent(urlBranch.url.url ?? '')}"
                data-testid="causality-orphan-link"
              >
                {truncate(urlBranch.url.label, 70)}
              </a>
            </li>
          {/each}
        </ul>
      </article>
    {/if}
  {/if}
</section>
