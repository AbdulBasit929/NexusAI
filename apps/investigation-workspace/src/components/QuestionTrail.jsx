export function QuestionTrail({ entries = [], history = [], onRerun, onEdit, onPin }) {
  if (!entries.length && !history.length) return null
  const pinnedCount = history.filter(item => item.pinned).length
  return <>
    {history.length ? <details className="question-history" aria-labelledby="question-history-title">
      <summary><span className="question-history__summary-title"><span className="eyebrow">This browser only</span><strong id="question-history-title" role="heading" aria-level="2">Question history</strong></span><span>{history.length} {history.length === 1 ? 'question' : 'questions'} · {pinnedCount} pinned</span></summary>
      <ol>{history.map(item => <li key={item.id}><div><LanguageText as="strong">{item.label || item.query}</LanguageText>{item.label ? <LanguageText as="small">{item.query}</LanguageText> : null}<small>{item.state === 'clarify' ? 'Clarification requested' : item.state}</small></div><div className="question-history__actions"><button type="button" onClick={() => onRerun?.(item.query)}>Re-run</button><button type="button" onClick={() => onEdit?.(item.query)}>Edit</button><button type="button" aria-pressed={item.pinned} onClick={() => onPin?.(item.id)}>{item.pinned ? 'Unpin' : 'Pin'}</button></div></li>)}</ol>
    </details> : null}
    {entries.length ? <details className="question-trail" aria-labelledby="question-trail-title">
      <summary><span><span className="eyebrow">Investigation trail</span><strong id="question-trail-title" role="heading" aria-level="2">How this question changed</strong></span><span aria-hidden="true">+</span></summary>
      <ol>
        {entries.map((entry, index) => (
          <li key={`${entry.query}-${index}`}>
            <p><span>Originally asked</span> “<LanguageText>{entry.originalQuestion}</LanguageText>”</p>
            <p><span>Choice selected</span> <LanguageText>{entry.label}</LanguageText></p>
            <p><span>Question sent</span> “<LanguageText>{entry.query}</LanguageText>”</p>
          </li>
        ))}
      </ol>
    </details> : null}
  </>
}
import { LanguageText } from './AnalystComponents.jsx'
