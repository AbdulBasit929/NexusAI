const ROUTE_TITLES = {
  '': 'Workspace',
  account: 'Account',
  agents: 'Agents',
  backends: 'Backends',
  chat: 'Chat',
  collections: 'Knowledge',
  face: 'Face Recognition',
  'fine-tune': 'Fine-Tune',
  manage: 'Host',
  middleware: 'Middleware',
  models: 'Model Library',
  nodes: 'Nodes',
  p2p: 'Swarm',
  quantize: 'Quantize',
  records: 'Cases',
  scheduling: 'Scheduling',
  settings: 'Settings',
  skills: 'Skills',
  studio: 'Studio',
  talk: 'Talk',
  traces: 'Traces',
  usage: 'Usage',
  users: 'Users',
  voice: 'Voice Recognition',
}

const ADMIN_ROUTES = new Set([
  'backends', 'manage', 'middleware', 'models', 'nodes', 'p2p', 'scheduling',
  'settings', 'traces', 'usage', 'users',
])

const INTELLIGENCE_ROUTES = new Set([
  'agent-jobs', 'agents', 'collections', 'face', 'fine-tune', 'quantize',
  'records', 'skills', 'voice',
])

export function getShellRouteContext(pathname, activeCase = null) {
  const segments = pathname.replace(/^\/app\/?/, '').split('/').filter(Boolean)
  const section = segments[0] || ''
  const caseContext = activeCase?.caseId ? { caseId: activeCase.caseId } : {}

  if (section === 'cases' && segments[1]) {
    return {
      area: 'Cases',
      title: 'Case Workspace',
      ...caseContext,
    }
  }

  if (ADMIN_ROUTES.has(section)) {
    return { area: 'System Administration', title: ROUTE_TITLES[section] || 'Administration', ...caseContext }
  }

  if (INTELLIGENCE_ROUTES.has(section)) {
    return { area: 'Intelligence Tools', title: ROUTE_TITLES[section] || 'Intelligence', ...caseContext }
  }

  if (section === 'account') {
    return { area: 'Personal workspace', title: ROUTE_TITLES[section], ...caseContext }
  }

  return {
    area: section ? 'Analyst Tools' : 'NexusAI',
    title: ROUTE_TITLES[section] || 'Workspace',
    ...caseContext,
  }
}
