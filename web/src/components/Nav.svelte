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
  nav {
    display: flex;
  }

  ul {
    display: flex;
    gap: 0;
    list-style: none;
    padding: 0;
    margin: 0;
  }

  li {
    display: flex;
  }

  a {
    display: flex;
    align-items: center;
    padding: 0 14px;
    color: var(--dim);
    text-decoration: none;
    border: none;
    font-size: 11.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    /* Reserve the indicator so the row does not shift on activation. */
    box-shadow: inset 0 -2px 0 transparent;
    transition:
      color 0.12s ease,
      background 0.12s ease;
  }

  a:hover {
    color: var(--text);
    background: oklch(1 0 0 / 0.04);
  }

  a.active {
    color: var(--text);
    background: var(--panel-3);
    box-shadow: inset 0 -2px 0 var(--green);
  }
</style>
