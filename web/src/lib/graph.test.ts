import { describe, it, expect } from 'vitest'
import { buildTree, countNodes } from './graph'
import type { CausalityGraph } from './api'

function graph(partial: Partial<CausalityGraph> = {}): CausalityGraph {
  return {
    run_id: 'run-1',
    nodes: [],
    edges: [],
    truncated: false,
    limit: 200,
    vectors_available: true,
    ...partial,
  }
}

const full = graph({
  nodes: [
    { id: 'search:s1', kind: 'search', ref_id: 's1', label: 'google · term', detail: 'typed' },
    { id: 'search:s2', kind: 'search', ref_id: 's2', label: 'bing · term', detail: 'direct' },
    { id: 'url:u1', kind: 'url', ref_id: 'u1', label: 'https://a.example', url: 'https://a.example' },
    { id: 'url:u2', kind: 'url', ref_id: 'u2', label: 'https://b.example', url: 'https://b.example' },
    { id: 'scrape:c1', kind: 'scrape', ref_id: 'c1', label: 'A page', detail: 'HTTP 200', url: 'https://a.example' },
    { id: 'fact:f1', kind: 'fact', ref_id: 'f1', label: 'A fact', url: 'https://a.example', has_vector: true },
  ],
  edges: [
    { from: 'search:s1', to: 'url:u1', rank: 2 },
    { from: 'search:s1', to: 'url:u2', rank: 1 },
    { from: 'search:s2', to: 'url:u1', rank: 5 },
    { from: 'url:u1', to: 'scrape:c1' },
    { from: 'url:u1', to: 'fact:f1' },
  ],
})

describe('buildTree', () => {
  it('nests urls under the searches that found them', () => {
    const tree = buildTree(full)

    expect(tree.searches).toHaveLength(2)
    expect(tree.searches[0].search.ref_id).toBe('s1')
    expect(tree.searches[0].urls.map((u) => u.url.ref_id)).toEqual(['u2', 'u1'])
  })

  it('orders a search\'s urls by rank', () => {
    const tree = buildTree(full)
    expect(tree.searches[0].urls.map((u) => u.rank)).toEqual([1, 2])
  })

  it('repeats a shared url under each finding search, with that search\'s rank', () => {
    const tree = buildTree(full)

    const fromS1 = tree.searches[0].urls.find((u) => u.url.ref_id === 'u1')
    const fromS2 = tree.searches[1].urls.find((u) => u.url.ref_id === 'u1')
    expect(fromS1?.rank).toBe(2)
    expect(fromS2?.rank).toBe(5)
  })

  it('attaches a url\'s scrapes and facts', () => {
    const tree = buildTree(full)
    const branch = tree.searches[1].urls[0]

    expect(branch.scrapes.map((s) => s.ref_id)).toEqual(['c1'])
    expect(branch.facts.map((f) => f.ref_id)).toEqual(['f1'])
    expect(branch.facts[0].has_vector).toBe(true)
  })

  it('leaves a url with no scrape or facts empty rather than undefined', () => {
    const tree = buildTree(full)
    const branch = tree.searches[0].urls.find((u) => u.url.ref_id === 'u2')

    expect(branch?.scrapes).toEqual([])
    expect(branch?.facts).toEqual([])
  })

  it('keeps a search that found nothing', () => {
    const tree = buildTree(
      graph({ nodes: [{ id: 'search:s1', kind: 'search', ref_id: 's1', label: 'google · term' }] }),
    )
    expect(tree.searches).toHaveLength(1)
    expect(tree.searches[0].urls).toEqual([])
  })

  it('surfaces urls no search points at rather than dropping them', () => {
    const tree = buildTree(
      graph({
        nodes: [{ id: 'url:u9', kind: 'url', ref_id: 'u9', label: 'https://orphan.example', url: 'https://orphan.example' }],
      }),
    )
    expect(tree.orphanUrls).toHaveLength(1)
    expect(tree.orphanUrls[0].url.ref_id).toBe('u9')
  })

  it('skips a dangling edge instead of rendering a blank branch', () => {
    const tree = buildTree(
      graph({
        nodes: [{ id: 'search:s1', kind: 'search', ref_id: 's1', label: 'google · term' }],
        edges: [{ from: 'search:s1', to: 'url:missing', rank: 1 }],
      }),
    )
    expect(tree.searches[0].urls).toEqual([])
  })

  it('handles an empty graph', () => {
    const tree = buildTree(graph())
    expect(tree.searches).toEqual([])
    expect(tree.orphanUrls).toEqual([])
  })
})

describe('countNodes', () => {
  it('counts each kind', () => {
    expect(countNodes(full)).toEqual({ searches: 2, urls: 2, scrapes: 1, facts: 1 })
  })

  it('counts an empty graph as zero', () => {
    expect(countNodes(graph())).toEqual({ searches: 0, urls: 0, scrapes: 0, facts: 0 })
  })
})
