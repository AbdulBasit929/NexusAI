import { Activity, Database, Folders, LayoutDashboard, MessageSquareText, Network, PanelsTopLeft, Gauge, Milestone } from 'lucide-react'

// One list of places, used by the sidebar, the breadcrumb and the command palette, so they can never disagree.
export const workspaceNav = [
  { id: 'dashboard', label: 'Dashboard', to: '/', end: true, icon: LayoutDashboard },
  { id: 'across', label: 'Ask across cases', to: '/investigate', icon: MessageSquareText },
  { id: 'cases', label: 'Cases', to: '/cases', icon: Folders },
  { id: 'activity', label: 'Activity', to: '/activity', icon: Activity },
]

const caseSections = [
  { id: 'overview', label: 'Overview', path: 'overview', icon: PanelsTopLeft },
  { id: 'evidence', label: 'Evidence', path: 'evidence', icon: Database },
  { id: 'ask', label: 'Ask', path: 'investigate', icon: MessageSquareText },
  { id: 'connections', label: 'Connections', path: 'connections', icon: Network },
  { id: 'timeline', label: 'Timeline', path: 'timeline', icon: Milestone },
  { id: 'activity', label: 'Activity', path: 'activity', icon: Activity },
]

// Cases with a local data inventory (public/runtime-config.js, dataInventoryCaseIds) get a Data section.
const dataCaseIds = () => globalThis.__INVESTIGATION_WORKSPACE_CONFIG__?.dataInventoryCaseIds || []

export function caseNav(caseId) {
  const base = `/cases/${encodeURIComponent(caseId)}`
  const sections = caseSections.concat(dataCaseIds().includes(caseId) ? [{ id: 'data', label: 'Data', path: 'data', icon: Gauge }] : [])
  return sections.map(section => ({ ...section, to: `${base}/${section.path}` }))
}

const sectionLabel = Object.fromEntries(caseSections.map(section => [section.path, section.label]).concat([['data', 'Data']]))

// The trail for the top bar: global places are one item, a place inside a case is Cases / <case> / <section> (/ <item>).
export function trailFor(pathname) {
  const parts = pathname.replace(/\/+$/, '').split('/').filter(Boolean).map(part => { try { return decodeURIComponent(part) } catch { return part } })
  if (!parts.length) return [{ label: 'Dashboard', to: '/' }]
  if (parts[0] === 'investigate') return [{ label: 'Ask across cases', to: '/investigate' }]
  if (parts[0] === 'activity') return [{ label: 'Activity', to: '/activity' }]
  if (parts[0] === 'settings') return [{ label: 'Settings', to: '/settings' }]
  if (parts[0] === 'design') return [{ label: 'Design', to: '/design/language' }, { label: parts.slice(1).join(' / ') || 'Language' }]
  if (parts[0] !== 'cases') return [{ label: 'NexusAI', to: '/' }]
  if (parts.length === 1) return [{ label: 'Cases', to: '/cases' }]
  if (parts[1] === 'new') return [{ label: 'Cases', to: '/cases' }, { label: 'New case' }]
  const caseId = parts[1]
  const base = `/cases/${encodeURIComponent(caseId)}`
  const trail = [{ label: 'Cases', to: '/cases' }, { caseId, to: `${base}/overview` }]
  if (parts[2]) trail.push({ label: sectionLabel[parts[2]] || parts[2], to: `${base}/${parts[2]}` })
  if (parts[3]) trail.push({ label: parts[2] === 'evidence' ? 'File' : parts[3] })
  return trail
}
