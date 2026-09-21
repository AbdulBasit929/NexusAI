export const CASE_WORKSPACE_MODULES = [
  { id: 'overview', label: 'Overview', icon: 'fa-gauge-high', description: 'Readiness, inventory, and investigative posture' },
  { id: 'ask', label: 'Ask', icon: 'fa-magnifying-glass-chart', description: 'Deterministic questions and cited analysis' },
  { id: 'evidence', label: 'Evidence', icon: 'fa-box-archive', description: 'Source registry, integrity, and provenance' },
  { id: 'relationships', label: 'Relationships', icon: 'fa-diagram-project', description: 'Evidence-backed networks and correlations' },
  { id: 'timeline', label: 'Timeline', icon: 'fa-timeline', description: 'Chronology, movement, and temporal activity' },
  { id: 'media', label: 'Media', icon: 'fa-photo-film', description: 'Registered visual, audio, and video evidence' },
  { id: 'reports', label: 'Reports', icon: 'fa-file-lines', description: 'On-demand deterministic case reporting' },
  { id: 'admin', label: 'Admin', icon: 'fa-shield-halved', description: 'Processing, retention, and case controls' },
]

export const CASE_WORKSPACE_COMPATIBILITY = {
  analyze: 'ask',
  jobs: 'admin',
  settings: 'admin',
}
