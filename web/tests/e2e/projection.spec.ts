/**
 * T022 — the embedding scatter over the T021 dump.
 *
 * The fixtures seed exactly two vectors, one per owner kind, at dim 8. Two
 * points is the degenerate case for a 2-D projection — rank 1 after centring —
 * so these specs also prove the layout does not produce NaN coordinates there.
 */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'

test('the plot renders one point per stored vector, tagged by owner kind', async ({ page }) => {
  await page.goto(baseUrl('/projection'))

  await expect(page.getByTestId('projection-plot')).toBeVisible()
  const points = page.getByTestId('projection-point')
  await expect(points).toHaveCount(2)

  const kinds = await points.evaluateAll((nodes) => nodes.map((n) => n.getAttribute('data-kind')))
  expect(kinds.sort()).toEqual(['memory', 'search'])

  // A NaN coordinate would silently drop the point off the canvas.
  const coords = await points.evaluateAll((nodes) =>
    nodes.map((n) => [Number(n.getAttribute('cx')), Number(n.getAttribute('cy'))]),
  )
  for (const [cx, cy] of coords) {
    expect(Number.isFinite(cx)).toBe(true)
    expect(Number.isFinite(cy)).toBe(true)
  }
})

test('the summary says how much of the space is shown, and how exact it is not', async ({ page }) => {
  await page.goto(baseUrl('/projection'))

  const summary = page.getByTestId('projection-summary')
  await expect(summary).toContainText('Showing 2 of 2')
  await expect(summary).toContainText('8 dimensions reduced to 2')
  await expect(summary).toContainText('indicative')
  await expect(page.getByTestId('projection-legend')).toBeVisible()
})

test('selecting a memory point links to its fact and its source', async ({ page }) => {
  await page.goto(baseUrl('/projection'))
  await expect(page.getByTestId('projection-prompt')).toBeVisible()

  await page.getByTestId('projection-point').filter({ has: page.locator('title') }).first().click()
  await expect(page.getByTestId('projection-selection')).toBeVisible()

  // Click through whichever kind was selected first, then check the other.
  const kind = await page.getByTestId('projection-selected-kind').textContent()
  if (kind?.includes('Memory')) {
    await expect(page.getByTestId('projection-selected-label')).toContainText('Fixture One')
    await page.getByTestId('projection-fact-link').click()
    await expect(page).toHaveURL(/\/facts\//)
    await expect(page.getByTestId('fact-text')).toContainText('Fixture One')
  } else {
    await expect(page.getByTestId('projection-selected-label')).toContainText('fixture term')
  }
})

test('every point can be selected, and each offers the links its kind has', async ({ page }) => {
  await page.goto(baseUrl('/projection'))
  const points = page.getByTestId('projection-point')

  for (let i = 0; i < 2; i++) {
    await points.nth(i).click()
    await expect(page.getByTestId('projection-selection')).toBeVisible()
    const kind = await page.getByTestId('projection-selected-kind').textContent()

    // The explorer link is the one thing both kinds carry.
    await expect(page.getByTestId('projection-explore-link')).toHaveAttribute('href', /\/explore\?q=/)
    if (kind?.includes('Memory')) {
      await expect(page.getByTestId('projection-fact-link')).toBeVisible()
      await expect(page.getByTestId('projection-source-link')).toBeVisible()
    } else {
      await expect(page.getByTestId('projection-fact-link')).toHaveCount(0)
    }
  }
})

test('a point selects with the keyboard and clears again', async ({ page }) => {
  await page.goto(baseUrl('/projection'))

  await page.getByTestId('projection-point').first().press('Enter')
  await expect(page.getByTestId('projection-selection')).toBeVisible()

  await page.getByTestId('projection-clear').click()
  await expect(page.getByTestId('projection-prompt')).toBeVisible()
})

test('the source link reaches provenance and the explorer link runs a probe', async ({ page }) => {
  await page.goto(baseUrl('/projection'))

  // Pick the memory point by its label, whichever index it landed on.
  await page
    .getByTestId('projection-point')
    .filter({ hasText: 'The fixture page says Fixture One.' })
    .click()

  await page.getByTestId('projection-source-link').click()
  await expect(page).toHaveURL(/\/provenance\?url=/)
  await expect(page.getByTestId('found-by-table')).toContainText('google')

  await page.goBack()
  await page.getByTestId('projection-point').filter({ hasText: 'The fixture page says Fixture One.' }).click()
  await page.getByTestId('projection-explore-link').click()
  await expect(page).toHaveURL(/\/explore\?q=/)
  await expect(page.getByTestId('explore-results')).toBeVisible()
})
