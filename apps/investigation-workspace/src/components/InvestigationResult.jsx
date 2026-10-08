import { ClaimText, SourcePanel } from './Citations.jsx'
import { ClarificationPrompt } from './ClarificationPrompt.jsx'
import { ResultChart } from './ResultChart.jsx'
import { answerSummary } from '../lib/answerSummary.js'
import { ResultTable } from './ResultTable.jsx'
import { EmptyState, FindingCard, LanguageText, MethodDetails, ResultStateBanner, TechnicalDisclosure } from './AnalystComponents.jsx'

function SectionHeading({ children }) {
  return <h2 className="section-heading">{children}</h2>
}

function ScopeChips({ scope }) {
  if (!scope.length) return null
  return (
    <dl className="scope-chips" aria-label="Applied scope">
      {scope.map((item, index) => (
        <div key={`${item.label}-${item.value}-${index}`}>
          <dt>{item.label}</dt><dd><LanguageText>{item.value}</LanguageText></dd>
        </div>
      ))}
    </dl>
  )
}

// `idPrefix` and `live` exist so several results can be rendered in one
// conversation thread.
//
//   - Section headings are referenced by `aria-labelledby`. Rendering two
//     results with the same fixed ids puts duplicate ids in the document and a
//     screen reader resolves the label to whichever came first, so the second
//     answer's "Citations" section is announced as the first answer's.
//   - `aria-live` belongs on the THREAD, not on every turn in it. A dozen live
//     regions on one page is a dozen things competing to interrupt.
//
// Both default to today's exact behaviour, so a single standalone result — and
// every existing test of one — is unchanged.
export function InvestigationResult({ presentation, onAsk, onClarificationChoice, onRetry, onCompare, clarificationRounds = 0, idPrefix = '', live = true, showOriginalQuestion = true, compact = false }) {
  const headingId = name => (idPrefix ? `${idPrefix}-${name}` : name)
  const liveRegion = live ? 'polite' : undefined
  if (presentation.state === 'clarify') {
    return (
      <article className="investigation-result investigation-result--clarify" aria-live={liveRegion}>
        <ResultStateBanner state="clarify" />
        <ClarificationPrompt
          model={presentation.clarification}
          onChoose={onClarificationChoice}
          canChoose={clarificationRounds === 0}
        />
      </article>
    )
  }
  const hasResult = presentation.result.columns.length > 0 && presentation.result.rows.length > 0
  const useSourceRail = (presentation.citations.groups || []).length >= 3
  const findingState = !['unsupported', 'failed'].includes(presentation.state)
  return (
    <article className={`investigation-result investigation-result--${presentation.state}${useSourceRail && !compact ? ' investigation-result--source-rail' : ''}${compact ? ' investigation-result--compact' : ''}`} aria-live={liveRegion}>
      <ResultStateBanner state={presentation.state} />
      <header className="answer-block" dir={presentation.direction || 'ltr'}>
        {findingState ? (
          <FindingCard classification="fact" citationCount={(presentation.citations.groups || []).length || presentation.citations.items.length}>
            <p className="answer-claim"><ClaimText text={presentation.answer} segments={presentation.claimSegments} citations={presentation.citations.markerItems || presentation.citations.items} /></p>
          </FindingCard>
        ) : (
          <div className={`answer-notice answer-notice--${presentation.state}`} role="note">
            <LanguageText as="p" className="answer-claim">{presentation.answer}</LanguageText>
          </div>
        )}
        {findingState ? (
          <dl className="answer-summary" aria-label="What this answer rests on">
            {answerSummary(presentation).map(item => (
              <div key={item.id} className={`answer-summary__item${item.tone ? ` answer-summary__item--${item.tone}` : ''}`}>
                <dt>{item.label}</dt>
                <dd><b><LanguageText>{item.value}</LanguageText></b>{item.detail ? <small>{item.detail}</small> : null}</dd>
              </div>
            ))}
          </dl>
        ) : null}
        {findingState && presentation.followUps.length > 0 ? (
          <div className="follow-ups follow-ups--inline" role="group" aria-label="Suggested next questions">
            <span>Ask next</span>
            {presentation.followUps.map(item => (
              <button key={item.query} type="button" onClick={() => onAsk(item.query)} title={item.reason || undefined}>{item.label}</button>
            ))}
          </div>
        ) : null}
        {showOriginalQuestion && presentation.originalQuestion && <p className="asked-question"><span>Asked</span> “<LanguageText>{presentation.originalQuestion}</LanguageText>”</p>}
        {onCompare && ['answered', 'partial', 'zero-result'].includes(presentation.state) ? <button className="compare-result" type="button" onClick={onCompare}>Add result to comparison</button> : null}
      </header>

      {presentation.state === 'failed' && (
        <div className="failure-action">
          <p>Reference: <LanguageText as="bdi" identifier>{presentation.errorReference}</LanguageText></p>
          <button type="button" onClick={onRetry}>Try this question again</button>
        </div>
      )}

      {presentation.state !== 'failed' && presentation.state !== 'unsupported' && (
        <>
          {hasResult || presentation.state === 'zero-result' ? (
            <section className="result-section result-section--result" aria-labelledby={headingId("result-heading")}>
              <SectionHeading><span id={headingId("result-heading")}>The result</span></SectionHeading>
              {presentation.state === 'zero-result' && <ScopeChips scope={presentation.scope} />}
              {hasResult ? <><ResultChart presentation={presentation} /><ResultTable result={presentation.result} citations={presentation.citations} /></> : <EmptyState kind="complete-zero" />}
            </section>
          ) : null}
          {!compact && (
            <>
          <section className="result-section result-section--citations" aria-labelledby={headingId("citations-heading")}>
            <SectionHeading><span id={headingId("citations-heading")}>Citations</span></SectionHeading>
            <SourcePanel citations={presentation.citations} />
          </section>
          <section className="result-section result-section--derivation" aria-labelledby={headingId("derivation-heading")}>
            <SectionHeading><span id={headingId("derivation-heading")}>How this was derived</span></SectionHeading>
            <TechnicalDisclosure summary="Read the analysis method">
              <p>{presentation.derivation}</p>
              <MethodDetails method={presentation.method} />
            </TechnicalDisclosure>
          </section>
            </>
          )}
          {presentation.limitations.length > 0 && !compact && (
            <section className="result-section result-section--limitations limitations" aria-labelledby={headingId("limitations-heading")}>
              <SectionHeading><span id={headingId("limitations-heading")}>Limitations</span></SectionHeading>
              <ul>{presentation.limitations.map(item => <li key={item}>{item}</li>)}</ul>
            </section>
          )}
          {presentation.limitations.length > 0 && compact ? (
            <ul className="compact-limits" aria-label="Limitations">{presentation.limitations.map(item => <li key={item}>{item}</li>)}</ul>
          ) : null}
        </>
      )}
    </article>
  )
}
