import { useEffect, useMemo, useRef, useState } from 'react'
import { Search } from 'lucide-react'
import { LanguageText } from '../AnalystComponents.jsx'
import { activeCueIndex, formatClock, scrollWithin } from '../../lib/viewerTools.js'

const TAG = { SEARCH_LEAD: 'Search lead', LOW_RESOURCE_LANGUAGE: 'Pointer to listen' }

// The transcript beside the player: the segment being spoken is marked and followed as it plays, a click seeks to it, the segment a citation pointed at is set
// apart, and a search narrows the list to the segments that contain the words. Each segment shows the span of the recording it covers; its text is read in its
// own language and direction. A segment whose reading is weaker than the rest of the recording says so (`overall` is the status of the recording); `stopAt`
// says where a reading that ran out of time ends.
export function TranscriptPanel({ cues, time, onSeek, cited = null, stopAt = null, overall = null }) {
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
    <div className="mx-tr">
      <div className="mx-tr__tools">
        <label className="mx-input"><Search aria-hidden="true" /><span className="visually-hidden">Search the transcript</span><input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Search the text" /></label>
        <label className="mx-check"><input type="checkbox" checked={follow} onChange={event => setFollow(event.target.checked)} />Follow playback</label>
      </div>
      <p className="mx-tr__count" role="status">{needle ? `${shown.length} of ${cues.length} segments match` : `${cues.length} ${cues.length === 1 ? 'segment' : 'segments'}`}</p>
      {shown.length ? (
        <ol className="mx-tr__list" ref={list}>
          {shown.map(({ cue, index }) => {
            const tag = cue.status && cue.status !== overall ? TAG[cue.status] : null
            return (
              <li key={cue.id} aria-current={active === index ? 'true' : undefined} className={`mx-tr__row${citedIndex === index ? ' is-cited' : ''}`}>
                <button type="button" className="mx-tr__btn" onClick={() => onSeek(cue.start)}>
                  <time className="mx-tr__time">{formatClock(cue.start)}{cue.end ? `–${formatClock(cue.end)}` : ''}</time>
                  <span className="mx-tr__body">
                    {cue.speaker ? <strong className="mx-tr__speaker">{cue.speaker}</strong> : null}
                    <LanguageText className="mx-tr__text" lang={cue.language || undefined}>{cue.text}</LanguageText>
                    {tag ? <span className="mx-tr__tag">{tag}</span> : null}
                  </span>
                </button>
              </li>
            )
          })}
          {stopAt !== null && !needle ? <li className="mx-tr__stop">Reading stopped at {formatClock(stopAt)}. The rest of the recording is not read.</li> : null}
        </ol>
      ) : <p className="viewer-gap">No segment contains those words.</p>}
    </div>
  )
}
