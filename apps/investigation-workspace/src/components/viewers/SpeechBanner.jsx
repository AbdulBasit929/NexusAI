import { FileText, Headphones, Search } from 'lucide-react'
import { speechNote } from '../../lib/audioSegments.js'

const ICON = { SEARCH_LEAD: Search, LOW_RESOURCE_LANGUAGE: Headphones, MACHINE_TRANSCRIPT: FileText }

export function languageName(code) {
  if (!code) return ''
  try { return new Intl.DisplayNames(['en'], { type: 'language' }).of(code) || code } catch { return code }
}

// One quiet note above the player that says what the text under it is: a machine transcript to check, a search lead (find the recording by a spoken word,
// do not read it as a transcript) or a pointer to listen (the language is read poorly). It carries only the figures that were measured and recorded with
// the route, never an invented one.
export function SpeechBanner({ model }) {
  const note = speechNote(model.status, model.measured, model)
  const Icon = ICON[note.status] || FileText
  const language = languageName(model.language)
  return (
    <div className={`mx-note mx-note--${note.tone}`} role="note" aria-label={`${note.label}${language ? `, ${language}` : ''}. ${note.sentence}`}>
      <Icon className="mx-note__icon" aria-hidden="true" />
      <div>
        <p className="mx-note__title"><strong>{note.label}</strong>{language ? <span>{language}</span> : null}</p>
        <p className="mx-note__text">{note.sentence}</p>
        {note.facts.length ? <p className="mx-note__facts">{note.facts.join(' · ')}</p> : null}
        {note.incomplete ? <p className="mx-note__stop">{note.incomplete}</p> : null}
      </div>
    </div>
  )
}
