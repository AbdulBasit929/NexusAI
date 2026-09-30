import { useState } from 'react'
import { AppShell } from '../components/CaseShell.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { ThemeControl } from '../components/ThemeControl.jsx'
import { RouteState } from '../components/AnalystComponents.jsx'
import { clearStoredQuestions, localStorageFootprint } from '../lib/workspaceState.js'
import { formatNumber } from '../lib/format.js'

function itemLabel(count, singular, plural = `${singular}s`) {
  return `${formatNumber(count)} ${count === 1 ? singular : plural}`
}

export default function SettingsPage() {
  const [footprint, setFootprint] = useState(() => localStorageFootprint())
  const [confirming, setConfirming] = useState(false)
  const [status, setStatus] = useState('')
  const workingItems = footprint.questions + footprint.drafts

  function clearNow() {
    clearStoredQuestions()
    setFootprint(localStorageFootprint())
    setConfirming(false)
    setStatus('Question history and drafts were cleared from this browser.')
  }

  return (
    <AppShell>
      <main id="workspace-main" className="catalog-page settings-page" tabIndex={-1}>
        <PageHeader
          eyebrow="Workspace preferences"
          title="Settings"
          description="Control how this browser displays and remembers your working session."
          breadcrumbs={[{ label: 'Workspace', to: '/' }, { label: 'Settings' }]}
        />

        <div className="settings-stack">
          <section className="settings-section" aria-labelledby="appearance-title">
            <header className="settings-section__header">
              <div>
                <p className="section-kicker">Display</p>
                <h2 id="appearance-title">Appearance</h2>
                <p>Choose a theme for this browser. Changes apply immediately.</p>
              </div>
              <span className="settings-scope">This browser</span>
            </header>
            <ThemeControl />
          </section>

          <section className="settings-section" aria-labelledby="browser-data-title">
            <header className="settings-section__header">
              <div>
                <p className="section-kicker">Local working data</p>
                <h2 id="browser-data-title">Remembered in this browser</h2>
                <p>These items help you resume work. They are not part of the case record.</p>
              </div>
              <span className="settings-scope">This browser</span>
            </header>

            {footprint.available ? (
              <>
                <dl className="settings-summary" aria-label="Browser-local working data">
                  <div><dt>Saved questions</dt><dd>{formatNumber(footprint.questions)}</dd></div>
                  <div><dt>Unfinished drafts</dt><dd>{formatNumber(footprint.drafts)}</dd></div>
                  <div><dt>Cases with local work</dt><dd>{formatNumber(footprint.cases)}</dd></div>
                </dl>

                {workingItems > 0 ? (
                  <div className="settings-action">
                    <div>
                      <h3>Question history and drafts</h3>
                      <p>
                        Remove {itemLabel(footprint.questions, 'saved question')} and {itemLabel(footprint.drafts, 'unfinished draft')} from this browser.
                        Appearance and navigation preferences will stay as they are.
                      </p>
                    </div>
                    {!confirming ? (
                      <button type="button" onClick={() => { setConfirming(true); setStatus('') }}>Clear question history and drafts…</button>
                    ) : null}
                  </div>
                ) : (
                  <RouteState
                    state="empty"
                    label="No local questions or drafts"
                    description="Nothing from your investigation working sessions is stored in this browser. Appearance and navigation preferences may still be remembered."
                  />
                )}

                {confirming ? (
                  <div className="settings-confirm" role="group" aria-labelledby="clear-browser-data-title">
                    <div>
                      <h3 id="clear-browser-data-title">Clear {formatNumber(workingItems)} local {workingItems === 1 ? 'item' : 'items'}?</h3>
                      <p>This removes the saved questions and drafts listed above. Case evidence and server-side findings are not affected.</p>
                    </div>
                    <div className="settings-confirm__actions">
                      <button type="button" className="settings-destructive" onClick={clearNow}>Clear from this browser</button>
                      <button type="button" onClick={() => setConfirming(false)}>Cancel</button>
                    </div>
                  </div>
                ) : null}
                <p className="settings-status" role="status" aria-live="polite">{status}</p>
              </>
            ) : (
              <RouteState
                state="unavailable"
                label="Browser storage is unavailable"
                description="The workspace still works in this tab, but preferences, questions and drafts may reset when it closes."
              />
            )}
          </section>

          <aside className="settings-boundary" aria-labelledby="settings-boundary-title">
            <div>
              <p className="section-kicker">Storage boundary</p>
              <h2 id="settings-boundary-title">What these settings affect</h2>
            </div>
            <dl>
              <div><dt>Kept here</dt><dd>Theme, navigation preference, saved questions and unfinished drafts.</dd></div>
              <div><dt>Not changed</dt><dd>Case evidence, processed records, findings and source citations.</dd></div>
              <div><dt>Not shared</dt><dd>Other browsers and other analysts do not receive this local working data.</dd></div>
            </dl>
          </aside>
        </div>
      </main>
    </AppShell>
  )
}
