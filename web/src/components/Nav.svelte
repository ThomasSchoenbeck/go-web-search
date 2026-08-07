<script lang="ts">
  import { navEntries, activeSection } from '../lib/routes'
  import { path } from '../lib/router'

  // Plain anchors: the router intercepts unmodified same-origin clicks, so
  // middle-click and open-in-new-tab keep working.
  let active = $derived(activeSection($path))
</script>

<nav aria-label="Views">
  <ul>
    {#each navEntries as entry (entry.section)}
      <li>
        <a
          href={entry.href}
          data-testid="nav-{entry.section}"
          aria-current={active === entry.section ? 'page' : undefined}
          class:active={active === entry.section}
        >
          {entry.label}
        </a>
      </li>
    {/each}
  </ul>
</nav>

<style>
  ul {
    display: flex;
    gap: 1rem;
    list-style: none;
    padding: 0;
    margin: 0;
  }
  a.active {
    font-weight: bold;
    text-decoration: underline;
  }
</style>
