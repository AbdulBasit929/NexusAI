import { useCallback, useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { useCaseOverview } from '../../lib/useCaseOverview.js'
import { useWorkspaceAttention } from '../../lib/workspaceAttention.js'
import { railBadges } from './railBadges.js'
import { useRail } from './appearance.js'
import { CommandPalette } from './CommandPalette.jsx'
import { Sidebar } from './Sidebar.jsx'
import { Topbar } from './Topbar.jsx'

export { railBadges }

// The frame of every page: a sidebar (the workspace, the case you are in, recent cases), a slim top bar (where you are, search, activity,
// appearance), and one scrolling region for the page. `rail` says what the page wants by default: a page with its own left column
// (Ask, the workbench) asks for the collapsed rail; the analyst's own choice, once made, is remembered for that kind of page.
export function AppShell({ caseId = '', children, rail = 'expanded', railKind = 'default' }) {
  const location = useLocation()
  const overview = useCaseOverview(caseId)
  const attention = useWorkspaceAttention()
  const [railChoice, chooseRail] = useRail(railKind, rail)
  const [navOpen, setNavOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const collapsed = railChoice === 'collapsed'

  useEffect(() => { setNavOpen(false) }, [location.pathname])
  useEffect(() => {
    function keydown(event) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); setPaletteOpen(true) }
    }
    globalThis.document.addEventListener('keydown', keydown)
    return () => globalThis.document.removeEventListener('keydown', keydown)
  }, [])
  const closePalette = useCallback(() => setPaletteOpen(false), [])

  return (
    <div className="nx-scope nx-app" data-rail={collapsed ? 'collapsed' : 'expanded'} data-nav-open={navOpen ? 'true' : 'false'}>
      <Sidebar caseId={caseId} overview={overview} attention={attention} collapsed={collapsed} onNavigate={() => setNavOpen(false)} onToggleRail={() => chooseRail(collapsed ? 'expanded' : 'collapsed')} />
      {navOpen ? <div className="nx-scrim" onClick={() => setNavOpen(false)} aria-hidden="true" /> : null}
      <Topbar pathname={location.pathname} attention={attention} onOpenNav={() => setNavOpen(true)} onOpenPalette={() => setPaletteOpen(true)} />
      <div className="nx-main" key={caseId ? undefined : 'global'}>{children}</div>
      <CommandPalette open={paletteOpen} onClose={closePalette} caseId={caseId} />
    </div>
  )
}

export const CaseShell = AppShell
