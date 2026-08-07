<script lang="ts">
  import { runsResource } from '../lib/api'
  import { formatTimestamp } from '../lib/format'

  const limitChoices = [10, 25, 50, 100]

  let limit = $state(25)

  // Rebuilt when the limit changes; the effect then fetches with the new value.
  let runs = $derived(runsResource(limit))

  $effect(() => {
    void runs.reload()
  })

  function changeLimit(event: Event): void {
    limit = Number((event.currentTarget as HTMLSelectElement).value)
  }
</script>

<section>
  <h1>Runs</h1>

  <label>
    Show
    <select data-testid="run-limit" value={limit} onchange={changeLimit}>
      {#each limitChoices as choice (choice)}
        <option value={choice}>{choice}</option>
      {/each}
    </select>
  </label>
  <button type="button" data-testid="runs-reload" onclick={() => runs.reload()}>Reload</button>

  {#if $runs.loading && !$runs.data}
    <p data-testid="runs-loading">Loading runs…</p>
  {/if}
  {#if $runs.error}
    <p data-testid="runs-error">Could not load runs: {$runs.error.message}</p>
  {/if}

  {#if $runs.data}
    {#if $runs.data.length === 0}
      <p data-testid="runs-empty">No runs recorded yet.</p>
    {:else}
      <table data-testid="runs-table">
        <thead>
          <tr>
            <th>Run</th>
            <th>Mode</th>
            <th>Started</th>
            <th>Finished</th>
            <th>Searches</th>
            <th>URLs</th>
            <th>Scrapes</th>
          </tr>
        </thead>
        <tbody>
          {#each $runs.data as run (run.id)}
            <tr>
              <td><a href="/runs/{run.id}" data-testid="run-link">{run.id}</a></td>
              <td>{run.mode}</td>
              <td>{formatTimestamp(run.started_at)}</td>
              <td>{run.finished_at ? formatTimestamp(run.finished_at) : 'running'}</td>
              <td>{run.searches}</td>
              <td>{run.urls}</td>
              <td>{run.scrapes}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  {/if}
</section>
