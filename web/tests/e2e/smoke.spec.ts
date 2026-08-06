/**
 * Harness smoke test: the embedded SPA loads from the Go binary, the read layer
 * reaches the API, and every interactive element on the page is exercised —
 * the standard T024 sets for every page in this plan.
 */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'

test('serves the embedded SPA shell', async ({ page }) => {
  await page.goto(baseUrl('/'))
  await expect(page.getByRole('heading', { name: 'Observability UI', level: 1 })).toBeVisible()
})

test('client-side routes fall back to the app', async ({ page }) => {
  await page.goto(baseUrl('/runs/123'))
  await expect(page.getByRole('heading', { name: 'Observability UI', level: 1 })).toBeVisible()
})

test('loads settings and stats through the read layer', async ({ page }) => {
  await page.goto(baseUrl('/'))

  await expect(page.getByTestId('settings')).toContainText('poll interval')
  await expect(page.getByTestId('settings-error')).toHaveCount(0)

  // Seeded by the fixture: one search, two URLs, one scrape, one fact. `runs`
  // is deliberately not asserted — every mode records a run row of its own, so
  // the count includes the testserve process itself.
  const stats = page.getByTestId('stats')
  await expect(stats).toContainText('"searches": 1')
  await expect(stats).toContainText('"urls": 2')
  await expect(stats).toContainText('"scrapes": 1')
  await expect(stats).toContainText('"memory_facts": 1')
})

test('exercises every interactive element', async ({ page }) => {
  await page.goto(baseUrl('/'))
  await expect(page.getByTestId('stats')).toBeVisible()

  const toggle = page.getByTestId('toggle-polling')
  const interval = page.getByTestId('interval')
  const reload = page.getByTestId('reload')

  // Config seeds polling off, so the button offers to start it.
  await expect(toggle).toBeEnabled()
  await expect(toggle).toHaveText('Start polling')
  await toggle.click()
  await expect(toggle).toHaveText('Stop polling')

  await interval.selectOption('1000')
  await expect(interval).toHaveValue('1000')
  // Polling keeps working on the new cadence rather than erroring out.
  await expect(page.getByTestId('stats-error')).toHaveCount(0)

  await toggle.click()
  await expect(toggle).toHaveText('Start polling')

  await reload.click()
  await expect(page.getByTestId('stats')).toContainText('"searches": 1')
  await expect(page.getByTestId('stats-error')).toHaveCount(0)
})

test('the API is reachable without a token', async ({ request }) => {
  const response = await request.get(baseUrl('/api/ui-config'))
  expect(response.status()).toBe(200)
  expect(await response.json()).toMatchObject({
    poll_interval_ms: expect.any(Number),
    poll_enabled: expect.any(Boolean),
    projection_sample_cap: expect.any(Number),
  })
})
