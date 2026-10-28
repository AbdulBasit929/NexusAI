import fs from 'node:fs'
import path from 'node:path'

const root = process.cwd()
const migrated = [
  'src/styles/base.css',
  'src/styles/layout.css',
  'src/styles/pages/foundations.css',
]

const checks = [
  ['raw colour', /#[\da-f]{3,8}\b|\b(?:rgb|hsl|oklch)\(/i],
  ['!important', /!important\b/],
  ['ID selector', /(^|[\s,{])#[\w-]+/m],
]

let failures = 0
for (const relative of migrated) {
  const source = fs.readFileSync(path.join(root, relative), 'utf8')
  for (const [label, pattern] of checks) {
    if (!pattern.test(source)) continue
    console.error(`${relative}: contains ${label}`)
    failures += 1
  }
  if (relative.includes('/pages/') && /\b\d*\.?\d+m?s\b/i.test(source)) {
    console.error(`${relative}: contains a duration literal; use a motion token`)
    failures += 1
  }
}

if (failures) process.exitCode = 1
else console.log(`Style contract passed for ${migrated.length} migrated files.`)
