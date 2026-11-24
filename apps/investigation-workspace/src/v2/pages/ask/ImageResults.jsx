import { useEffect, useRef, useState } from 'react'
import { Link, useInRouterContext } from 'react-router-dom'
import { ArrowUpRight, ImageOff } from 'lucide-react'
import { getEvidenceContent } from '../../../lib/apiClient.js'
import { evidenceHref } from '../evidence/EvidenceCard.jsx'
import './image-results.css'

// The pictures an image search found, as a grid you can look at: the thumbnail is the case's own file, the label says HOW it matched (the identical file, the same image saved again, the same photograph
// lightly edited, or only visually similar), and nothing is drawn that the search did not return. A match is a candidate to look at, never a statement that two pictures are the same evidence or event.
// Reference-image search shows two groups, kept apart: copies first, then similar images ranked below them.

const COPY_KINDS = ['identical_file', 'same_image', 'same_photograph']
const CONTAINS_KINDS = ['region_verified', 'region_probable']
const LABELS = {
  identical_file: 'Identical file',
  same_image: 'Same image, saved again',
  same_photograph: 'Same photograph, lightly edited',
  region_verified: 'Contains it',
  region_probable: 'Probably contains it',
  similar_region: 'A part looks like it',
  similar: 'Visually similar',
  ranked: 'Candidate',
  possible: 'Also possible',
  in_part: 'In a part of it',
  duplicate_lead: 'Stands for the group',
  linked_copy: 'Linked through other copies',
}

export const isImageSearchAnswer = presentation => (presentation?.original?.route || []).includes('image_search')

// The groups of an answer, in the order the service ranked them. `ranked` (photos by words, more like this) is one list; a reference search has `copies` and `similar`.
export function imageResultGroups(response) {
  const rows = (response?.evidence?.results || []).map(row => ({ ...row, kind: row?.metadata?.match_kind || 'ranked' }))
  const stacks = []
  for (const row of rows) {
    const number = row?.metadata?.dup_group
    if (!number) continue
    let stack = stacks.find(item => item.number === number)
    if (!stack) { stack = { number, size: row.metadata.dup_group_size || 0, rows: [] }; stacks.push(stack) }
    stack.rows.push(row)
  }
  const stacked = new Set(stacks.flatMap(stack => stack.rows))
  const loose = rows.filter(row => !stacked.has(row))
  return {
    stacks,
    copies: loose.filter(row => COPY_KINDS.includes(row.kind)),
    contains: loose.filter(row => CONTAINS_KINDS.includes(row.kind)),
    parts: loose.filter(row => row.kind === 'similar_region'),
    similar: loose.filter(row => row.kind === 'similar'),
    ranked: loose.filter(row => row.kind === 'ranked'),
    possible: loose.filter(row => row.kind === 'possible'),
    inPart: loose.filter(row => row.kind === 'in_part'),
  }
}

// Where the matched region sits in a picture, as percentages of the thumbnail frame. The frame has a fixed 4:3 shape and the picture is shown whole inside it (object-fit: contain), so the
// picture's own shape decides how much of the frame it fills; the box (fractions of the picture) is placed inside that.
export const FRAME_ASPECT = 4 / 3
export function boxStyle(box, natural) {
  if (!Array.isArray(box) || box.length !== 4 || !natural?.w || !natural?.h) return null
  const [x0, y0, x1, y1] = box.map(Number)
  if (![x0, y0, x1, y1].every(Number.isFinite) || x1 <= x0 || y1 <= y0) return null
  const aspect = natural.w / natural.h
  const shownW = aspect > FRAME_ASPECT ? 100 : (aspect / FRAME_ASPECT) * 100
  const shownH = aspect > FRAME_ASPECT ? (FRAME_ASPECT / aspect) * 100 : 100
  const offX = (100 - shownW) / 2
  const offY = (100 - shownH) / 2
  const pct = value => `${Math.round(value * 100) / 100}%`
  return { left: pct(offX + x0 * shownW), top: pct(offY + y0 * shownH), width: pct((x1 - x0) * shownW), height: pct((y1 - y0) * shownH) }
}

