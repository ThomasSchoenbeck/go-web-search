/**
 * Ids written by seedTestData in testsupport.go. Fixed UUIDv7-shaped values so
 * specs can deep-link without first scraping them out of the page.
 */

export const seeded = {
  runId: '00000000-0000-7000-8000-000000000001',
  /** google, typed, 200, with stored SERP HTML. */
  searchWithSerp: '00000000-0000-7000-8000-000000000002',
  /** bing, blocked with an error, no SERP stored — the 404 path. */
  searchWithoutSerp: '00000000-0000-7000-8000-00000000000c',
  /** 200, has text, clean and raw HTML plus one image. */
  scrapeWithContent: '00000000-0000-7000-8000-000000000008',
  /** 404 with an error, no content and no images. */
  scrapeFailed: '00000000-0000-7000-8000-00000000000d',
} as const
