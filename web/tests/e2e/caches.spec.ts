/** T017 — the two cache browsers: filters, paging, and the links out. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'
import { seeded } from './fixtures-data'

const fixtureUrl = 'https://example.com/fixture-one'

test('the search cache lists cached queries with tier, hits and a results summary', async ({ page }) => {
  await page.goto(baseUrl('/cache/searches'))

  const table = page.getByTestId('search-cache-table')
  await expect(table).toBeVisible()
  await expect(page.getByTestId('search-cache-row')).toHaveCount(2)
  await expect(table).toContainText('fixture term')
  await expect(table).toContainText('Fixture Archive')
  await expect(table).toContainText('short')
  await expect(table).toContainText('long')
})

test('the search cache filters by tier and by query text', async ({ page }) => {
  await page.goto(baseUrl('/cache/searches'))
  await expect(page.getByTestId('search-cache-table')).toBeVisible()

  await page.getByTestId('search-cache-tier').selectOption('long')
  await expect(page.getByTestId('search-cache-row')).toHaveCount(1)
  await expect(page.getByTestId('search-cache-tier-cell')).toHaveText('long')

  await page.getByTestId('search-cache-filter').fill('zzz-no-such-query')
  await page.getByTestId('search-cache-submit').click()
  await expect(page.getByTestId('search-cache-empty')).toContainText('No cached queries match')

  await page.getByTestId('search-cache-clear').click()
  await expect(page.getByTestId('search-cache-row')).toHaveCount(1) // the tier filter still stands

  await page.getByTestId('search-cache-tier').selectOption('')
  await expect(page.getByTestId('search-cache-row')).toHaveCount(2)
})

test('the search cache paging controls reflect a single short page', async ({ page }) => {
  await page.goto(baseUrl('/cache/searches'))
  await expect(page.getByTestId('search-cache-table')).toBeVisible()

  await expect(page.getByTestId('search-cache-prev')).toBeDisabled()
  await expect(page.getByTestId('search-cache-next')).toBeDisabled()
  await expect(page.getByTestId('search-cache-page')).toContainText('Page 1')
})

test('the scrape cache lists cached pages with sizes, not bodies', async ({ page }) => {
  await page.goto(baseUrl('/cache/scrapes'))

  const table = page.getByTestId('scrape-cache-table')
  await expect(table).toBeVisible()
  await expect(page.getByTestId('scrape-cache-row')).toHaveCount(3)
  await expect(table).toContainText('example.com/fixture-one')
  await expect(table).toContainText('text/html')
  await expect(table).toContainText('not found') // the failed fetch keeps its error
  await expect(table).not.toContainText('<h1>')
})

test('the scrape cache filters by tier and by URL', async ({ page }) => {
  await page.goto(baseUrl('/cache/scrapes'))
  await expect(page.getByTestId('scrape-cache-table')).toBeVisible()

  await page.getByTestId('scrape-cache-tier').selectOption('long')
  await expect(page.getByTestId('scrape-cache-row')).toHaveCount(1)
  await expect(page.getByTestId('scrape-cache-tier-cell')).toHaveText('long')
  await page.getByTestId('scrape-cache-tier').selectOption('')

  await page.getByTestId('scrape-cache-filter').fill('example.org')
  await page.getByTestId('scrape-cache-submit').click()
  await expect(page.getByTestId('scrape-cache-row')).toHaveCount(1)

  await page.getByTestId('scrape-cache-filter').fill('zzz-no-such-host')
  await page.getByTestId('scrape-cache-submit').click()
  await expect(page.getByTestId('scrape-cache-empty')).toContainText('No cached pages match')

  await page.getByTestId('scrape-cache-clear').click()
  await expect(page.getByTestId('scrape-cache-row')).toHaveCount(3)
})

test('the scrape cache paging controls reflect a single short page', async ({ page }) => {
  await page.goto(baseUrl('/cache/scrapes'))
  await expect(page.getByTestId('scrape-cache-table')).toBeVisible()

  await expect(page.getByTestId('scrape-cache-prev')).toBeDisabled()
  await expect(page.getByTestId('scrape-cache-next')).toBeDisabled()
  await expect(page.getByTestId('scrape-cache-page')).toContainText('Page 1')
})

test('a scrape cache row opens its scrape detail', async ({ page }) => {
  await page.goto(baseUrl('/cache/scrapes'))
  await page.getByTestId('scrape-cache-filter').fill(fixtureUrl)
  await page.getByTestId('scrape-cache-submit').click()

  await page.getByTestId('scrape-cache-detail-link').click()
  await expect(page).toHaveURL(new RegExp(`/scrapes/${seeded.scrapeWithContent}$`))
  await expect(page.getByTestId('scrape-metadata')).toBeVisible()
})

test('a scrape cache row opens its provenance', async ({ page }) => {
  await page.goto(baseUrl('/cache/scrapes'))
  await page.getByTestId('scrape-cache-filter').fill(fixtureUrl)
  await page.getByTestId('scrape-cache-submit').click()

  await page.getByTestId('scrape-cache-provenance-link').click()
  await expect(page).toHaveURL(/\/provenance\?url=/)
  await expect(page.getByTestId('found-by-table')).toContainText('google')
})

test('the two cache views link to each other', async ({ page }) => {
  await page.goto(baseUrl('/cache/searches'))
  await page.getByTestId('to-scrape-cache').click()
  await expect(page.getByTestId('scrape-cache-table')).toBeVisible()

  await page.getByTestId('to-search-cache').click()
  await expect(page.getByTestId('search-cache-table')).toBeVisible()
})