// How a copy matched, in words (the numbers behind it are the service's, shown only as a tooltip): a hash distance in bits, or a cosine distance.
export function copyDetail(row) {
  const meta = row?.metadata || {}
  if (row.kind === 'identical_file') return 'The very same file is stored again in this case.'
  if (row.kind === 'same_image') return `The picture is the same; its hash differs by ${meta.hash_distance_bits ?? '?'} of 256 bits.`
  if (row.kind === 'same_photograph') return 'The same photograph after re-compression, resizing or a light edit.'
  if (row.kind === 'region_verified') return `${meta.region_inliers ?? 'Many'} matching points agree${meta.region_mirrored ? ' (mirrored)' : ''}. Outlined where it sits.`
  if (row.kind === 'region_probable') return `${meta.region_inliers ?? 'Some'} matching points agree: probable, not certain. Outlined where it sits.`
  if (row.kind === 'similar_region') return 'The outlined part looks like your picture. Not verified.'
  if (row.kind === 'in_part') return 'The outlined part may show what you described. Not verified.'
  return ''
}

// A bar that is honest about what it shows: the score of each row relative to the best row of its list (a similarity has no meaning as a percentage of anything else).
export function relativeShare(score, best) {
  const value = Number(score)
  const top = Number(best)
  if (!(value > 0) || !(top > 0)) return 0
  return Math.max(4, Math.min(100, Math.round((value / top) * 100)))
}

// ---- thumbnails: at most four files read at once, each turned into a small picture the browser can show, freed when the card goes away ----
const LIMIT = 4
let active = 0
const waiting = []
function runNext() {
  while (active < LIMIT && waiting.length) {
    const job = waiting.shift()
    if (job.cancelled) continue
    active += 1
    job.start().finally(() => { active -= 1; runNext() })
  }
}
function schedule(start) {
  const job = { start, cancelled: false }
  waiting.push(job)
  runNext()
  return () => { job.cancelled = true }
}

function useThumbnail(caseId, evidenceId) {
  const [state, setState] = useState({ url: '', failed: false })
  useEffect(() => {
    let url = ''
    let alive = true
    const cancel = schedule(async () => {
      try {
        const blob = await getEvidenceContent({ caseId, evidenceId })
        if (!alive) return
        url = globalThis.URL.createObjectURL(blob)
        setState({ url, failed: false })
      } catch {
        if (alive) setState({ url: '', failed: true })
      }
    })
    return () => { alive = false; cancel(); if (url) globalThis.URL.revokeObjectURL(url) }
  }, [caseId, evidenceId])
  return state
}

