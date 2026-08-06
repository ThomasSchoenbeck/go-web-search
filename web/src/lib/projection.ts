/**
 * PCA down to two dimensions, in the browser.
 *
 * The confirmed decision (plans/observability-ui/index.md) is that the layout is
 * computed client-side and is PCA, not UMAP or t-SNE: PCA is deterministic — the
 * same vectors always produce the same picture — and it is short enough that a
 * local helper beats taking on a dependency for it.
 *
 * The method is power iteration on the covariance matrix, never forming that
 * matrix: for embeddings the dimension is far larger than the point count, so a
 * d×d covariance (4096² floats) would dwarf the data it summarises. Each
 * iteration is a pass over the points instead.
 */

export interface Point2D {
  x: number
  y: number
}

/** Bounded so a pathological input cannot spin; PCA converges well before this. */
const maxIterations = 128
const tolerance = 1e-7

/**
 * Projects vectors onto their first two principal components.
 *
 * Returns one point per input, in the same order. Degenerate inputs are handled
 * rather than producing NaN: fewer than two points, or vectors that are all
 * identical, collapse to the origin.
 */
export function projectTo2D(vectors: number[][]): Point2D[] {
  if (vectors.length === 0) return []
  const dim = vectors[0].length
  if (dim === 0) return vectors.map(() => ({ x: 0, y: 0 }))

  const centered = center(vectors, dim)
  const first = principalComponent(centered, dim, [])
  const second = principalComponent(centered, dim, [first])

  return centered.map((row) => ({ x: dot(row, first), y: dot(row, second) }))
}

/**
 * Fits projected points into a width×height box with a margin, flipping y so the
 * plot reads the usual way up in SVG coordinates. A dimension with no spread is
 * centred rather than divided by zero.
 */
export function scaleToBox(points: Point2D[], width: number, height: number, margin: number): Point2D[] {
  if (points.length === 0) return []
  const xs = points.map((p) => p.x)
  const ys = points.map((p) => p.y)
  const spanX = Math.max(...xs) - Math.min(...xs)
  const spanY = Math.max(...ys) - Math.min(...ys)
  const minX = Math.min(...xs)
  const minY = Math.min(...ys)
  const innerWidth = width - 2 * margin
  const innerHeight = height - 2 * margin

  return points.map((p) => ({
    x: spanX === 0 ? width / 2 : margin + ((p.x - minX) / spanX) * innerWidth,
    // SVG y grows downwards, so the largest value must land nearest the top.
    y: spanY === 0 ? height / 2 : height - margin - ((p.y - minY) / spanY) * innerHeight,
  }))
}

function center(vectors: number[][], dim: number): number[][] {
  const mean = new Array<number>(dim).fill(0)
  for (const vector of vectors) {
    for (let i = 0; i < dim; i++) mean[i] += vector[i]
  }
  for (let i = 0; i < dim; i++) mean[i] /= vectors.length
  return vectors.map((vector) => vector.map((value, i) => value - mean[i]))
}

/**
 * One principal component by power iteration, kept orthogonal to any already
 * found. Deflating the data would cost another full copy of it; projecting the
 * candidate instead is equivalent and allocates nothing.
 */
function principalComponent(centered: number[][], dim: number, previous: number[][]): number[] {
  let vector = orthogonalize(seededUnit(dim, previous.length + 1), previous)

  for (let iteration = 0; iteration < maxIterations; iteration++) {
    const next = new Array<number>(dim).fill(0)
    for (const row of centered) {
      const weight = dot(row, vector)
      for (let i = 0; i < dim; i++) next[i] += weight * row[i]
    }
    const orthogonal = orthogonalize(next, previous)
    const length = norm(orthogonal)
    if (length < tolerance) {
      // No variance left in this direction — a single point, identical vectors,
      // or data whose rank the earlier components already exhausted.
      return new Array<number>(dim).fill(0)
    }
    const normalized = orthogonal.map((value) => value / length)
    const shift = distance(normalized, vector)
    vector = normalized
    // Converged, or converged onto the opposite sign of the same axis.
    if (shift < tolerance || shift > 2 - tolerance) break
  }
  return vector
}

/**
 * A deterministic pseudo-random unit vector. A uniform start would be exactly
 * orthogonal to the leading component on symmetric data, which power iteration
 * cannot recover from; a fixed seed keeps the layout reproducible anyway.
 */
function seededUnit(dim: number, seed: number): number[] {
  let state = seed * 2654435761
  const vector = new Array<number>(dim)
  for (let i = 0; i < dim; i++) {
    state = (state * 1664525 + 1013904223) >>> 0
    vector[i] = state / 0xffffffff - 0.5
  }
  const length = norm(vector)
  return length === 0 ? vector : vector.map((value) => value / length)
}

function orthogonalize(vector: number[], previous: number[][]): number[] {
  let result = vector
  for (const basis of previous) {
    const weight = dot(result, basis)
    result = result.map((value, i) => value - weight * basis[i])
  }
  return result
}

function dot(a: number[], b: number[]): number {
  let sum = 0
  for (let i = 0; i < a.length; i++) sum += a[i] * b[i]
  return sum
}

function norm(vector: number[]): number {
  return Math.sqrt(dot(vector, vector))
}

function distance(a: number[], b: number[]): number {
  let sum = 0
  for (let i = 0; i < a.length; i++) sum += (a[i] - b[i]) ** 2
  return Math.sqrt(sum)
}
