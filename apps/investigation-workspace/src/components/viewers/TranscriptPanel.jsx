import { useEffect, useMemo, useRef, useState } from 'react'
import { Search } from 'lucide-react'
import { LanguageText } from '../AnalystComponents.jsx'
import { activeCueIndex, formatClock, scrollWithin } from '../../lib/viewerTools.js'

// The transcript beside the player: the cue being spoken is marked and followed as it plays, a click seeks to it, the cue a
// citation pointed at is set apart, and a search narrows the list to the cues that contain the words.
export function TranscriptPanel({ cues, time, onSeek, cited = null }) {
  const [query, setQuery] = useState('')
  const [follow, setFollow] = useState(true)
  const list = useRef(null)
  const active = activeCueIndex(cues, time)
  const needle = query.trim().toLocaleLowerCase()
  const shown = useMemo(() => cues.map((cue, index) => ({ cue, index })).filter(({ cue }) => !needle || `${cue.speaker || ''} ${cue.text}`.toLocaleLowerCase().includes(needle)), [cues, needle])
  const citedIndex = cited === null ? -1 : (() => { const inside = activeCueIndex(cues, cited); return inside >= 0 ? inside : cues.reduce((best, cue, index) => (cue.start <= cited ? index : best), -1) })()

  useEffect(() => {
    if (!follow || needle) return
    scrollWithin(list.current, list.current?.querySelector('[aria-current="true"]'), 'nearest', true)
  }, [active, follow, needle])
  useEffect(() => {
    if (citedIndex >= 0) scrollWithin(list.current, list.current?.querySelector('.is-cited'), 'center')
  }, [citedIndex])

  return (
    <div className="tp">
      <div className="tp__tools">
        <label className="tp__search"><Search aria-hidden="true" /><span className="visually-hidden">Search the transcript</span><input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Search the transcript" /></label>
        <label className="tp__follow"><input type="checkbox" checked={follow} onChange={event => setFollow(event.target.checked)} />Follow playback</label>
      </div>
      <p className="tp__count" role="status">{needle ? `${shown.length} of ${cues.length} cues match` : `${cues.length} cues`}</p>
      {shown.length ? (
        <ol className="transcript-cues" ref={list}>
          {shown.map(({ cue, index }) => (
            <li key={cue.id} aria-current={active === index ? 'true' : undefined} className={citedIndex === index ? 'is-cited' : undefined}>
              <button type="button" onClick={() => onSeek(cue.start)}><time>{formatClock(cue.start)}</time>{cue.speaker ? <strong>{cue.speaker}</strong> : null}<LanguageText>{cue.text}</LanguageText></button>
            </li>
          ))}
        </ol>
      ) : <p className="viewer-gap">No cue contains those words.</p>}
    </div>
  )
}
