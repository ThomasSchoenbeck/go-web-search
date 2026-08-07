/** T006 — runs list and run detail, against the seeded fixture data. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'
import { seeded } from './fixtures-data'

test('the runs list renders seeded runs and links to detail', async ({ page }) => {
  await page.goto(baseUrl('/runs'))

  const table = page.getByTestId('runs-table')
  await expect(table).toBeVisible()
  await expect(page.getByTestId('run-link').filter({ hasText: seeded.runId })).toBeVisible()
  await expect(table).toContainText('testserve')
})

test('the limit select and reload button both refetch', async ({ page }) => {
  await page.goto(baseUrl('/runs'))
  await expect(page.getByTestId('runs-table')).toBeVisible()

  const limitRequest = page.waitForResponse((r) => r.url().includes('/api/runs?limit=10'))
  await page.getByTestId('run-limit').selectOption('10')
  await limitRequest
  await expect(page.getByTestId('run-limit')).toHaveValue('10')
  await expect(page.getByTestId('runs-table')).toBeVisible()

  const reloadRequest = page.waitForResponse((r) => r.url().includes('/api/runs'))
  await page.getByTestId('runs-reload').click()
  await reloadRequest
  await expect(page.getByTestId('runs-table')).toBeVisible()
})

test('clicking a run opens its detail with all three child lists', async ({ page }) => {
  await page.goto(baseUrl('/runs'))
  await page.getByTestId('run-link').filter({ hasText: seeded.runId }).click()

  await expect(page).toHaveURL(new RegExp(`/runs/${seeded.runId}$`))
  await expect(page.getByTestId('run-summary')).toContainText('testserve')
  await expect(page.getByTestId('run-summary')).toContainText('2 searches · 2 URLs · 2 scrapes')

  await expect(page.getByTestId('searches-list')).toContainText('google')
  await expect(page.getByTestId('searches-list')).toContainText('bing')
  await expect(page.getByTestId('search-blocked')).toBeVisible()
  await expect(page.getByTestId('urls-table')).toContainText('example.com')
  await expect(page.getByTestId('scrapes-list')).toContainText(seeded.scrapeWithContent)
})

test('run detail links navigate onward and back', async ({ page }) => {
  await page.goto(baseUrl(`/runs/${seeded.runId}`))
  await expect(page.getByTestId('run-summary')).toBeVisible()

  // External result URLs open in a new tab by design, so assert the href
  // rather than following them out of the app.
  const firstUrl = page.getByTestId('urls-table').getByRole('link').first()
  await expect(firstUrl).toHaveAttribute('target', '_blank')
  await expect(firstUrl).toHaveAttribute('rel', /noreferrer/)

  await page.getByTestId('scrape-link').first().click()
  await expect(page).toHaveURL(/\/scrapes\//)
  await page.goBack()

  await page.getByTestId('search-link').first().click()
  await expect(page).toHaveURL(/\/searches\//)
  await page.goBack()

  await page.getByTestId('all-searches-link').click()
  await expect(page).toHaveURL(new RegExp(`/runs/${seeded.runId}/searches$`))

  await page.getByTestId('back-to-run').click()
  await expect(page.getByTestId('run-summary')).toBeVisible()

  await page.getByTestId('back-to-runs').click()
  await expect(page.getByTestId('runs-table')).toBeVisible()
})

test('a run with no children renders empty states', async ({ page }) => {
  // The testserve process records a run of its own that has no children.
  await page.goto(baseUrl('/runs'))
  const own = page.getByTestId('run-link').filter({ hasNotText: seeded.runId }).first()
  await own.click()

  await expect(page.getByTestId('searches-empty')).toBeVisible()
  await expect(page.getByTestId('urls-empty')).toBeVisible()
  await expect(page.getByTestId('scrapes-empty')).toBeVisible()
})
