/** T011 + T013 — the memory facts browser, fact detail, and the semantic explorer. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'

const fixtureUrl = 'https://example.com/fixture-one'
const factText = 'The fixture page says Fixture One.'

test('the facts browser lists the seeded fact with its metadata', async ({ page }) => {
  await page.goto(baseUrl('/facts'))

  const table = page.getByTestId('facts-table')
  await expect(table).toBeVisible()
  await expect(table).toContainText(factText)
  await expect(table).toContainText('stable')
  await expect(table).toContainText('short')
})

test('the search box filters and clears', async ({ page }) => {
  await page.goto(baseUrl('/facts'))
  await expect(page.getByTestId('facts-table')).toBeVisible()

  await page.getByTestId('facts-search').fill('fixture')
  await page.getByTestId('facts-search-submit').click()
  await expect(page.getByTestId('facts-table')).toContainText(factText)

  // A term that matches nothing must read as "no match", not as an empty store.
  await page.getByTestId('facts-search').fill('zzzz-no-such-fact')
  await page.getByTestId('facts-search-submit').click()
  await expect(page.getByTestId('facts-empty')).toContainText('No facts match')

  await page.getByTestId('facts-search-clear').click()
  await expect(page.getByTestId('facts-table')).toBeVisible()
})

test('paging controls reflect a single short page', async ({ page }) => {
  await page.goto(baseUrl('/facts'))
  await expect(page.getByTestId('facts-table')).toBeVisible()

  await expect(page.getByTestId('facts-prev')).toBeDisabled()
  await expect(page.getByTestId('facts-next')).toBeDisabled()
  await expect(page.getByTestId('facts-page')).toContainText('Page 1')

  const request = page.waitForResponse((r) => r.url().includes('limit=100'))
  await page.getByTestId('facts-limit').selectOption('100')
  await request
  await expect(page.getByTestId('facts-table')).toBeVisible()
})

test('opening a fact shows its detail, source sizes and raw link', async ({ page }) => {
  await page.goto(baseUrl('/facts'))
  await page.getByTestId('fact-link').first().click()

  await expect(page).toHaveURL(/\/facts\//)
  await expect(page.getByTestId('fact-text')).toContainText(factText)
  await expect(page.getByTestId('fact-metadata')).toContainText('stable')

  await expect(page.getByTestId('fact-source')).toContainText('Fixture One')
  await expect(page.getByTestId('fact-read-raw')).toHaveAttribute('href', /\/api\/scrapes\/.*raw=1/)
})

test('a fact links back to its source through provenance — the reverse path', async ({ page }) => {
  await page.goto(baseUrl('/facts'))
  await page.getByTestId('fact-link').first().click()

  await page.getByTestId('fact-provenance-link').click()
  await expect(page).toHaveURL(/\/provenance\?url=/)
  await expect(page.getByTestId('found-by-table')).toContainText('google')
})

test('a fact links to the scrape it was distilled from', async ({ page }) => {
  await page.goto(baseUrl('/facts'))
  await page.getByTestId('fact-link').first().click()

  await page.getByTestId('fact-scrape-link').click()
  await expect(page).toHaveURL(/\/scrapes\//)
  await expect(page.getByTestId('scrape-metadata')).toBeVisible()

  await page.goBack()
  await page.getByTestId('back-to-facts').click()
  await expect(page.getByTestId('facts-table')).toBeVisible()
})

test('the facts list links a source straight to its provenance', async ({ page }) => {
  await page.goto(baseUrl('/facts'))
  await page.getByTestId('fact-source-link').first().click()

  await expect(page).toHaveURL(new RegExp(encodeURIComponent(fixtureUrl).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))
  await expect(page.getByTestId('provenance-scrape')).toBeVisible()
})

test('a missing fact id reports not found', async ({ page }) => {
  await page.goto(baseUrl('/facts/does-not-exist'))
  await expect(page.getByTestId('fact-missing')).toBeVisible()
})

test('the explorer prompts before a query is entered', async ({ page }) => {
  await page.goto(baseUrl('/explore'))
  await expect(page.getByTestId('explore-prompt')).toBeVisible()
})

test('querying the explorer returns neighbours from both owner kinds', async ({ page }) => {
  await page.goto(baseUrl('/explore'))

  await page.getByTestId('explore-input').fill(factText)
  await page.getByTestId('explore-submit').click()

  // The probe lives in the URL, so it can be linked to.
  await expect(page).toHaveURL(/\/explore\?q=/)

  const rows = page.getByTestId('explore-row')
  await expect(rows).toHaveCount(2)

  const kinds = await page.getByTestId('explore-kind').allTextContents()
  expect(kinds).toContain('fact')
  expect(kinds).toContain('cached search')

  // Querying a stored item's exact text must rank that item first.
  await expect(page.getByTestId('explore-kind').first()).toHaveText('fact')
})

test('the explorer honours k', async ({ page }) => {
  await page.goto(baseUrl('/explore'))

  await page.getByTestId('explore-input').fill(factText)
  await page.getByTestId('explore-k').selectOption('5')
  await page.getByTestId('explore-submit').click()

  await expect(page).toHaveURL(/k=5/)
  await expect(page.getByTestId('explore-row').first()).toBeVisible()
})

test('explorer neighbours link into the facts and provenance views', async ({ page }) => {
  await page.goto(baseUrl(`/explore?q=${encodeURIComponent(factText)}&k=10`))
  await expect(page.getByTestId('explore-results')).toBeVisible()

  await expect(page.getByTestId('explore-search-context')).toContainText('results')

  await page.getByTestId('explore-source-link').click()
  await expect(page).toHaveURL(/\/provenance\?url=/)
  await page.goBack()

  await page.getByTestId('explore-fact-link').click()
  await expect(page.getByTestId('fact-text')).toContainText(factText)
})
