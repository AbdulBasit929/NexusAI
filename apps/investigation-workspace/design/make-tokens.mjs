// Writes src/v2/styles/tokens.generated.css from design/tokens-v2.mjs and fails when a colour pair is below its ratio.
//   node design/make-tokens.mjs           write the file, then check
//   node design/make-tokens.mjs --check   check only (CI); exit 1 on any failure
import { mkdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { buildCss, checkTheme, contrast, dataLightnessGaps, themes } from './tokens-v2.mjs'

const here = dirname(fileURLToPath(import.meta.url))
const target = join(here, '..', 'src', 'v2', 'styles', 'tokens.generated.css')
const checkOnly = process.argv.includes('--check')

const failures = Object.keys(themes).flatMap(checkTheme)
for (const name of Object.keys(themes)) {
  const gaps = dataLightnessGaps(name)
  if (gaps.some(gap => gap < 0.012)) console.log(`note: two neighbouring data colours in ${name} are close in lightness (${gaps.map(gap => gap.toFixed(3)).join(', ')})`)
}

if (failures.length) {
  console.error(`${failures.length} colour pair(s) below their ratio:`)
  for (const failure of failures) console.error(`  [${failure.theme}] ${failure.what}: ${failure.fg} on ${failure.bg} = ${failure.ratio}:1 (needs ${failure.minimum}:1)`)
  process.exit(1)
}

const css = buildCss()
if (checkOnly) {
  const current = existsSync(target) ? readFileSync(target, 'utf8') : ''
  if (current !== css) { console.error('tokens.generated.css is out of date: run `npm run tokens:v2`'); process.exit(1) }
  console.log('tokens: all pairs pass and the generated file is current')
} else {
  mkdirSync(dirname(target), { recursive: true })
  writeFileSync(target, css)
  console.log(`wrote ${target}`)
  for (const name of Object.keys(themes)) console.log(`${name}: ${themes[name].text} on ${themes[name].bg} = ${contrast(themes[name].text, themes[name].bg).toFixed(1)}:1; accent ${themes[name].accent}`)
}
