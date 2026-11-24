import { useEffect, useState } from 'react'
import { ArrowLeftRight, Check, ChevronRight, CircleAlert, CircleHelp, Copy, FileSearch, GitFork, Info, Pencil, Pin, PinOff, RefreshCw, RotateCcw, SearchX, Sparkles, ThumbsDown, ThumbsUp } from 'lucide-react'
import { ClaimText } from '../../../components/Citations.jsx'
import { ClarificationPrompt } from '../../../components/ClarificationPrompt.jsx'
import { MethodDetails } from '../../../components/AnalystComponents.jsx'
import { AnswerReading } from '../../../components/AnswerBasis.jsx'
import { formatNumber } from '../../../lib/format.js'
import { ResultBlock } from './ResultBlock.jsx'
import { ImageResults, isImageSearchAnswer } from './ImageResults.jsx'

function useElapsed(startedAt) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = globalThis.setInterval(() => setNow(Date.now()), 1000)
    return () => globalThis.clearInterval(timer)
  }, [])
  return Math.max(0, Math.round((now - (startedAt || now)) / 1000))
}

function Waiting({ turn }) {
  const seconds = useElapsed(turn.startedAt)
  const where = !turn.scope || turn.scope.label === 'All evidence' ? 'the evidence of this case' : `${turn.scope.label} evidence`
  const phase = turn.phase
  const clock = phase?.elapsedMs != null ? Math.round(phase.elapsedMs / 1000) : seconds
  // only phases the service really reports; it never invents progress, so there is a clock and no percentage
  let title = `Searching ${where}…`
  if (phase?.name === 'queued') title = `Waiting for the language model${phase.position ? ` (number ${phase.position} in the queue)` : ''}…`
  else if (phase?.label) title = `${phase.label}…`
  return (
    <div className="nx-wait" role="status">
      <span className="nx-spinner" aria-hidden="true" />
      <div>
        <p className="nx-wait__title">{title}</p>
        <p className="nx-wait__text nx-muted">{clock} s. A question usually takes 20 to 60 seconds on this machine.{turn.stored || phase ? ' It keeps running if you leave this page.' : ' You can keep reading the answers above.'}</p>
      </div>
    </div>
  )
}

// A word and a shape for how strong the evidence under an answer is; the colour only supports them.
const SHAPE = { strong: 'observed', medium: 'inferred', weak: 'candidate' }
function StrengthMark({ shape }) {
  if (shape === 'observed') return <svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="currentColor" /></svg>
  if (shape === 'inferred') return <svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="none" stroke="currentColor" strokeWidth="1.5" /><path d="M6 1.5a4.5 4.5 0 0 1 0 9z" fill="currentColor" /></svg>
  return <svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="2.2 1.8" /></svg>
}

export function topStrength(presentation) {
  const groups = presentation?.citations?.groups || []
  if (!groups.length) return null
  const counts = new Map()
  for (const group of groups) {
    const key = group.strength?.label || 'Strength unavailable'
    const entry = counts.get(key) || { count: 0, shape: SHAPE[group.strength?.id] || 'candidate', label: key }
    entry.count += 1
    counts.set(key, entry)
  }
  return [...counts.values()].sort((left, right) => right.count - left.count)[0]
}

function Strength({ strength }) {
  if (!strength) return <span className="nx-strength" data-strength="candidate"><StrengthMark shape="candidate" />No openable source</span>
  return <span className="nx-strength" data-strength={strength.shape}><StrengthMark shape={strength.shape} />{strength.label}</span>
}

const CHECK = {
  yes: 'Recomputed from the records: matches',
  no: 'Recomputed from the records: does not match',
  not_checked: 'Not recomputed from the records',
}

// What the answer rests on, in one line: the records and files behind the figure, and whether they were counted again.
function EvidenceBar({ presentation, open, onToggle }) {
  if (isImageSearchAnswer(presentation)) return null // the pictures themselves are the evidence, and each opens as the file
  const evidence = presentation.original?.enterprise?.evidence
  const groups = presentation.citations?.groups || []
  let lead
  if (evidence?.state === 'itemised') {
    const files = (evidence.files || []).length
    lead = evidence.kind === 'lines'
      ? `${formatNumber(evidence.lines_total || (evidence.lines || []).length)} lines, each standing for records`
      : `${formatNumber(evidence.total_rows)} ${evidence.total_rows === 1 ? 'record' : 'records'} in ${formatNumber(files)} ${files === 1 ? 'file' : 'files'}`
  } else if (groups.length) {
    lead = `${formatNumber(groups.length)} ${groups.length === 1 ? 'source file' : 'source files'}`
  } else {
    lead = 'No openable source came with this answer'
  }
  const check = evidence?.state === 'itemised' ? CHECK[evidence.reconciled] || CHECK.not_checked : ''
  return (
    <button type="button" className="nx-evidence-bar" aria-expanded={open} onClick={onToggle}>
      <FileSearch aria-hidden="true" />
      <span className="nx-evidence-bar__text"><strong>Behind this answer</strong> {lead}{check ? <span className={`nx-evidence-bar__check${evidence.reconciled === 'no' ? ' is-warn' : ''}`}> · {check}</span> : null}</span>
      <span className="nx-evidence-bar__go">{open ? 'Close the evidence' : 'Open the evidence'}<ChevronRight aria-hidden="true" /></span>
    </button>
  )
}

