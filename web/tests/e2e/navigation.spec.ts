/** T027 — the navigation shell: reachable, correctly marked, history-aware. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'
import { seeded } from './fixtures-data'

const sections = [
  'runs',
  'provenance',
  'facts',
  'explore',
  'projection',
  'jobs',
  'cache',
  'logs',
  'stats',
] as const

test('every nav entry is reachable from every page', async ({ page }) => {
  // Start from a deep view, not the home page, to prove the nav is in the shell.
  await page.goto(baseUrl(`/scrapes/${seeded.scrapeWithContent}`))

  for (const section of sections) {
    await page.getByTestId(`nav-${section}`).click()
    await expect(page.getByTestId(`nav-${section}`)).toHaveAttribute('aria-current', 'page')
  }
})

test('the active entry follows nested routes', async ({ page }) => {
  await page.goto(baseUrl(`/runs/${seeded.runId}/causality`))
  await expect(page.getByTestId('nav-runs')).toHaveAttribute('aria-current', 'page')
  await expect(page.getByTestId('nav-provenance')).not.toHaveAttribute('aria-current', 'page')

  await page.goto(baseUrl('/provenance'))
  await expect(page.getByTestId('nav-provenance')).toHaveAttribute('aria-current', 'page')
})

test('the active entry follows back and forward', async ({ page }) => {
  await page.goto(baseUrl('/runs'))
  await page.getByTestId('nav-provenance').click()
  await expect(page.getByTestId('nav-provenance')).toHaveAttribute('aria-current', 'page')

  await page.goBack()
  await expect(page.getByTestId('nav-runs')).toHaveAttribute('aria-current', 'page')

  await page.goForward()
  await expect(page.getByTestId('nav-provenance')).toHaveAttribute('aria-current', 'page')
})

test('nav entries are real links, not click handlers', async ({ page, context }) => {
  await page.goto(baseUrl('/runs'))

  // A modified click must still open a new tab — the router leaves those alone.
  const opened = context.waitForEvent('page')
  await page.getByTestId('nav-provenance').click({ modifiers: ['ControlOrMeta'] })
  const newTab = await opened
  await expect(newTab).toHaveURL(/\/provenance$/)
  await newTab.close()

  // …and the original tab did not navigate.
  await expect(page).toHaveURL(/\/runs$/)
})

test('the home link returns to runs', async ({ page }) => {
  await page.goto(baseUrl('/provenance'))
  await page.getByTestId('nav-home').click()
  await expect(page.getByRole('heading', { name: 'Runs', level: 1 })).toBeVisible()
})
