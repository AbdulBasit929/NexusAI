import { LanguageText } from './AnalystComponents.jsx'
import { useCaseRecords } from '../lib/useCaseRecords.js'

// A case is shown by the name an analyst gave it when there is one, with the identifier kept in reach (as the title, and beside the name where there is room).
// With no record the output is exactly what the interface showed before: the identifier. The name is user text: it is only ever rendered as text, never as markup.
export function CaseTitle({ caseId, as = 'bdi', showId = false, ...props }) {
  const { records } = useCaseRecords()
  const name = records[caseId]?.display_name
  if (!name) return <LanguageText as={as} identifier {...props}>{caseId}</LanguageText>
  return (
    <>
      <LanguageText as={as} title={caseId} {...props}>{name}</LanguageText>
      {showId ? <small className="case-title__id"><LanguageText as="bdi" identifier>{caseId}</LanguageText></small> : null}
    </>
  )
}
