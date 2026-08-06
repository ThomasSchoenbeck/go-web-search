/**
 * Playwright global setup: build the Go binary, start it against a throwaway
 * main + log database on a free port, and tear all of it down afterwards.
 *
 * The isolation rule this enforces (see plans/observability-ui/T024): no test
 * ever touches the developer's ./data. The temp directory is created and
 * removed by *this* process, not by the Go server, so the data is cleaned up
 * even when the server is killed rather than shut down — pass, fail or
 * interrupt.
 */

import { spawn, spawnSync, type ChildProcess } from 'node:child_process'
import { createServer } from 'node:net'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

const repoRoot = resolve(import.meta.dirname, '..', '..', '..')

/** The running server's origin. Set by globalSetup, inherited by workers. */
export function baseUrl(path = '/'): string {
  const base = process.env.E2E_BASE_URL
  if (!base) throw new Error('E2E_BASE_URL is unset — Playwright globalSetup did not run')
  return new URL(path, base).toString()
}

async function freePort(): Promise<number> {
  return new Promise((resolvePort, reject) => {
    const probe = createServer()
    probe.on('error', reject)
    probe.listen(0, '127.0.0.1', () => {
      const address = probe.address()
      if (address === null || typeof address === 'string') {
        probe.close()
        reject(new Error('could not determine a free port'))
        return
      }
      const { port } = address
      probe.close(() => resolvePort(port))
    })
  })
}

async function waitForHealth(url: string, child: ChildProcess, timeoutMs = 60_000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`test server exited early with code ${child.exitCode}`)
    }
    try {
      const response = await fetch(url)
      if (response.ok) return
    } catch {
      // Not listening yet.
    }
    await new Promise((r) => setTimeout(r, 200))
  }
  throw new Error(`test server did not become healthy within ${timeoutMs}ms`)
}

export default async function globalSetup(): Promise<() => Promise<void>> {
  // Build once so the harness runs a real binary. `go run` would fork a child
  // the harness could not reliably kill on Windows, leaving the temp databases
  // locked and undeletable.
  const binary = join(repoRoot, 'harvester-e2e.exe')
  const build = spawnSync('go', ['build', '-o', binary, '.'], { cwd: repoRoot, encoding: 'utf8' })
  if (build.status !== 0) {
    throw new Error(`go build failed:\n${build.stderr || build.stdout}`)
  }

  const dataDir = await mkdtemp(join(tmpdir(), 'harvester-e2e-'))
  const port = await freePort()

  const child = spawn(binary, ['-mode', 'testserve', '-data', dataDir, '-port', String(port)], {
    cwd: repoRoot,
    env: { ...process.env, HARVESTER_TEST_SEED: '1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  const log: string[] = []
  child.stdout?.on('data', (chunk: Buffer) => log.push(chunk.toString()))
  child.stderr?.on('data', (chunk: Buffer) => log.push(chunk.toString()))

  const cleanup = async (): Promise<void> => {
    if (child.exitCode === null) {
      child.kill()
      // Give the process a moment to release the database file handles before
      // trying to remove the directory.
      await new Promise((r) => setTimeout(r, 500))
    }
    await rm(dataDir, { recursive: true, force: true, maxRetries: 5 })
    await rm(binary, { force: true, maxRetries: 5 })
  }

  try {
    await waitForHealth(`http://127.0.0.1:${port}/healthz`, child)
  } catch (error) {
    await cleanup()
    throw new Error(`${(error as Error).message}\nserver output:\n${log.join('')}`)
  }

  process.env.E2E_BASE_URL = `http://127.0.0.1:${port}`
  process.env.E2E_DATA_DIR = dataDir
  return cleanup
}
