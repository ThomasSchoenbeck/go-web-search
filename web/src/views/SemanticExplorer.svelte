<script lang="ts">
  import { exploreResource } from '../lib/api'
  import { navigate } from '../lib/router'
  import { truncate } from '../lib/format'

  interface Props {
    /** Query text from ?q=, so a probe can be linked to. */
    query: string
    k: number
  }

  const { query, k }: Props = $props()

  const kChoices = [5, 10, 25, 50]

  let draft = $state('')
  let draftK = $state(10)

  let result = $derived(query ? exploreResource(query, k) : null)

  $effect(() => {
    void result?.reload()
  })

  $effect(() => {
    draft = query
    draftK = k
  })

  function submit(event: SubmitEvent): void {
    event.preventDefault()
    const text = draft.trim()
    if (!text) {
      navigate('/explore')
      return
    }
    navigate(`/explore?q=${encodeURIComponent(text)}&k=${draftK}`)
  }

  /** Cosine distance is small-is-near; show enough digits to rank by eye. */
  function formatDistance(value: number): string {
    return value.toFixed(4)
  }
</script>

<section>
  <h1>Semantic explorer</h1>
  <p>
    A raw nearest-neighbour probe over the vector store. It embeds your text and
    reports what sits closest — no confidence gating and no synthesized answer,
    unlike a memory query.
  </p>

  <form onsubmit={submit}>
    <label>
      Text
      <input type="text" data-testid="explore-input" placeholder="anything at all" bind:value={draft} size="50" />
    </label>
    <label>
      Neighbours
      <select data-testid="explore-k" bind:value={draftK}>
        {#each kChoices as choice (choice)}
          <option value={choice}>{choice}</option>
        {/each}
      </select>
    </label>
    <button type="submit" data-testid="explore-submit">Search</button>
  </form>

  {#if !query}
    <p data-testid="explore-prompt">Enter some text to find its nearest neighbours.</p>
  {:else}
    {#if $result?.loading && !$result?.data}
      <p data-testid="explore-loading">Embedding and searching…</p>
    {/if}
    {#if $result?.error}
      <p data-testid="explore-error">Could not run the search: {$result.error.message}</p>
    {/if}

    {#if $result?.data}
      {@const data = $result.data}
      {#if !data.available}
        <p data-testid="explore-unavailable">
          {data.note || 'The vector store is unavailable.'}
        </p>
      {:else if data.neighbors.length === 0}
        <p data-testid="explore-empty">Nothing is stored near “{truncate(query, 60)}”.</p>
      {:else}
        <p data-testid="explore-summary">
          {data.neighbors.length} nearest of {data.memory_hits} facts and
          {data.search_hits} cached searches considered.
        </p>
        <table data-testid="explore-results">
          <thead>
            <tr>
              <th>Distance</th>
              <th>Similarity</th>
              <th>Kind</th>
              <th>Text</th>
              <th>Links</th>
            </tr>
          </thead>
          <tbody>
            {#each data.neighbors as neighbor (neighbor.owner_kind + neighbor.id)}
              <tr data-testid="explore-row">
                <td data-testid="explore-distance">{formatDistance(neighbor.distance)}</td>
                <td>{formatDistance(neighbor.similarity)}</td>
                <td data-testid="explore-kind">{neighbor.owner_kind === 'memory' ? 'fact' : 'cached search'}</td>
                <td>{truncate(neighbor.text, 90)}</td>
                <td>
                  {#if neighbor.owner_kind === 'memory'}
                    <a href="/facts/{neighbor.id}" data-testid="explore-fact-link">fact</a>
                    {#if neighbor.source_url}
                      ·
                      <a
                        href="/provenance?url={encodeURIComponent(neighbor.source_url)}"
                        data-testid="explore-source-link"
                      >
                        source
                      </a>
                    {/if}
                  {:else}
                    <span data-testid="explore-search-context">
                      {neighbor.result_count ?? 0} results · {neighbor.tier ?? '—'}
                    </span>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    {/if}
  {/if}
</section>
