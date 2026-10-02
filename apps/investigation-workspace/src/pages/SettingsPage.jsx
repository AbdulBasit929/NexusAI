import { useEffect, useState } from 'react'
import { AppShell } from '../components/CaseShell.jsx'
import { RouteState } from '../components/AnalystComponents.jsx'
import { useThemePreference } from '../components/ThemeControl.jsx'
import { configuredCaseIds } from '../lib/apiClient.js'
import { clearStorageKeys, clearStoredQuestions, localStorageFootprint, storageExport, storageInventory } from '../lib/workspaceState.js'
import { formatNumber } from '../lib/format.js'
import { AppearancePicker, BoundaryCard, ExportButton, InventoryTable, ShortcutsCard, formatSize } from './settings/SettingsParts.jsx'

function itemLabel(count, singular, plural = `${singular}s`) {
  return `${formatNumber(count)} ${count === 1 ? singular : plural}`
}

const SECTIONS = [['st-appearance', 'Appearance'], ['st-data', 'Browser data'], ['st-shortcuts', 'Shortcuts'], ['st-about', 'About these settings']]

// Settings: a section index on the left (sticky, follows the scroll), then cards on one 16px rhythm. Appearance previews each
// theme. Browser data lists what is kept, by kind, with a clear for each and a copy to download; clearing questions and
// drafts keeps its reviewed confirmation. Shortcuts is a reference of what actually works where. Everything here is stored
// in this browser only, and the page says so; there is no identity, role or connection information because none is real.
export default function SettingsPage() {
  const { choose } = useThemePreference()
  const caseCount = configuredCaseIds().length
  const [footprint, setFootprint] = useState(() => localStorageFootprint())
  const [inventory, setInventory] = useState(() => storageInventory())
  const [confirming, setConfirming] = useState(false)
  const [status, setStatus] = useState('')
  const [active, setActive] = useState(SECTIONS[0][0])
  const workingItems = footprint.questions + footprint.drafts
  const refresh = () => { setFootprint(localStorageFootprint()); setInventory(storageInventory()) }

  useEffect(() => {
    if (!globalThis.IntersectionObserver) return undefined
    const observer = new globalThis.IntersectionObserver(entries => {
      const visible = entries.filter(entry => entry.isIntersecting).sort((left, right) => left.boundingClientRect.top - right.boundingClientRect.top)[0]
      if (visible) setActive(visible.target.id)
    }, { rootMargin: '-20% 0px -65% 0px' })
    for (const [id] of SECTIONS) { const node = globalThis.document.getElementById(id); if (node) observer.observe(node) }
    return () => observer.disconnect()
  }, [])

  function clearNow() {
    clearStoredQuestions()
    refresh()
    setConfirming(false)
    setStatus('Question history and drafts were cleared from this browser.')
  }
  function clearRow(row) {
    if (row.id === 'appearance') choose('system')
    clearStorageKeys(row.keys)
    refresh()
    setStatus(`${row.label} were cleared from this browser.`)
  }
  function exportNow() {
    const url = URL.createObjectURL(new Blob([storageExport()], { type: 'application/json' }))
    const link = globalThis.document.createElement('a')
    link.href = url; link.download = 'nexusai-browser-data.json'
    globalThis.document.body.append(link); link.click(); link.remove()
    globalThis.setTimeout(() => URL.revokeObjectURL(url), 1000)
  }

  return (
    <AppShell>
      <main id="workspace-main" className="catalog-page settings-page st" tabIndex={-1}>
        <header className="st-head">
          <h1>Settings</h1>
          <p>Control how this browser displays and remembers your working session.</p>
        </header>

        <div className="st-layout">
          <nav className="st-nav" aria-label="Settings sections">
            <ul>{SECTIONS.map(([id, label]) => <li key={id}><a href={`#${id}`} aria-current={active === id ? 'true' : undefined} onClick={() => setActive(id)}>{label}</a></li>)}</ul>
            <p>Everything here is saved in this browser only.</p>
          </nav>

          <div className="st-main">
            <section id="st-appearance" className="st-card st-span" aria-labelledby="appearance-title">
              <header><h2 id="appearance-title">Appearance</h2><small>This browser · applies immediately</small></header>
              <AppearancePicker />
            </section>

            <section id="st-data" className="st-card st-span" aria-labelledby="browser-data-title">
              <header><h2 id="browser-data-title">Remembered in this browser</h2><small>Not part of the case record</small></header>
              {footprint.available ? (
                <>
                  <dl className="st-stats" aria-label="Browser-local working data">
                    <div><dt>Saved questions</dt><dd>{formatNumber(footprint.questions)}</dd></div>
                    <div><dt>Unfinished drafts</dt><dd>{formatNumber(footprint.drafts)}</dd></div>
                    <div><dt>Cases with local work</dt><dd>{formatNumber(footprint.cases)}</dd></div>
                    <div><dt>Stored in total</dt><dd>{formatSize(inventory.bytes)}</dd></div>
                  </dl>

                  <InventoryTable inventory={inventory} onClearQuestions={() => { setConfirming(true); setStatus('') }} onClear={clearRow} />

                  {workingItems > 0 ? (
                    <div className="st-action">
                      <div>
                        <h3>Question history and drafts</h3>
                        <p>Remove {itemLabel(footprint.questions, 'saved question')} and {itemLabel(footprint.drafts, 'unfinished draft')} from this browser. Appearance and navigation preferences will stay as they are.</p>
                      </div>
                      {!confirming ? <button type="button" className="st-btn" onClick={() => { setConfirming(true); setStatus('') }}>Clear question history and drafts…</button> : null}
                    </div>
                  ) : (
                    <RouteState state="empty" label="No local questions or drafts" description="Nothing from your investigation working sessions is stored in this browser. Appearance and navigation preferences may still be remembered." />
                  )}

                  {confirming ? (
                    <div className="st-confirm" role="group" aria-labelledby="clear-browser-data-title">
                      <div>
                        <h3 id="clear-browser-data-title">Clear {formatNumber(workingItems)} local {workingItems === 1 ? 'item' : 'items'}?</h3>
                        <p>This removes the saved questions and drafts listed above. Case evidence and server-side findings are not affected.</p>
                      </div>
                      <div className="st-confirm__actions">
                        <button type="button" className="st-danger" onClick={clearNow}>Clear from this browser</button>
                        <button type="button" className="st-btn" onClick={() => setConfirming(false)}>Cancel</button>
                      </div>
                    </div>
                  ) : null}
                  <div className="st-foot"><ExportButton onExport={exportNow} /><p className="st-status" role="status" aria-live="polite">{status}</p></div>
                </>
              ) : (
                <RouteState state="unavailable" label="Browser storage is unavailable" description="The workspace still works in this tab, but preferences, questions and drafts may reset when it closes." />
              )}
            </section>

            <div id="st-shortcuts" className="st-pair">
              <ShortcutsCard />
              <div id="st-about" className="st-about"><BoundaryCard caseCount={caseCount} /></div>
            </div>
          </div>
        </div>
      </main>
    </AppShell>
  )
}
