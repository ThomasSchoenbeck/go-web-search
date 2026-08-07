/** T020 — the stats dashboard over the extended /api/stats. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'

test('the dashboard renders every count and the size aggregates', async ({ page }) => {
  await page.goto(baseUrl('/stats'))

  const counts = page.getByTestId('stats-counts')
  await expect(counts).toBeVisible()
  await expect(page.getByTestId('stats-count')).toHaveCount(8)
  // Runs are not asserted exactly: the test server records a run for itself.
  await expect(counts).toContainText('Searches: 2')
  await expect(counts).toContainText('URLs: 2')
  await expect(counts).toContainText('Memory facts: 1')
  await expect(counts).toContainText('Scrapes: 3')

  await expect(page.getByTestId('stats-sizes')).toContainText('Raw HTML, average')
})

test('the embedding section reports the active model and table', async ({ page }) => {
  await page.goto(baseUrl('/stats'))

  const embedding = page.getByTestId('stats-embedding')
  await expect(embedding).toBeVisible()
  // The e2e fixtures seed a vector table but no model meta, which must read as
  // "unknown" rather than as a claim the backend never made.
  await expect(embedding).toContainText('vectors_test')
  await expect(embedding).toContainText('unknown')
  await expect(page.getByTestId('stats-migrating')).toHaveCount(0)
})

test('both caches are broken down by tier and hit count', async ({ page }) => {
  await page.goto(baseUrl('/stats'))

  const caches = page.getByTestId('stats-caches')
  await expect(caches).toBeVisible()
  await expect(page.getByTestId('stats-cache-row')).toHaveCount(2)
  await expect(caches).toContainText('Search')
  await expect(caches).toContainText('Scrape')
  // Counts, with the reason a rate is not shown.
  await expect(page.getByTestId('stats-hit-note')).toContainText('not a hit rate')
})

test('the job summary covers status, type and timing', async ({ page }) => {
  await page.goto(baseUrl('/stats'))

  await expect(page.getByTestId('stats-job-status')).toHaveCount(4)
  await expect(page.getByTestId('stats-jobs')).toContainText('pending: 1')
  await expect(page.getByTestId('stats-jobs')).toContainText('failed: 1')

  const types = page.getByTestId('stats-job-types')
  await expect(types).toContainText('embed: 2')
  await expect(types).toContainText('distill: 1')

  const timing = page.getByTestId('stats-job-timing')
  await expect(timing).toContainText('Most attempts on one job: 3')
  await expect(timing).toContainText('Average time to finish')
})
