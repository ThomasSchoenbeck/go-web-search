import { describe, it, expect } from 'vitest'
import { projectTo2D, scaleToBox } from './projection'

/** Spread along the first axis, in the order the projection should preserve. */
const alongX = [
  [-2, 0, 0],
  [-1, 0, 0],
  [1, 0, 0],
  [2, 0, 0],
]

describe('projectTo2D', () => {
  it('returns one point per input, in order', () => {
    expect(projectTo2D(alongX)).toHaveLength(4)
  })

  it('puts the axis of greatest variance on x', () => {
    // The data varies 10× more along dimension 0 than dimension 1.
    const points = projectTo2D([
      [-10, -1, 0],
      [-5, 1, 0],
      [5, -1, 0],
      [10, 1, 0],
    ])
    const spanX = Math.max(...points.map((p) => p.x)) - Math.min(...points.map((p) => p.x))
    const spanY = Math.max(...points.map((p) => p.y)) - Math.min(...points.map((p) => p.y))
    expect(spanX).toBeGreaterThan(spanY * 5)
  })

  it('preserves the ordering along the leading component', () => {
    const xs = projectTo2D(alongX).map((p) => p.x)
    const ascending = [...xs].sort((a, b) => a - b)
    const descending = [...ascending].reverse()
    // PCA fixes the axis, not its sign, so either direction is correct.
    expect(xs).toEqual(xs[0] < xs[3] ? ascending : descending)
  })

  it('is deterministic — the same vectors always give the same layout', () => {
    expect(projectTo2D(alongX)).toEqual(projectTo2D(alongX))
  })

  it('separates two clusters', () => {
    const cluster = (offset: number) =>
      Array.from({ length: 5 }, (_, i) => [offset + i * 0.01, i * 0.01, 0])
    const points = projectTo2D([...cluster(0), ...cluster(100)])

    const left = points.slice(0, 5).map((p) => p.x)
    const right = points.slice(5).map((p) => p.x)
    const gap = Math.abs(Math.min(...right) - Math.max(...left))
    const spread = Math.max(...left) - Math.min(...left)
    expect(gap).toBeGreaterThan(spread * 10)
  })

  it('handles a second component that carries real variance', () => {
    const points = projectTo2D([
      [-2, -1, 0],
      [-2, 1, 0],
      [2, -1, 0],
      [2, 1, 0],
    ])
    const spanY = Math.max(...points.map((p) => p.y)) - Math.min(...points.map((p) => p.y))
    expect(spanY).toBeGreaterThan(0.5)
    for (const point of points) {
      expect(Number.isFinite(point.x)).toBe(true)
      expect(Number.isFinite(point.y)).toBe(true)
    }
  })

  // The degenerate shapes the seeded fixtures actually produce.
  it('returns nothing for no vectors', () => {
    expect(projectTo2D([])).toEqual([])
  })

  it('collapses a single vector to the origin without NaN', () => {
    expect(projectTo2D([[1, 2, 3]])).toEqual([{ x: 0, y: 0 }])
  })

  it('collapses identical vectors to the origin without NaN', () => {
    const points = projectTo2D([
      [1, 1],
      [1, 1],
    ])
    expect(points).toEqual([
      { x: 0, y: 0 },
      { x: 0, y: 0 },
    ])
  })

  it('handles two points, where the second component has no variance left', () => {
    const points = projectTo2D([
      [0, 0],
      [1, 1],
    ])
    expect(points).toHaveLength(2)
    for (const point of points) {
      expect(Number.isNaN(point.x)).toBe(false)
      expect(Number.isNaN(point.y)).toBe(false)
    }
    expect(points[0].x).not.toBe(points[1].x)
    // Rank 1 after centring: there is no second axis to spread along.
    expect(points[0].y).toBe(0)
    expect(points[1].y).toBe(0)
  })

  it('survives zero-length vectors', () => {
    expect(projectTo2D([[], []])).toEqual([
      { x: 0, y: 0 },
      { x: 0, y: 0 },
    ])
  })
})

describe('scaleToBox', () => {
  it('fits points inside the box, margin included', () => {
    const scaled = scaleToBox(
      [
        { x: -1, y: -1 },
        { x: 1, y: 1 },
      ],
      100,
      100,
      10,
    )
    for (const point of scaled) {
      expect(point.x).toBeGreaterThanOrEqual(10)
      expect(point.x).toBeLessThanOrEqual(90)
      expect(point.y).toBeGreaterThanOrEqual(10)
      expect(point.y).toBeLessThanOrEqual(90)
    }
  })

  it('flips y so the largest value is nearest the top', () => {
    const scaled = scaleToBox(
      [
        { x: 0, y: 0 },
        { x: 0, y: 10 },
      ],
      100,
      100,
      10,
    )
    expect(scaled[1].y).toBeLessThan(scaled[0].y)
  })

  it('centres an axis with no spread instead of dividing by zero', () => {
    const scaled = scaleToBox(
      [
        { x: 0, y: 0 },
        { x: 0, y: 0 },
      ],
      100,
      50,
      10,
    )
    expect(scaled).toEqual([
      { x: 50, y: 25 },
      { x: 50, y: 25 },
    ])
  })

  it('returns nothing for no points', () => {
    expect(scaleToBox([], 100, 100, 10)).toEqual([])
  })
})
