import { useMemo } from 'react'
import { LanguageText } from '../AnalystComponents.jsx'
import { plateDiff, readsAt } from '../../lib/mediaReadings.js'
import { formatClock } from '../../lib/viewerTools.js'
import { Conf, PlateChip, RegionCrop } from './MediaMarks.jsx'

const SHOWN_READINGS = 40
const SHOWN_WORDS = 40

// Plates of a video that have no group of their own (a short clip is read frame by frame): the readings of one plate are one group.
export function groupsFromReads(groups, reads) {
  if (groups.length || !reads.length) return groups
  const byPlate = new Map()
  for (const read of reads) {
    if (read.time === null) continue
    const kept = byPlate.get(read.plate) || { id: `plate-${read.plate}`, plate: read.plate, first: read.time, last: read.time, count: 0, confidence: null, reads: [], box: read.box }
    kept.first = Math.min(kept.first, read.time)
    kept.last = Math.max(kept.last, read.time)
    kept.count += 1
    kept.confidence = Math.max(kept.confidence ?? 0, read.confidence ?? 0)
    kept.reads.push(read)
    byPlate.set(read.plate, kept)
  }
  return [...byPlate.values()].sort((a, b) => a.first - b.first)
}

// The plates read in a video, beside the player. On top, the plate on the screen at the playing second: its ORIGINAL region, cut from the frame itself and not
// redrawn, and the reading. Under it, one row per plate: where in the video it is seen, how many frames agree and how sure the reading is (a click goes to the
// first second); the readings behind a plate can be opened, each its own click, with the characters that differ from the settled plate marked. The words read
// in the frames follow. A plate is a model reading to check by eye, not an identification.
export function VideoPlates({ groups, texts, time, onSeek, media }) {
  const reads = useMemo(() => groups.flatMap(group => (group.reads || []).map(read => ({ ...read, group }))), [groups])
  const now = readsAt(reads, time, 0.6, () => 'one')[0] || null
  return (
    <section className="mx-readings" aria-labelledby="mx-plates-title">
      <div>
        <h3 className="mx-title" id="mx-plates-title">Plates read<span>{groups.length}</span></h3>
        <p className="mx-readings__note">Machine readings. Check each plate by eye; a reading is not an identification.</p>
      </div>
      <div className={`mx-now${now ? '' : ' is-empty'}`} aria-live="polite">
        {now ? (
          <>
            <RegionCrop source={media?.current} box={now.box} height={56} tick={Math.round(time * 10)} label={`Original region of the plate ${now.plate} at ${formatClock(now.time)}`} />
            <div className="mx-now__text">
              <PlateChip diff={plateDiff(now.plate, now.group.plate)}>{now.plate}</PlateChip>
              <span className="mx-now__meta"><time>{formatClock(now.time)}</time><Conf value={now.confidence} /></span>
              <span className="mx-now__label">Original region of this frame</span>
            </div>
          </>
        ) : <p>Play or step to a plate to see its original region here.</p>}
      </div>
      <ol className="mx-list">
        {groups.map(group => {
          const active = time >= group.first - 0.05 && time <= group.last + 0.05
          const members = group.reads || []
          const frames = group.count ?? members.length
          return (
            <li key={group.id} className={`mx-item${active ? ' is-active' : ''}`} aria-current={active ? 'true' : undefined}>
              <button type="button" className="mx-item__main" onClick={() => onSeek(group.first)} aria-label={`Plate ${group.plate}, seen from ${formatClock(group.first)} to ${formatClock(group.last)}`}>
                <PlateChip>{group.plate}</PlateChip>
                <time className="mx-item__time">{formatClock(group.first)}–{formatClock(group.last)}</time>
                <span className="mx-item__sub">{frames} {frames === 1 ? 'frame' : 'frames'}</span>
                <Conf value={group.confidence} />
              </button>
              {members.length ? (
                <details className="mx-reads">
                  <summary>{members.length} {members.length === 1 ? 'reading' : 'readings'}</summary>
                  <ul>
                    {members.slice(0, SHOWN_READINGS).map(read => (
                      <li key={read.id}>
                        <button type="button" onClick={() => onSeek(read.time ?? group.first)}>
                          <time>{formatClock(read.time ?? group.first)}</time>
                          <PlateChip diff={plateDiff(read.plate, group.plate)}>{read.plate}</PlateChip>
                          <Conf value={read.confidence} />
                        </button>
                      </li>
                    ))}
                    {members.length > SHOWN_READINGS ? <li className="mx-reads__more">and {members.length - SHOWN_READINGS} more</li> : null}
                  </ul>
                </details>
              ) : null}
            </li>
          )
        })}
      </ol>
      {texts.length ? (
        <details className="mx-words">
          <summary>Words read in the frames<span>{texts.length}</span></summary>
          <ul>
            {texts.slice(0, SHOWN_WORDS).map(word => (
              <li key={word.id}>
                <button type="button" onClick={() => onSeek(word.time)}>
                  <time>{formatClock(word.time)}</time>
                  <LanguageText>{word.text}</LanguageText>
                  <Conf value={word.confidence} />
                </button>
              </li>
            ))}
            {texts.length > SHOWN_WORDS ? <li className="mx-reads__more">and {texts.length - SHOWN_WORDS} more</li> : null}
          </ul>
        </details>
      ) : null}
    </section>
  )
}
