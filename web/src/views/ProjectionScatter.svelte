<script lang="ts">
  /**
   * The embedding space, flattened to two dimensions.
   *
   * The server sends raw vectors (T021) and the layout is computed here with
   * PCA (lib/projection.ts). PCA is deterministic, so the same store always
   * draws the same picture — a point that moves means the data moved.
   *
   * Distances are indicative, not exact: two dimensions cannot hold everything
   * a 4096-dimensional space knows. The view says so rather than letting a
   * viewer read precision into it.
   */
  import { onMount } from 'svelte'
  import { projectionResource, type ProjectionDump, type ProjectionPoint } from '../lib/api'
  import { projectTo2D, scaleToBox, type Point2D } from '../lib/projection'
  import { loadUIConfig } from '../lib/uiconfig'
  import { truncate } from '../lib/format'

  const width = 640
  const height = 420
  const margin = 24

  // The cap lives in config.toml and reaches the SPA through /api/ui-config, so
  // how much of the space this view may pull is deployment policy, not a
  // constant compiled in here.
  let cap = $state(0)
  let capError = $state('')
  let dump = $derived(cap > 0 ? projectionResource(cap) : null)

  let projecting = $state(false)
  let placed = $state<Array<{ point: ProjectionPoint; at: Point2D }>>([])
  let selectedId = $state('')

  onMount(() => {
    loadUIConfig()
      .then((config) => (cap = config.projectionSampleCap))
      .catch((cause: unknown) => (capError = cause instanceof Error ? cause.message : String(cause)))
  })

  $effect(() => {
    void dump?.reload()
  })

  // Laying out thousands of high-dimensional vectors is real work, so the
  // "projecting" state is painted before it starts rather than after.
  $effect(() => {
    const data = $dump?.data
    if (!data) return
    projecting = true
    selectedId = ''
    const timer = setTimeout(() => {
      placed = layout(data)
      projecting = false
    }, 0)
    return () => clearTimeout(timer)
  })

  function layout(data: ProjectionDump): Array<{ point: ProjectionPoint; at: Point2D }> {
    const coords = scaleToBox(
      projectTo2D(data.points.map((point) => point.vector)),
      width,
      height,
      margin,
    )
    return data.points.map((point, i) => ({ point, at: coords[i] }))
  }

  function select(id: string): void {
    selectedId = selectedId === id ? '' : id
  }

  function onPointKey(event: KeyboardEvent, id: string): void {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      select(id)
    }
  }

  let selected = $derived(placed.find((entry) => entry.point.id === selectedId)?.point ?? null)
  let shown = $derived($dump?.data?.points.length ?? 0)
  let stored = $derived(
    Object.values($dump?.data?.total ?? {}).reduce((sum, count) => sum + count, 0),
  )
</script>

<section>
  <h1>Embedding projection</h1>

  {#if capError}
    <p data-testid="projection-config-error">Could not read the sample cap: {capError}</p>
  {/if}

  {#if $dump?.loading && !$dump?.data}
    <p data-testid="projection-loading">Loading vectors…</p>
  {/if}
  {#if $dump?.error}
    <p data-testid="projection-error">Could not load vectors: {$dump.error.message}</p>
  {/if}

  {#if $dump?.data}
    {#if !$dump.data.available}
      <p data-testid="projection-unavailable">{$dump.data.note}</p>
    {:else if $dump.data.points.length === 0}
      <p data-testid="projection-empty">No vectors have been stored yet, so there is nothing to plot.</p>
    {:else}
      <p data-testid="projection-summary">
        Showing {shown} of {stored} vectors{$dump.data.truncated ? ` (capped at ${$dump.data.limit})` : ''},
        {$dump.data.dim} dimensions reduced to 2 by PCA. Distances are indicative, not exact.
      </p>

      {#if projecting}
        <p data-testid="projection-computing">Projecting {shown} vectors…</p>
      {:else}
        <svg
          data-testid="projection-plot"
          viewBox="0 0 {width} {height}"
          width="100%"
          role="group"
          aria-label="Embedding vectors projected to two dimensions"
        >
          {#each placed as entry (entry.point.id)}
            <circle
              data-testid="projection-point"
              data-kind={entry.point.owner_kind}
              class="point {entry.point.owner_kind}"
              class:selected={entry.point.id === selectedId}
              cx={entry.at.x}
              cy={entry.at.y}
              r={entry.point.id === selectedId ? 9 : 6}
              role="button"
              tabindex="0"
              aria-label="{entry.point.owner_kind}: {truncate(entry.point.label, 60)}"
              onclick={() => select(entry.point.id)}
              onkeydown={(event) => onPointKey(event, entry.point.id)}
            >
              <title>{entry.point.label}</title>
            </circle>
          {/each}
        </svg>

        <ul data-testid="projection-legend">
          <li><span class="swatch memory"></span> memory fact</li>
          <li><span class="swatch search"></span> cached search</li>
        </ul>
      {/if}

      {#if selected}
        <aside data-testid="projection-selection">
          <p data-testid="projection-selected-kind">
            {selected.owner_kind === 'memory' ? 'Memory fact' : 'Cached search'}
          </p>
          <p data-testid="projection-selected-label">{selected.label}</p>
          {#if selected.owner_kind === 'memory'}
            <a href="/facts/{selected.id}" data-testid="projection-fact-link">Open the fact</a>
            {#if selected.source_url}
              <a
                href="/provenance?url={encodeURIComponent(selected.source_url)}"
                data-testid="projection-source-link"
              >
                Trace its source
              </a>
            {/if}
          {/if}
          <a href="/explore?q={encodeURIComponent(selected.label)}&k=10" data-testid="projection-explore-link">
            Find its neighbours
          </a>
          <button type="button" data-testid="projection-clear" onclick={() => (selectedId = '')}>
            Clear selection
          </button>
        </aside>
      {:else}
        <p data-testid="projection-prompt">Select a point to see what it is.</p>
      {/if}
    {/if}
  {/if}
</section>

<style>
  .point {
    cursor: pointer;
    stroke: #ffffff;
    stroke-width: 1;
  }
  /* Owner kind is the one thing the plot must encode, so it gets the colour. */
  .memory {
    fill: #1f6feb;
  }
  .search {
    fill: #b8860b;
  }
  .point.selected {
    stroke: #000000;
    stroke-width: 2;
  }
  ul {
    display: flex;
    gap: 1rem;
    list-style: none;
    padding: 0;
  }
  .swatch {
    display: inline-block;
    width: 0.75rem;
    height: 0.75rem;
    border-radius: 50%;
  }
  .swatch.memory {
    background: #1f6feb;
  }
  .swatch.search {
    background: #b8860b;
  }
  aside a {
    margin-right: 1rem;
  }
</style>
