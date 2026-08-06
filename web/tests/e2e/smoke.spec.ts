/**
 * Harness smoke test: the embedded SPA loads from the Go binary, deep links
 * resolve through the server-side fallback, and the API answers without a
 * token. Per-view coverage lives in the sibling specs.
 */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'

test('serves the embedded SPA shell', async ({ page }) => {
  await page.goto(baseUrl('/'))
  await expect(page.getByTestId('nav-runs')).toHaveText('Observability UI')
  await expect(page.getByRole('heading', { name: 'Runs', level: 1 })).toBeVisible()
})

test('a deep link resolves through the SPA fallback', async ({ page }) => {
  // Loaded directly, not navigated to: this only works because the Go handler
  // serves index.html for unknown non-API paths.
  await page.goto(baseUrl('/runs/does-not-exist'))
  await expect(page.getByRole('heading', { name: /^Run /, level: 1 })).toBeVisible()
  await expect(page.getByTestId('run-error')).toBeVisible()
})

test('an unknown route renders the not-found view', async ({ page }) => {
  await page.goto(baseUrl('/nope/deep/path'))
  await expect(page.getByTestId('not-found')).toBeVisible()
  await page.getByTestId('not-found-home').click()
  await expect(page.getByRole('heading', { name: 'Runs', level: 1 })).toBeVisible()
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

test('an unknown API path stays a 404 instead of returning the app', async ({ request }) => {
  const response = await request.get(baseUrl('/api/bogus'))
  expect(response.status()).toBe(404)
  expect(response.headers()['content-type']).not.toContain('text/html')
})
