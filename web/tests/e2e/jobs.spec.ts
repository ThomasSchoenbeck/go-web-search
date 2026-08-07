/** T015 — the jobs monitor: the queue, its filters, and the polling controls. */

import { test, expect } from '@playwright/test'
import { baseUrl } from './fixtures'

test('the monitor lists the seeded queue with its metadata', async ({ page }) => {
  await page.goto(baseUrl('/jobs'))

  const table = page.getByTestId('jobs-table')
  await expect(table).toBeVisible()
  await expect(page.getByTestId('job-row')).toHaveCount(4)
  await expect(table).toContainText('distill')
  await expect(table).toContainText('embed')
  await expect(table).toContainText('cleanup')
  // The running job is the one holding a lock.
  await expect(table).toContainText('running')
})

test('the breakdown covers every status', async ({ page }) => {
  await page.goto(baseUrl('/jobs'))

  for (const status of ['pending', 'running', 'done', 'failed']) {
    await expect(page.getByTestId(`jobs-count-${status}`)).toContainText(`${status}: 1`)
  }
})

test('the status and type filters narrow the list', async ({ page }) => {
  await page.goto(baseUrl('/jobs'))
  await expect(page.getByTestId('jobs-table')).toBeVisible()

  await page.getByTestId('jobs-status').selectOption('failed')
  await expect(page.getByTestId('job-row')).toHaveCount(1)
  await expect(page.getByTestId('job-status')).toHaveText('failed')

  await page.getByTestId('jobs-type').selectOption('distill')
  await expect(page.getByTestId('jobs-empty')).toContainText('No jobs match')

  // The breakdown is the whole queue, so filtering must not move it.
  await expect(page.getByTestId('jobs-count-failed')).toContainText('failed: 1')

  await page.getByTestId('jobs-status').selectOption('')
  await page.getByTestId('jobs-type').selectOption('')
  await expect(page.getByTestId('job-row')).toHaveCount(4)
})

test('the paging controls reflect a single short page', async ({ page }) => {
  await page.goto(baseUrl('/jobs'))
  await expect(page.getByTestId('jobs-table')).toBeVisible()

  await expect(page.getByTestId('jobs-prev')).toBeDisabled()
  await expect(page.getByTestId('jobs-next')).toBeDisabled()
  await expect(page.getByTestId('jobs-page')).toContainText('Page 1')
})

test('polling starts off, and the toggle and interval dropdown drive it', async ({ page }) => {
  await page.goto(baseUrl('/jobs'))
  await expect(page.getByTestId('jobs-table')).toBeVisible()

  // config.toml ships poll_enabled = false, so the view must start paused.
  await expect(page.getByTestId('poll-status')).toContainText('paused')

  await page.getByTestId('poll-interval').selectOption('1000')
  await page.getByTestId('poll-toggle').click()
  await expect(page.getByTestId('poll-status')).toContainText('refreshing every 1.0s')

  // A tick must actually re-request the list.
  await page.waitForResponse((r) => r.url().includes('/api/jobs') && r.request().method() === 'GET')

  await page.getByTestId('poll-toggle').click()
  await expect(page.getByTestId('poll-status')).toContainText('paused')
})

test('leaving the view stops the poll', async ({ page }) => {
  await page.goto(baseUrl('/jobs'))
  await page.getByTestId('poll-interval').selectOption('1000')
  await page.getByTestId('poll-toggle').click()
  await expect(page.getByTestId('poll-status')).toContainText('refreshing')

  await page.getByTestId('nav-stats').click()
  await expect(page.getByTestId('stats-counts')).toBeVisible()

  let polled = false
  page.on('request', (request) => {
    if (request.url().includes('/api/jobs')) polled = true
  })
  await page.waitForTimeout(2500)
  expect(polled).toBe(false)
})
