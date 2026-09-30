import { Link } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { Card } from '../../components/Card.jsx'

// Questions that already work today through Investigate, so the card is useful before entity summaries exist.
export const ENTITY_QUESTIONS = [
  'Who are the most frequent contacts?',
  'Where was activity most often observed?',
  'Which vehicle plates appear most often?',
]

export function entityQuestionLink(caseId, question) {
  return `/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(question)}`
}

// Key entities. Who and where show up most is the second thing an investigator looks for, and no endpoint summarises it
// for a whole case yet (backend request 19: today frequent contacts and top locations refuse an empty target). This
// card says so plainly, shows where the bubbles will appear, and offers the same questions through Investigate, which
// answers them now. Nothing here is a number.
export function KeyEntities({ caseId }) {
  return (
    <div id="dashboard-entities" className="dash-slot">
      <Card className="key-entities" title="Who and where shows up most?">
        <div className="ke-ghost" aria-hidden="true"><i /><i /><i /><i /><i /></div>
        <p className="ke-note">Top contacts, places and plates will be summarised here once the case can report them. Until then, ask.</p>
        {caseId ? (
          <ul className="ke-asks" aria-label="Ask in Investigate">
            {ENTITY_QUESTIONS.map(question => <li key={question}><Link to={entityQuestionLink(caseId, question)}>{question}<ArrowRight aria-hidden="true" /></Link></li>)}
          </ul>
        ) : null}
      </Card>
    </div>
  )
}
