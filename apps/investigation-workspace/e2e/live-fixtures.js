import fs from 'node:fs'
import { fileURLToPath } from 'node:url'

const fixtureSets = [
  '../fixtures/live-20260924-retry-3/raw/',
  '../fixtures/live-20260924-retry-2/raw/',
  '../fixtures/live-20260924-retry-1/raw/',
  '../fixtures/live-20260924/raw/',
]

export function liveFixture(name) {
  for (const directory of fixtureSets) {
    const path = fileURLToPath(new URL(`${directory}${name}`, import.meta.url))
    if (!fs.existsSync(path)) continue
    const value = JSON.parse(fs.readFileSync(path, 'utf8'))
    if (!value?._error) return value
  }
  throw new Error(`No successful refreshed live fixture is available for ${name}`)
}
