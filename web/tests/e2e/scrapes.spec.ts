/** T008 — scrape detail: metadata, the raw/clean/text toggle, and images. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'
import { seeded } from './fixtures-data'

test('shows every fetch-metadata field', async ({ page }) => {
  await page.goto(baseUrl(`/scrapes/${seeded.scrapeWithContent}`))

  const table = page.getByTestId('scrape-metadata')
  await expect(table).toBeVisible()
  await expect(table).toContainText('https://example.com/fixture-one')
  await expect(table).toContainText('text/html')
  await expect(table).toContainText('fixturehash')
  await expect(table).toContainText('W/"fixture-etag"')
  await expect(table).toContainText('short')
  await expect(table).toContainText('Hit count')
  await expect(table).toContainText('Last modified')
})

test('the raw HTML is fetched only when its tab is opened', async ({ page }) => {
  const rawRequests: string[] = []
  page.on('request', (request) => {
    if (request.url().includes('raw=1')) rawRequests.push(request.url())
  })

  await page.goto(baseUrl(`/scrapes/${seeded.scrapeWithContent}`))
  await expect(page.getByTestId('scrape-text')).toBeVisible()
  expect(rawRequests).toHaveLength(0)

  await page.getByTestId('tab-raw').click()
  await expect(page.getByTestId('raw-frame')).toBeVisible()
  expect(rawRequests).toHaveLength(1)
})

test('the text, clean and raw tabs each render', async ({ page }) => {
  await page.goto(baseUrl(`/scrapes/${seeded.scrapeWithContent}`))

  await expect(page.getByTestId('scrape-text')).toContainText('Fixture One')

  await page.getByTestId('tab-clean').click()
  const cleanFrame = page.getByTestId('clean-frame')
  await expect(cleanFrame).toHaveAttribute('sandbox', '')
  await expect(page.frameLocator('[data-testid="clean-frame"]').locator('h1')).toContainText('Fixture One')

  await page.getByTestId('tab-raw').click()
  const rawFrame = page.getByTestId('raw-frame')
  await expect(rawFrame).toHaveAttribute('sandbox', '')
  await expect(page.getByTestId('raw-size')).toContainText('characters')

  await page.getByTestId('tab-text').click()
  await expect(page.getByTestId('scrape-text')).toBeVisible()
})

test('stored images render with their dimensions', async ({ page }) => {
  await page.goto(baseUrl(`/scrapes/${seeded.scrapeWithContent}`))

  const images = page.getByTestId('images-list')
  await expect(images).toBeVisible()
  await expect(images).toContainText('320×200')
  // Remote image bytes were never stored, so the link out is the useful part.
  await expect(images.getByRole('link').first()).toHaveAttribute('target', '_blank')
})

test('a failed scrape renders its error and empty states', async ({ page }) => {
  await page.goto(baseUrl(`/scrapes/${seeded.scrapeFailed}`))

  await expect(page.getByTestId('scrape-metadata')).toContainText('not found')
  await expect(page.getByTestId('scrape-metadata')).toContainText('404')
  await expect(page.getByTestId('text-empty')).toBeVisible()
  await expect(page.getByTestId('images-empty')).toBeVisible()

  await page.getByTestId('tab-clean').click()
  await expect(page.getByTestId('clean-empty')).toBeVisible()

  await page.getByTestId('tab-raw').click()
  await expect(page.getByTestId('raw-empty')).toBeVisible()
})

test('a missing scrape id shows a not-found state', async ({ page }) => {
  await page.goto(baseUrl('/scrapes/does-not-exist'))
  await expect(page.getByTestId('scrape-missing')).toBeVisible()
})

test('the run link returns to the run that produced the scrape', async ({ page }) => {
  await page.goto(baseUrl(`/scrapes/${seeded.scrapeWithContent}`))
  await page.getByTestId('scrape-run-link').click()

  await expect(page).toHaveURL(new RegExp(`/runs/${seeded.runId}$`))
  await expect(page.getByTestId('run-summary')).toBeVisible()
})
