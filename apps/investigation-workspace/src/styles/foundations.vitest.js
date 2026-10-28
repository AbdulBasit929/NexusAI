import fs from 'node:fs'
import path from 'node:path'
import zlib from 'node:zlib'
import { describe, expect, it } from 'vitest'
import report from '../../design/tokens.generated.json'

const cwd = process.cwd()
const read = file => fs.readFileSync(path.join(cwd, file), 'utf8')

describe('U1 foundation contract', () => {
  it('ships only passing generated contrast pairs', () => {
    expect(report.summary).toEqual({ checked: 160, passed: 160, failed: 0 })
    expect(report.contrastPairs.every(pair => pair.pass)).toBe(true)
  })

  it('maps every legacy analyst token through the alias layer', () => {
    const legacyFiles = ['tokens.css', 'workspace.css', 'identity-a.css', 'shell.css', 'canvas.css', 'connections.css', 'workbench.css', 'palette-options.css']
    const used = new Set(legacyFiles.flatMap(file => [...read(`src/styles/${file}`).matchAll(/--analyst-[\w-]+/g)].map(match => match[0])))
    const aliases = new Set([...read('src/styles/legacy-aliases.css').matchAll(/(--analyst-[\w-]+)\s*:/g)].map(match => match[1]))
    expect([...used].filter(name => !aliases.has(name))).toEqual([])
    expect(aliases.size).toBe(150)
    expect([...aliases].every(name => new RegExp(`${name.replaceAll('-', '\\-')}:\\s*var\\(--nx-`).test(read('src/styles/legacy-aliases.css')))).toBe(true)
  })

  it('declares the cascade once and places legacy styles in a bounded layer', () => {
    const index = read('src/styles/index.css')
    expect(index).toContain('@layer reset, tokens, base, layout, components, patterns, pages, utilities;')
    for (const file of ['workspace.css', 'identity-a.css', 'shell.css', 'canvas.css', 'connections.css']) {
      expect(index).toContain(`@import './${file}' layer(pages.legacy);`)
    }
  })

  it('keeps migrated page CSS free of raw colours, important rules, IDs and duration literals', () => {
    const migrated = ['src/styles/layout.css', 'src/styles/pages/foundations.css'].map(read).join('\n')
    expect(migrated).not.toMatch(/#[\da-f]{3,8}\b|\b(?:rgb|hsl|oklch)\(/i)
    expect(migrated).not.toMatch(/!important\b/)
    expect(migrated).not.toMatch(/(^|[\s,{])#[\w-]+/m)
    expect(migrated).not.toMatch(/\b\d*\.?\d+m?s\b/i)
  })

  it('keeps the foundation route CSS within the 30 KB gzip chunk budget', () => {
    const files = ['src/styles/tokens.generated.css', 'src/styles/legacy-aliases.css', 'src/styles/base.css', 'src/styles/layout.css', 'src/styles/pages/foundations.css']
    const source = files.map(read).join('\n')
    expect(zlib.gzipSync(source).byteLength).toBeLessThanOrEqual(30 * 1024)
  })
})
