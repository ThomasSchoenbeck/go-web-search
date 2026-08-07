/**
 * Turns the flat nodes/edges of a run causality graph into the tree the view
 * renders: search → urls (with rank) → that url's scrape and facts.
 *
 * The payload is flat and deduplicated — a URL found by two searches is one node
 * with two incoming edges — so the tree deliberately repeats such a URL under
 * each search that found it. That is the honest rendering: the same page really
 * was found twice, at two ranks.
 *
 * Pure functions, no DOM and no layout library.
 */

import type { CausalityGraph, CausalityNode } from './api'

export interface UrlBranch {
  url: CausalityNode
  /** Rank this search gave the URL. */
  rank?: number
  scrapes: CausalityNode[]
  facts: CausalityNode[]
}

export interface SearchBranch {
  search: CausalityNode
  urls: UrlBranch[]
}

export interface CausalityTree {
  searches: SearchBranch[]
  /** Searches that found nothing still appear, with an empty url list. */
  orphanUrls: UrlBranch[]
}

function byId(graph: CausalityGraph): Map<string, CausalityNode> {
  return new Map(graph.nodes.map((node) => [node.id, node]))
}

export function buildTree(graph: CausalityGraph): CausalityTree {
  const nodes = byId(graph)

  // Children of each url node, split by kind.
  const scrapesOf = new Map<string, CausalityNode[]>()
  const factsOf = new Map<string, CausalityNode[]>()
  // url -> [{searchId, rank}]
  const urlsOf = new Map<string, Array<{ urlId: string; rank?: number }>>()
  const attachedUrls = new Set<string>()

  for (const edge of graph.edges) {
    const from = nodes.get(edge.from)
    const to = nodes.get(edge.to)
    // A dangling edge is a backend bug; skip it rather than render a blank row.
    if (!from || !to) continue

    if (from.kind === 'search' && to.kind === 'url') {
      const list = urlsOf.get(from.id) ?? []
      list.push({ urlId: to.id, rank: edge.rank })
      urlsOf.set(from.id, list)
      attachedUrls.add(to.id)
      continue
    }
    if (from.kind === 'url' && to.kind === 'scrape') {
      const list = scrapesOf.get(from.id) ?? []
      list.push(to)
      scrapesOf.set(from.id, list)
      continue
    }
    if (from.kind === 'url' && to.kind === 'fact') {
      const list = factsOf.get(from.id) ?? []
      list.push(to)
      factsOf.set(from.id, list)
    }
  }

  const branchFor = (urlId: string, rank?: number): UrlBranch | null => {
    const url = nodes.get(urlId)
    if (!url) return null
    return { url, rank, scrapes: scrapesOf.get(urlId) ?? [], facts: factsOf.get(urlId) ?? [] }
  }

  const searches: SearchBranch[] = graph.nodes
    .filter((node) => node.kind === 'search')
    .map((search) => ({
      search,
      urls: (urlsOf.get(search.id) ?? [])
        .map(({ urlId, rank }) => branchFor(urlId, rank))
        .filter((branch): branch is UrlBranch => branch !== null)
        .sort((a, b) => (a.rank ?? Infinity) - (b.rank ?? Infinity)),
    }))

  // A URL can be scraped without any search in this run pointing at it; showing
  // it separately beats dropping it.
  const orphanUrls = graph.nodes
    .filter((node) => node.kind === 'url' && !attachedUrls.has(node.id))
    .map((node) => branchFor(node.id))
    .filter((branch): branch is UrlBranch => branch !== null)

  return { searches, orphanUrls }
}

export interface GraphCounts {
  searches: number
  urls: number
  scrapes: number
  facts: number
}

/** Distinct node counts by kind, for the view's summary line. */
export function countNodes(graph: CausalityGraph): GraphCounts {
  const counts: GraphCounts = { searches: 0, urls: 0, scrapes: 0, facts: 0 }
  for (const node of graph.nodes) {
    if (node.kind === 'search') counts.searches += 1
    else if (node.kind === 'url') counts.urls += 1
    else if (node.kind === 'scrape') counts.scrapes += 1
    else if (node.kind === 'fact') counts.facts += 1
  }
  return counts
}
