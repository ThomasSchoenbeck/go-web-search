/** T010 + T026 — the URL provenance pivot and the whole-run causality graph. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'
import { seeded } from './fixtures-data'

const fixtureUrl = 'https://example.com/fixture-one'

test('the provenance page prompts before a pivot is chosen', async ({ page }) => {
  await page.goto(baseUrl('/provenance'))
  await expect(page.getByTestId('provenance-prompt')).toBeVisible()
})

test('tracing a URL renders both directions of the chain', async ({ page }) => {
  await page.goto(baseUrl('/provenance'))

  await page.getByTestId('provenance-input').fill(fixtureUrl)
  await page.getByTestId('provenance-submit').click()

  // The pivot is in the URL, so the chain is linkable.
  await expect(page).toHaveURL(/\/provenance\?url=/)

  await expect(page.getByTestId('found-by-table')).toContainText('google')
  await expect(page.getByTestId('found-by-table')).toContainText('fixture term')
  await expect(page.getByTestId('provenance-scrape')).toContainText('Fixture One')
  await expect(page.getByTestId('facts-list')).toContainText('The fixture page says Fixture One.')
})

test('vector presence is reported when the store is available', async ({ page }) => {
  // The fixture seeds an active vector table and embeds the fact, so the view
  // makes a definite claim rather than the "unknown" degraded one. The degraded
  // path is covered in Go (TestURLProvenanceDegradesWithoutVectors and
  // …DegradesDuringMigration) and in Provenance.test.ts.
  await page.goto(baseUrl(`/provenance?url=${encodeURIComponent(fixtureUrl)}`))

  await expect(page.getByTestId('vectors-unavailable')).toHaveCount(0)
  await expect(page.getByTestId('fact-vector')).toHaveText('embedded')
})

test('provenance links reach the run, SERP and scrape views', async ({ page }) => {
  await page.goto(baseUrl(`/provenance?url=${encodeURIComponent(fixtureUrl)}`))

  await expect(page.getByTestId('fact-link')).toHaveAttribute('href', /\/facts\//)

  await page.getByTestId('provenance-scrape-link').click()
  await expect(page).toHaveURL(/\/scrapes\//)
  await page.goBack()

  await page.getByTestId('found-search-link').click()
  await expect(page).toHaveURL(/\/searches\//)
  await page.goBack()

  await page.getByTestId('found-run-link').click()
  await expect(page.getByTestId('run-summary')).toBeVisible()
})

test('an unknown URL reports an empty chain, not an error', async ({ page }) => {
  await page.goto(baseUrl(`/provenance?url=${encodeURIComponent('https://nobody.example/nope')}`))

  await expect(page.getByTestId('provenance-unknown')).toBeVisible()
  await expect(page.getByTestId('found-by-empty')).toBeVisible()
  await expect(page.getByTestId('scrape-none')).toBeVisible()
  await expect(page.getByTestId('facts-empty')).toBeVisible()
  await expect(page.getByTestId('provenance-error')).toHaveCount(0)
})

test('the run causality graph renders the whole chain', async ({ page }) => {
  await page.goto(baseUrl(`/runs/${seeded.runId}/causality`))

  await expect(page.getByTestId('causality-counts')).toContainText('2 searches')
  await expect(page.getByTestId('causality-counts')).toContainText('2 URLs')
  await expect(page.getByTestId('causality-search')).toHaveCount(2)
  await expect(page.getByTestId('causality-rank').first()).toContainText('#1')
  await expect(page.getByTestId('causality-scrape-link').first()).toBeVisible()
  await expect(page.getByTestId('causality-fact-link')).toContainText('The fixture page says')
})

test('a search that found nothing says so', async ({ page }) => {
  // The seeded bing search was blocked and returned no URLs.
  await page.goto(baseUrl(`/runs/${seeded.runId}/causality`))
  await expect(page.getByTestId('causality-search-empty')).toBeVisible()
})

test('causality nodes cross-link into the other views', async ({ page }) => {
  await page.goto(baseUrl(`/runs/${seeded.runId}/causality`))

  await expect(page.getByTestId('causality-fact-link')).toHaveAttribute('href', /\/facts\//)

  await page.getByTestId('causality-url-link').first().click()
  await expect(page).toHaveURL(/\/provenance\?url=/)
  await expect(page.getByTestId('found-by-table')).toBeVisible()
  await page.goBack()

  await page.getByTestId('causality-scrape-link').first().click()
  await expect(page).toHaveURL(/\/scrapes\//)
  await page.goBack()

  await page.getByTestId('causality-search-link').first().click()
  await expect(page).toHaveURL(/\/searches\//)
  await page.goBack()

  await page.getByTestId('back-to-run').click()
  await expect(page.getByTestId('run-summary')).toBeVisible()
})

test('the causality graph is reachable from the run detail view', async ({ page }) => {
  await page.goto(baseUrl(`/runs/${seeded.runId}`))
  await page.getByTestId('run-causality-link').click()

  await expect(page).toHaveURL(new RegExp(`/runs/${seeded.runId}/causality$`))
  await expect(page.getByTestId('causality-counts')).toBeVisible()
})

test('an empty run renders a clear message', async ({ page }) => {
  await page.goto(baseUrl('/runs'))
  const own = page.getByTestId('run-link').filter({ hasNotText: seeded.runId }).first()
  const href = await own.getAttribute('href')
  await page.goto(baseUrl(`${href}/causality`))

  await expect(page.getByTestId('causality-empty')).toBeVisible()
})
