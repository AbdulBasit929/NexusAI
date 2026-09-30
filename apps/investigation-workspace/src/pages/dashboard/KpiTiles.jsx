import { useEffect, useRef, useState } from 'react'
import { ArrowUpRight, CircleCheckBig, Clock3, FileWarning, Rows3, SearchCheck } from 'lucide-react'
import { ProportionBar } from '../../components/DataVisualizations.jsx'
import { formatNumber } from '../../lib/format.js'
import { jumpToElement } from '../../lib/jump.js'

const prefersReducedMotion = () => Boolean(globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches)

// Floored, never rounded: 999 of 1,000 ready is 99%, so a real failure is not rounded into "100%".
function readyPercent(kpis) {
  return kpis.sources ? `${Math.floor((kpis.ready / kpis.sources) * 100)}%` : null
}

function reviewLine(kpis) {
  const parts = []
  if (kpis.failed > 0) parts.push(`${formatNumber(kpis.failed)} failed`)
  if (kpis.gaps > 0) parts.push(`${formatNumber(kpis.gaps)} missing copy`)
  return parts.length ? parts.join(' · ') : 'All clear'
}

// Four metrics, four cards. Each card carries its own tone (failed red, ready green, processing amber, data teal)
// so the eye can rank them before reading; the first is the hero because it is the page's job. Tone follows the
// data: zero review is green ("All clear"), zero processing is a quiet neutral. Words carry the meaning too, and the
// long definitions live in the tooltip and accessible name, not as body copy. Rules: UI_REDESIGN_BRIEF §11.
export function kpiTiles(kpis, targets = {}) {
  // Totals cover only the cases that reported. Say so on the card, so a partial sum is never read as the whole.
  const scope = kpis.unreported > 0 && kpis.reported > 0 ? `${formatNumber(kpis.reported)} of ${formatNumber(kpis.cases)} cases` : null
  return [
    {
      id: 'review',
      label: 'Needs review',
      icon: kpis.review > 0 ? FileWarning : CircleCheckBig,
      value: kpis.review,
      line: reviewLine(kpis),
      title: `${formatNumber(kpis.failed)} failed sources and ${formatNumber(kpis.gaps)} completed jobs missing their retained copy`,
      href: targets.review || '#dashboard-attention',
      tone: kpis.review > 0 ? 'failed' : 'ready',
      hero: true,
      scope,
    },
    {
      id: 'ready',
      label: 'Ready',
      icon: SearchCheck,
      value: kpis.ready,
      valueSuffix: kpis.sources ? `of ${formatNumber(kpis.sources)}` : null,
      percent: readyPercent(kpis),
      line: kpis.sources ? null : 'No sources yet',
      title: 'Sources you can search now',
      href: targets.ready || '#dashboard-cases',
      tone: 'ready',
      // A bar of nothing says nothing, so it is drawn only when there is something to measure.
      bar: kpis.sources ? { total: kpis.sources, ready: kpis.ready, processing: kpis.processing, failed: kpis.failed } : null,
      scope,
    },
    {
      id: 'processing',
      label: 'Processing',
      icon: Clock3,
      value: kpis.processing,
      line: kpis.processing ? 'In progress' : 'Idle',
      title: 'Sources that are not searchable until processing finishes',
      href: targets.processing || '#dashboard-cases',
      tone: kpis.processing ? 'processing' : 'quiet',
      quiet: !kpis.processing,
      live: kpis.processing > 0,
      scope,
    },
    { id: 'rows', label: 'Structured rows', icon: Rows3, value: kpis.acceptedRows, line: 'Accepted', title: 'Records accepted for analysis', href: targets.rows || '#dashboard-families', tone: 'data', scope },
  ]
}

// Counts up to its target in about 700 ms with ease-out, and only ever animates transform-free text. Reduced motion
// or a missing animation clock writes the final value immediately. Screen readers get the final value separately.
export function useCountUp(target, { animate = true, duration = 700 } = {}) {
  const canAnimate = animate && !prefersReducedMotion() && typeof globalThis.requestAnimationFrame === 'function'
  const [value, setValue] = useState(canAnimate ? 0 : target)
  const current = useRef(value)
  useEffect(() => {
    if (!canAnimate) {
      current.current = target
      setValue(target)
      return undefined
    }
    const from = current.current
    if (from === target) return undefined
    const started = globalThis.performance.now()
    let frame
    const tick = now => {
      const progress = Math.min(1, (now - started) / duration)
      const next = Math.round(from + (target - from) * (1 - (1 - progress) ** 3))
      current.current = next
      setValue(next)
      if (progress < 1) frame = globalThis.requestAnimationFrame(tick)
    }
    frame = globalThis.requestAnimationFrame(tick)
    return () => globalThis.cancelAnimationFrame(frame)
  }, [target, canAnimate, duration])
  return value
}

// A card click lands on its section (see lib/jump.js); when the section is not on the page the link behaves normally.
function jumpTo(event, href) {
  if (jumpToElement(href)) event.preventDefault()
}

function Figure({ value, unknown, animate }) {
  const shown = useCountUp(value, { animate })
  if (unknown) return <span className="dash-kpi__num">{unknown}</span>
  return (
    <>
      <span className="dash-kpi__num" aria-hidden="true">{formatNumber(shown)}</span>
      <span className="visually-hidden">{formatNumber(value)}</span>
    </>
  )
}

// Loading shows an ellipsis and "nothing reported" shows a dash: neither is ever drawn as a measured zero.
export function KpiTiles({ kpis, loading, animate = true, targets = {}, label = 'Workspace totals' }) {
  const unavailable = !loading && kpis.cases > 0 && kpis.reported === 0
  const unknown = loading ? '…' : unavailable ? '—' : null
  return (
    <ul className="dash-kpis dash-cards" aria-label={label} aria-busy={loading ? 'true' : 'false'}>
      {kpiTiles(kpis, targets).map(tile => {
        const Icon = tile.icon
        const tone = unknown ? 'quiet' : tile.tone
        return (
          <li key={tile.id} className={`dash-kpi dash-kpi--${tone}${tile.hero ? ' dash-kpi--hero' : ''}`}>
            <a href={tile.href} title={tile.title} onClick={event => jumpTo(event, tile.href)}>
              <span className="dash-kpi__top">
                <span className="dash-kpi__chip"><Icon aria-hidden="true" /></span>
                <span className="dash-kpi__label">{tile.label}{tile.live && !unknown ? <span className="dash-kpi__pulse" aria-hidden="true" /> : null}</span>
                <ArrowUpRight className="dash-kpi__go" aria-hidden="true" />
              </span>
              <strong>
                <Figure value={tile.value} unknown={unknown} animate={animate} />
                {!unknown && tile.valueSuffix ? <span className="dash-kpi__of"> {tile.valueSuffix}</span> : null}
                {!unknown && tile.percent ? <span className="dash-kpi__pct">{tile.percent}</span> : null}
              </strong>
              {tile.bar && !unknown ? <ProportionBar {...tile.bar} label="Evidence readiness across all cases" /> : null}
              {unavailable ? <small>Unavailable</small> : tile.line ? <small>{tile.line}</small> : null}
              {tile.scope && !unknown ? <small className="dash-kpi__scope">{tile.scope}</small> : null}
            </a>
          </li>
        )
      })}
    </ul>
  )
}