function Card({ row, caseId, best, tooltip, position, onAction }) {
  const inRouter = useInRouterContext()
  const id = row?.metadata?.evidence_id || row.id
  const name = row?.metadata?.source_file || id
  const { url, failed } = useThumbnail(caseId, id)
  const [natural, setNatural] = useState(null)
  // A ranked row says where it stands, not that it is "likely": a list by words always has a best and a weakest, and the bar shows how close each one is to the best.
  const label = row.kind === 'ranked' && position ? `Rank ${position}` : LABELS[row.kind] || LABELS.ranked
  const outline = boxStyle(row?.metadata?.region_box, natural)
  const picture = url
    ? <img className="nx-imgres__img" src={url} alt={name} loading="lazy" onLoad={event => setNatural({ w: event.currentTarget.naturalWidth, h: event.currentTarget.naturalHeight })} />
    : <span className="nx-imgres__ph" role="img" aria-label={failed ? 'This picture could not be opened' : 'Loading the picture'}>{failed ? <ImageOff aria-hidden="true" /> : <span className="nx-spinner" aria-hidden="true" />}</span>
  const share = row.kind === 'similar' || row.kind === 'ranked' ? relativeShare(row.score, best) : null
  const body = (
    <>
      <span className="nx-imgres__frame">{picture}{outline ? <span className="nx-imgres__box" data-kind={row.kind} style={outline} aria-hidden="true" /> : null}</span>
      <span className="nx-imgres__name" dir="auto" title={name}>{name}</span>
      <span className="nx-imgres__label" data-kind={row.kind}>{label}</span>
      {share != null ? <span className="nx-imgres__bar" role="img" aria-label={`${share} percent of the best match in this list`} title={tooltip}><span style={{ inlineSize: `${share}%` }} /></span> : <span className="nx-imgres__detail">{copyDetail(row)}</span>}
    </>
  )
  return (
    <li className="nx-imgres__card">
      {inRouter && id ? <Link className="nx-imgres__link" to={evidenceHref(caseId, { evidence_id: id })} aria-label={`${name}: ${label}. Open the evidence`}>{body}<ArrowUpRight className="nx-imgres__go" aria-hidden="true" /></Link> : <div className="nx-imgres__link">{body}</div>}
      {onAction && id ? (
        <span className="nx-imgres__actions">
          <button type="button" className="nx-imgres__action" onClick={() => onAction('more_like_this', id, name)} aria-label={`Pictures that look like ${name}`}>More like this</button>
          <button type="button" className="nx-imgres__action" onClick={() => onAction('reference', id, name)} aria-label={`Copies of ${name}`}>Find copies</button>
        </span>
      ) : null}
    </li>
  )
}

function Group({ title, note, rows, caseId, tooltip, onAction }) {
  if (!rows.length) return null
  const best = Math.max(...rows.map(row => Number(row.score) || 0))
  return (
    <section className="nx-imgres__group" aria-label={title}>
      <header><h3>{title} <span className="nx-muted">({rows.length})</span></h3>{note ? <p className="nx-muted">{note}</p> : null}</header>
      <ul className="nx-imgres__grid">{rows.map((row, index) => <Card key={`${row.id}-${row.kind}`} row={row} caseId={caseId} best={best} tooltip={tooltip} position={index + 1} onAction={onAction} />)}</ul>
    </section>
  )
}

export function ImageResults({ presentation, caseId, onAction }) {
  const { stacks, copies, contains, parts, similar, ranked, possible, inPart } = imageResultGroups(presentation?.original)
  const seen = useRef(false)
  seen.current = true
  if (!stacks.length && !copies.length && !contains.length && !parts.length && !similar.length && !ranked.length && !possible.length && !inPart.length) return <p className="nx-muted nx-imgres__empty">No picture matched.</p>
  return (
    <div className="nx-imgres">
      {stacks.map(stack => <Group key={`stack-${stack.number}`} title={`Group ${stack.number}`} note={`${stack.size} pictures. The first stands for the group.`} rows={stack.rows} caseId={caseId} onAction={onAction} />)}
      <Group title="Exact and near copies" note="Strongest first." rows={copies} caseId={caseId} onAction={onAction} />
      <Group title="Contains your picture" note="Found inside these pictures; the outline shows where." rows={contains} caseId={caseId} onAction={onAction} />
      <Group title="A part looks like it" note="Not verified; the outline shows the part." rows={parts} caseId={caseId} tooltip="Relative to the closest part in this list" onAction={onAction} />
      <Group title="Visually similar" note="Nearest first. Not copies." rows={similar} caseId={caseId} tooltip="Relative to the most similar image in this list" onAction={onAction} />
      <Group title="Best matches" note="Best first." rows={ranked} caseId={caseId} tooltip="Relative to the best match in this list" onAction={onAction} />
      <Group title="Also possible" note="Weaker matches." rows={possible} caseId={caseId} tooltip="Relative to the best match of the first list" onAction={onAction} />
      <Group title="In a part of a picture" note="Not verified; the outline shows the part." rows={inPart} caseId={caseId} tooltip="Relative to the best part in this list" onAction={onAction} />
    </div>
  )
}
