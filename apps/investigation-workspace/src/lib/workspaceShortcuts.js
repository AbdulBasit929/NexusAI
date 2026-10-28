export const routeShortcuts = {
  d: '/',
  q: '/investigate',
  c: '/cases',
  a: '/activity',
  s: '/settings',
}

export const shortcutSections = [
  { group: 'Everywhere', rows: [
    { keys: ['Ctrl', 'K'], what: 'Open the command palette (⌘ K on a Mac)', together: true },
    { keys: ['/'], what: 'Open workspace search' },
    { keys: ['?'], what: 'Open the shortcut sheet' },
    { keys: ['G', 'D'], what: 'Go to Dashboard', sequence: true },
    { keys: ['G', 'Q'], what: 'Go to Ask across cases', sequence: true },
    { keys: ['G', 'C'], what: 'Go to Cases', sequence: true },
    { keys: ['G', 'A'], what: 'Go to Activity', sequence: true },
    { keys: ['G', 'S'], what: 'Go to Settings', sequence: true },
    { keys: ['Esc'], what: 'Close the active drawer, panel or dialog' },
  ] },
  { group: 'Case workspace', rows: [
    { keys: ['Alt', 'S'], what: 'Switch case', together: true },
  ] },
  { group: 'Timeline', rows: [
    { keys: ['←', '→'], what: 'Previous or next day' },
    { keys: ['P'], what: 'Pin or unpin the day' },
  ] },
  { group: 'Activity', rows: [
    { keys: ['J', 'K'], what: 'Next or previous entry' },
    { keys: ['E'], what: 'Mark reviewed and move on' },
    { keys: ['↑', '↓'], what: 'Move in the journal' },
  ] },
  { group: 'Evidence viewers', rows: [
    { keys: ['Space'], what: 'Play or pause media' },
    { keys: ['←', '→'], what: 'Skip 5 seconds' },
    { keys: ['M', 'F'], what: 'Mute or enter full screen' },
    { keys: ['+', '−'], what: 'Zoom an image' },
    { keys: ['0', 'R'], what: 'Fit or rotate an image' },
  ] },
]

export function isEditableTarget(target) {
  return target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement || target?.isContentEditable
}
