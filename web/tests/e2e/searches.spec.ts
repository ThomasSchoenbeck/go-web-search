/** T007 — searches list and the sandboxed raw SERP viewer. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'
import { seeded } from './fixtures-data'

test('the searches list shows every metadata field', async ({ page }) => {
  await page.goto(baseUrl(`/runs/${seeded.runId}/searches`))

  const table = page.getByTestId('searches-table')
  await expect(table).toBeVisible()
  await expect(page.getByTestId('search-row')).toHaveCount(2)

  await expect(table).toContainText('google')
  await expect(table).toContainText('typed')
  await expect(table).toContainText('200')
  await expect(table).toContainText('bing')
  await expect(table).toContainText('direct')
  await expect(table).toContainText('429')
  await expect(table).toContainText('challenged by engine')
})

test('a stored SERP renders in a fully sandboxed frame', async ({ page }) => {
  await page.goto(baseUrl(`/searches/${seeded.searchWithSerp}`))

  const frame = page.getByTestId('serp-frame')
  await expect(frame).toBeVisible()
  await expect(frame).toHaveAttribute('sandbox', '')
  await expect(page.getByTestId('serp-size')).toContainText('characters')

  // The sandboxed document really does render the stored markup.
  await expect(page.frameLocator('[data-testid="serp-frame"]').locator('body')).toContainText('fixture SERP')
})

test('the rendered and source views toggle', async ({ page }) => {
  await page.goto(baseUrl(`/searches/${seeded.searchWithSerp}`))
  await expect(page.getByTestId('serp-frame')).toBeVisible()

  await page.getByTestId('serp-view-source').click()
  await expect(page.getByTestId('serp-source')).toContainText('<html>')
  await expect(page.getByTestId('serp-frame')).toHaveCount(0)

  await page.getByTestId('serp-view-rendered').click()
  await expect(page.getByTestId('serp-frame')).toBeVisible()
})

test('a search with no stored SERP shows a clear empty state', async ({ page }) => {
  await page.goto(baseUrl(`/searches/${seeded.searchWithoutSerp}`))

  await expect(page.getByTestId('serp-missing')).toBeVisible()
  await expect(page.getByTestId('serp-error')).toHaveCount(0)
})

test('reaching the SERP viewer from the searches list', async ({ page }) => {
  await page.goto(baseUrl(`/runs/${seeded.runId}/searches`))
  await page.getByTestId('serp-link').first().click()

  await expect(page).toHaveURL(/\/searches\//)
  await expect(page.getByRole('heading', { name: 'Raw SERP', level: 1 })).toBeVisible()
})