export function Turn({ turn, index, last, caseId, onAsk, onChoose, onRetry, onEdit, onCopy, copied, evidenceOpen, onToggleEvidence, onRerun, onFork, onFeedback, onPin, onCompare, onImageAction }) {
  const presentation = turn.presentation
  const state = presentation?.state
  const answered = presentation && ['answered', 'partial', 'zero-result'].includes(state)
  const hasResult = answered && presentation.result.columns.length > 0 && presentation.result.rows.length > 0
  const interpretation = (presentation?.scope || []).slice(0, 6)
  const time = turn.finishedAt || turn.startedAt
  return (
    <article className="nx-turn" aria-label={`Question ${index + 1}`}>
      <header className="nx-turn__question">
        {turn.parentId ? <p className="nx-turn__branch">Following your choice{turn.label ? <>: <bdi>{turn.label}</bdi></> : null}</p> : null}
        <h2 className="nx-turn__text" dir="auto">{turn.query}</h2>
        {turn.kind === 'edit' ? <p className="nx-turn__branch">Edited question</p> : null}
        <p className="nx-turn__meta">{turn.scope?.label || 'All evidence'}{time ? <> · <time dateTime={new Date(time).toISOString()}>{new Date(time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</time></> : null}</p>
      </header>
      <div id={`${turn.id}-anchor`} tabIndex={-1} className="nx-turn__answer">
        {turn.busy ? <Waiting turn={turn} /> : null}
        {turn.aborted && !turn.busy ? <p className="nx-callout nx-callout--info" role="status"><Info aria-hidden="true" />{turn.stoppedWaiting ? 'You stopped waiting. The question may still finish: open this conversation again to see its answer.' : 'This question was stopped before a result came back.'}</p> : null}
        {turn.answerMissing && !turn.presentation && !turn.busy ? <p className="nx-callout nx-callout--info" role="status"><Info aria-hidden="true" /><span>This answer was not kept, or is still being loaded.{onRerun ? <> <button type="button" className="nx-link-button" onClick={onRerun}>Run it again</button></> : null}</span></p> : null}

        {state === 'clarify' ? (
          <div className="nx-clarify">
            <p className="nx-clarify__lead"><CircleHelp aria-hidden="true" />I need one detail to answer this well.</p>
            <ClarificationPrompt model={presentation.clarification} onChoose={option => onChoose(option, presentation.clarification)} canChoose={!turn.parentId} />
          </div>
        ) : null}

        {state === 'failed' ? (
          <div className="nx-callout nx-callout--danger" role="alert">
            <CircleAlert aria-hidden="true" />
            <div>
              <p><strong>This question could not be completed.</strong> Your case evidence was not changed.</p>
              {turn.serviceMessage ? <p dir="auto">{turn.serviceMessage}</p> : null}
              <p className="nx-muted">Reference <span className="nx-mono">{presentation.errorReference}</span></p>
              <p><button type="button" className="nx-btn nx-btn--sm" onClick={onRetry}><RefreshCw aria-hidden="true" />Try this question again</button></p>
            </div>
          </div>
        ) : null}

        {state === 'unsupported' ? (
          <div className="nx-callout" role="note">
            <SearchX aria-hidden="true" />
            <div><p><strong>This question is outside what can be answered from this case.</strong></p><p dir="auto">{presentation.answer}</p></div>
          </div>
        ) : null}

        {state === 'processing' ? (
          <div className="nx-callout nx-callout--warn" role="status"><Info aria-hidden="true" /><p dir="auto">{presentation.answer}</p></div>
        ) : null}

        {answered ? (
          <div className="nx-answer">
            <div className="nx-answer__meta">
              {isImageSearchAnswer(presentation) ? <span className="nx-strength" data-strength="candidate"><StrengthMark shape="candidate" />Candidates to look at</span> : <Strength strength={topStrength(presentation)} />}
              {state === 'partial' ? <span className="nx-badge nx-badge--warn">Partial answer</span> : null}
              {turn.diff ? (
                turn.diff.changed
                  ? <button type="button" className="nx-badge nx-badge--warn nx-badge--button" onClick={onCompare}><ArrowLeftRight aria-hidden="true" />Changed since the earlier run · Compare</button>
                  : <span className="nx-badge">Same as the earlier run</span>
              ) : turn.reruns?.latest_changed ? <button type="button" className="nx-badge nx-badge--warn nx-badge--button" onClick={onCompare}><ArrowLeftRight aria-hidden="true" />A later run differs · Compare</button> : null}
            </div>
            <p className="nx-answer__text" dir={presentation.direction || 'ltr'}>
              <ClaimText text={presentation.answer} segments={presentation.claimSegments} citations={presentation.citations.markerItems || presentation.citations.items} />
            </p>
            {interpretation.length ? (
              <dl className="nx-scope-line" aria-label="How the question was read">
                {interpretation.map((item, position) => <div key={`${item.label}-${item.value}-${position}`}><dt>{item.label}</dt><dd dir="auto">{item.value}</dd></div>)}
              </dl>
            ) : null}
            {hasResult ? <div className="nx-answer__result"><ResultBlock presentation={presentation} caseId={caseId} onAsk={onAsk} /></div> : null}
            {isImageSearchAnswer(presentation) ? <div className="nx-answer__result"><ImageResults presentation={presentation} caseId={caseId} onAction={onImageAction} /></div> : null}
            <EvidenceBar presentation={presentation} open={evidenceOpen} onToggle={onToggleEvidence} />
            <AnswerReading caseId={caseId} reading={presentation.original?.enterprise?.reading} />
            {presentation.limitations.length ? (
              isImageSearchAnswer(presentation) ? (
                <details className="nx-disclosure">
                  <summary><ChevronRight aria-hidden="true" />What to keep in mind</summary>
                  <div className="nx-disclosure__body"><ul className="nx-limits" aria-label="Limitations">{presentation.limitations.map(item => <li key={item}><Info aria-hidden="true" /><span>{item}</span></li>)}</ul></div>
                </details>
              ) : (
                <ul className="nx-limits" aria-label="Limitations">{presentation.limitations.map(item => <li key={item}><Info aria-hidden="true" /><span>{item}</span></li>)}</ul>
              )
            ) : null}
            {presentation.derivation ? (
              <details className="nx-disclosure">
                <summary><ChevronRight aria-hidden="true" />How this was worked out</summary>
                <div className="nx-disclosure__body"><p>{presentation.derivation}</p><MethodDetails method={presentation.method} /></div>
              </details>
            ) : null}
            {presentation.followUps.length ? (
              <div className="nx-followups" role="group" aria-label="Suggested next questions">
                <span className="nx-followups__label"><Sparkles aria-hidden="true" />Ask next</span>
                {presentation.followUps.map(item => <button key={item.query} type="button" className="nx-chip" title={item.reason || undefined} onClick={() => onAsk(item.query)}>{item.label}</button>)}
              </div>
            ) : null}
          </div>
        ) : null}

        {presentation && state !== 'clarify' ? (
          <div className="nx-actions" role="group" aria-label="Actions for this answer">
            {answered ? <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost" onClick={onCopy}>{copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}{copied ? 'Copied' : 'Copy answer'}</button> : null}
            <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost" onClick={onEdit}><Pencil aria-hidden="true" />Ask a variation</button>
            {turn.stored && onRerun && !turn.busy ? <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost" onClick={onRerun} title="Ask the same question again against the data as it is now"><RotateCcw aria-hidden="true" />Run again</button> : null}
            {turn.stored && answered && onPin ? <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost" aria-pressed={turn.pinnedAs?.length > 0} onClick={onPin} title="Keep this answer as a finding of the case">{turn.pinnedAs?.length ? <PinOff aria-hidden="true" /> : <Pin aria-hidden="true" />}{turn.pinnedAs?.length ? 'Pinned to findings' : 'Pin to findings'}</button> : null}
            {turn.stored && onFork ? <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost" onClick={onFork} title="Continue from here in a conversation of your own"><GitFork aria-hidden="true" />Fork</button> : null}
            {turn.stored && answered && onFeedback ? (
              <span className="nx-actions__verdict" role="group" aria-label="Is this answer right?">
                <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost nx-btn--icon" aria-pressed={turn.feedback?.mine?.verdict === 'correct'} aria-label="This answer is correct" title="This answer is correct" onClick={() => onFeedback('correct')}><ThumbsUp aria-hidden="true" /></button>
                <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost nx-btn--icon" aria-pressed={turn.feedback?.mine?.verdict === 'wrong'} aria-label="This answer is wrong" title="This answer is wrong" onClick={() => onFeedback('wrong')}><ThumbsDown aria-hidden="true" /></button>
              </span>
            ) : null}
          </div>
        ) : null}
      </div>
      {last ? null : <hr className="nx-divider" />}
    </article>
  )
}
