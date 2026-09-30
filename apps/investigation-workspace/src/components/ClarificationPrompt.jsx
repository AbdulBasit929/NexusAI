export function ClarificationPrompt({ model, onChoose, canChoose = true }) {
  return (
    <section className="clarification" aria-labelledby="clarification-title">
      <p className="state-kicker">Careful result</p>
      <h2 id="clarification-title">{model.title}</h2>
      <LanguageText as="p" className="clarification__question">{model.question}</LanguageText>
      {model.options.length > 0 && (
        <div className="clarification__options" aria-label="Choose the missing detail">
          {model.options.map((option, index) => option.query && canChoose ? (
            <button
              key={`${option.label}-${index}`}
              type="button"
              title={option.description || undefined}
              onClick={() => onChoose(option, model)}
            >
              <strong>{option.label}</strong>
              <span>Ask: “<LanguageText>{option.query}</LanguageText>”</span>
            </button>
          ) : (
            <span key={`${option.label}-${index}`} className="clarification__unbound" title={option.description || undefined}>
              <strong>{option.label}</strong>
              {option.query
                ? <span>Question: “<LanguageText>{option.query}</LanguageText>”</span>
                : <span>This saved choice does not include a question that can be sent.</span>}
            </span>
          ))}
        </div>
      )}
      {!canChoose && model.options.length > 0 && (
        <p className="clarification__degraded" role="note">
          This follow-up still needs a manual scope choice. No second automatic clarification was sent.
        </p>
      )}
      {model.degraded && (
        <p className="clarification__degraded" role="note">
          {model.hasLegacyOptions
            ? 'This saved response contains labels but no questions that can be sent. No analysis was run and no result was stated.'
            : 'The required choices were not supplied with this response. No analysis was run and no result was stated.'}
        </p>
      )}
      {model.originalQuestion && <p className="asked-question"><span>Asked</span> “<LanguageText>{model.originalQuestion}</LanguageText>”</p>}
    </section>
  )
}
import { LanguageText } from './AnalystComponents.jsx'
