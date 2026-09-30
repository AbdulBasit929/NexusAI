import fs from 'node:fs'
import path from 'node:path'

const fixtureDirectories = {
  'golden-v1-legacy-20260923': path.resolve(process.cwd(), '../../reports/golden-ir-20260923-final/raw'),
  'clarification-v2-legacy-20260923': path.resolve(process.cwd(), '../../reports/clarification-options-20260923/fixtures'),
}

const refreshedDirectories = [
  path.resolve(process.cwd(), 'fixtures/live-20260924-retry-3/raw'),
  path.resolve(process.cwd(), 'fixtures/live-20260924-retry-2/raw'),
  path.resolve(process.cwd(), 'fixtures/live-20260924-retry-1/raw'),
  path.resolve(process.cwd(), 'fixtures/live-20260924/raw'),
]

function loadFixtureSet(fixtureSet) {
  const fixtureDirectory = fixtureDirectories[fixtureSet]
  return fs.readdirSync(fixtureDirectory)
    .filter(name => name.endsWith('.json'))
    .sort()
    .map(name => ({
      name,
      fixtureSet,
      response: JSON.parse(fs.readFileSync(path.join(fixtureDirectory, name), 'utf8')),
    }))
}

export function loadGoldenFixtures() {
  return loadFixtureSet('golden-v1-legacy-20260923')
}

export function loadClarificationFixtures() {
  return loadFixtureSet('clarification-v2-legacy-20260923')
}

export function loadRefreshedFixtures() {
  const names = [...new Set(refreshedDirectories.flatMap(directory => fs.existsSync(directory)
    ? fs.readdirSync(directory).filter(name => name.endsWith('.json'))
    : []))].sort()
  return names.flatMap(name => {
    for (const directory of refreshedDirectories) {
      const file = path.join(directory, name)
      if (!fs.existsSync(file)) continue
      const response = JSON.parse(fs.readFileSync(file, 'utf8'))
      if (!response?._error) return [{ name, fixtureSet: 'live-20260924-refreshed', response }]
    }
    return []
  })
}

export function refreshedFixture(name) {
  return loadRefreshedFixtures().find(item => item.name === name)?.response
}

export function goldenFixture(name) {
  return loadGoldenFixtures().find(item => item.name === name)?.response
}

export function clarificationFixture(name) {
  return loadClarificationFixtures().find(item => item.name === name)?.response
}
