/**
 * T019 — the logs viewer: the tail over the separate log database.
 *
 * The test server logs its own startup into that same database, under its own
 * run id, so no spec here asserts a total line count — the same trap as the
 * `runs` table. Assertions pivot on the seeded run id or on fixture text.
 */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'
import { seeded } from './fixtures-data'

test('the viewer lists log lines with every field', async ({ page }) => {
  await page.goto(baseUrl('/logs'))

  const table = page.getByTestId('logs-table')
  await expect(table).toBeVisible()
  await expect(table).toContainText('fixture run started')
  await expect(table).toContainText('fixture fetch failed')
  await expect(table).toContainText('scraper')
})

test('the seeded run reads newest-first', async ({ page }) => {
  await page.goto(baseUrl('/logs'))
  await page.getByTestId('logs-run').fill(seeded.runId)
  await page.getByTestId('logs-submit').click()

  // The three run-attached fixtures were written a second apart, warn last.
  await expect(page.getByTestId('log-row')).toHaveCount(3)
  expect(await page.getByTestId('log-level').allTextContents()).toEqual(['warn', 'notice', 'info'])
})

test('levels are visually distinguishable', async ({ page }) => {
  await page.goto(baseUrl('/logs'))
  await expect(page.getByTestId('logs-table')).toBeVisible()

  await expect(page.getByTestId('log-row').filter({ hasText: 'fixture fetch failed' })).toHaveClass(/level-error/)
  await expect(page.getByTestId('log-row').filter({ hasText: 'fixture page was slow' })).toHaveClass(/level-warn/)
  await expect(page.getByTestId('log-row').filter({ hasText: 'fixture run started' })).toHaveClass(/level-info/)
})

test('the level filter narrows the tail', async ({ page }) => {
  await page.goto(baseUrl('/logs'))
  await expect(page.getByTestId('logs-table')).toBeVisible()

  await page.getByTestId('logs-level').selectOption('warn')
  await expect(page.getByTestId('logs-table')).toContainText('fixture page was slow')
  const levels = await page.getByTestId('log-level').allTextContents()
  expect(levels.length).toBeGreaterThan(0)
  expect(new Set(levels)).toEqual(new Set(['warn']))

  await page.getByTestId('logs-level').selectOption('')
  await expect(page.getByTestId('logs-table')).toContainText('fixture run started')
})

test('the run and source filters narrow the tail, and clear resets everything', async ({ page }) => {
  await page.goto(baseUrl('/logs'))
  await expect(page.getByTestId('logs-table')).toBeVisible()

  await page.getByTestId('logs-run').fill(seeded.runId)
  await page.getByTestId('logs-submit').click()
  await expect(page.getByTestId('log-row')).toHaveCount(3)

  await page.getByTestId('logs-source').fill('scraper')
  await page.getByTestId('logs-submit').click()
  await expect(page.getByTestId('log-row')).toHaveCount(1)

  await page.getByTestId('logs-source').fill('no-such-source')
  await page.getByTestId('logs-submit').click()
  await expect(page.getByTestId('logs-empty')).toContainText('No log lines match')

  await page.getByTestId('logs-clear').click()
  // The line written outside a run is back, so every filter really did lift.
  await expect(page.getByTestId('logs-table')).toContainText('fixture fetch failed')
})

test('a log line links to the run that produced it', async ({ page }) => {
  await page.goto(baseUrl('/logs'))
  await page.getByTestId('logs-run').fill(seeded.runId)
  await page.getByTestId('logs-submit').click()

  await page.getByTestId('log-run-link').first().click()
  await expect(page).toHaveURL(new RegExp(`/runs/${seeded.runId}$`))
  await expect(page.getByTestId('run-summary')).toBeVisible()
})

test('the paging controls page through the tail', async ({ page }) => {
  await page.goto(baseUrl('/logs'))
  await expect(page.getByTestId('logs-table')).toBeVisible()

  await expect(page.getByTestId('logs-prev')).toBeDisabled()
  await expect(page.getByTestId('logs-page')).toContainText('Page 1')

  // One seeded run's lines are far short of a page, so there is nothing older.
  await page.getByTestId('logs-run').fill(seeded.runId)
  await page.getByTestId('logs-submit').click()
  await expect(page.getByTestId('logs-next')).toBeDisabled()
})

test('the tail starts paused and the controls drive it', async ({ page }) => {
  await page.goto(baseUrl('/logs'))
  await expect(page.getByTestId('logs-table')).toBeVisible()
  await expect(page.getByTestId('poll-status')).toContainText('paused')

  await page.getByTestId('poll-interval').selectOption('1000')
  await page.getByTestId('poll-toggle').click()
  await expect(page.getByTestId('poll-status')).toContainText('refreshing every 1.0s')
  await page.waitForResponse((r) => r.url().includes('/api/logs') && r.request().method() === 'GET')

  await page.getByTestId('poll-toggle').click()
  await expect(page.getByTestId('poll-status')).toContainText('paused')
})
