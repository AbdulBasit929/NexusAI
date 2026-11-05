import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { CircleAlert } from 'lucide-react'
import { configuredCaseIds } from '../../../lib/apiClient.js'
import { attentionItems, familyRows, reviewByCase } from '../../../lib/dashboardCharts.js'
import { aggregateFamilyRows, dashboardKpis, dashboardRowState, latestEvidenceActivity, parseTimestamp } from '../../../lib/dashboardCases.js'
import { useCaseActivity } from '../../../lib/useCaseActivity.js'
import { useCaseRecords } from '../../../lib/useCaseRecords.js'
import { formatNumber } from '../../../lib/format.js'
import { summariseCase, useConfiguredCaseOverviews } from '../../../lib/useCaseOverview.js'
import { useQuestionHistoryAcross } from '../../../lib/workspaceState.js'
import { AppShell } from '../../shell/AppShell.jsx'
import { useEngineHealthAcross, useTypingReportsAcross } from './data.js'
import { AnswersPanel, EvidenceTypesPanel, RecordKindsPanel, SetAsidePanel, TimeToReadyPanel, TimelinePanel } from './insights.jsx'
import { answerQuality, caseStates, caseTimelines, dataNotes, modalityMix, recordKinds, setAside, timeToReady } from './model.js'
import { CasesPanel } from './panels.jsx'
import { Hero, LatestPanel, MetricStrip, NeedsPanel, QuestionsPanel, ResumeStrip, latestItems, resumeEntries } from './sections.jsx'

// The most cases any one dashboard read covers: the heavy reads (the typing report, the timeline) are made for each case, and a workspace with a very long list should not start dozens of them.
const READ_LIMIT = 12

// The dashboard. The state of the workspace in five figures, then what needs you, when things happened, what the evidence holds, how the questions were answered, how the
// pipeline did, and the cases. Every figure is a count a service reported, a share of two such counts, or a median of dated jobs; a case that could not report is named, never counted as zero.
// The cheap reads (status, case names, answers) start at once; the heavy ones (typing report, timeline) start when every case has reported, and each panel fills as its read arrives.
export default function DashboardPage() {
  const cases = configuredCaseIds()
  const covered = useMemo(() => cases.slice(0, READ_LIMIT), [cases.join('|')]) // eslint-disable-line react-hooks/exhaustive-deps
  const recent = useQuestionHistoryAcross(cases)
  const { states: overviews, reload } = useConfiguredCaseOverviews(cases)
  const { records } = useCaseRecords()
  useEffect(() => { globalThis.document.title = 'Dashboard · NexusAI' }, [])

  const rows = useMemo(() => cases.map((caseId, index) => {
    const state = overviews[caseId]
    return { caseId, index, state, status: dashboardRowState(state), summary: state?.data ? summariseCase(state.data) : null, activity: state?.data ? latestEvidenceActivity(state.data) : null }
  }), [cases, overviews])
  const kpis = useMemo(() => dashboardKpis(rows), [rows])
  const loading = rows.some(row => row.status === 'loading')
  const unreadable = rows.filter(row => row.status === 'unavailable').length
  const attention = useMemo(() => attentionItems(rows), [rows])
  const reviewCases = useMemo(() => reviewByCase(rows), [rows])
  const latest = useMemo(() => latestItems(rows), [rows])
  const hasProcessing = rows.some(row => row.status === 'processing')
  const isRefreshing = rows.some(row => row.state?.refreshing)
  // The age of the figures is the age of the oldest read among the cases shown; the newest would hide a case whose refresh failed.
  const lastUpdated = rows.filter(row => row.summary).map(row => parseTimestamp(row.state?.receivedAt)).filter(Boolean).sort((left, right) => left - right)[0] || null

  const [token, setToken] = useState(0)
  const refreshAll = useCallback(() => { reload(); setToken(value => value + 1) }, [reload])
  const engine = useEngineHealthAcross(covered, { days: 7, refreshToken: token })
  const typing = useTypingReportsAcross(covered, { enabled: !loading, refreshToken: token })
  const activity = useCaseActivity(loading ? [] : covered, { refreshToken: token })

  const reports = useMemo(() => Object.values(typing.items).filter(Boolean), [typing.items])
  const quality = useMemo(() => answerQuality(Object.values(engine.items).filter(Boolean)), [engine.items])
  const mix = useMemo(() => modalityMix(reports), [reports])
  const kinds = useMemo(() => recordKinds(reports, rows), [reports, rows])
  const notes = useMemo(() => dataNotes(reports), [reports])
  const aside = useMemo(() => setAside(rows), [rows])
  const ready = useMemo(() => timeToReady(rows), [rows])
  const timelines = useMemo(() => caseTimelines(activity.results), [activity.results])
  // "Still reading" is also "not started yet": a read that has produced neither a result nor a failure has not finished, so a panel shows its skeleton rather than "nothing" for a moment.
  const typingLoading = loading || typing.pending || (!reports.length && !typing.failed.length)
  const timelineLoading = loading || activity.status !== 'ready' || (!activity.results.length && !activity.failures.length)
  const answersLoading = !quality.reported && !engine.failed.length
  const caseStateCounts = useMemo(() => caseStates(records, cases), [records, cases])

  useEffect(() => {
    if (!hasProcessing) return undefined
    const refreshWhenVisible = () => { if (globalThis.document?.visibilityState === 'visible') reload() }
    const timer = globalThis.setInterval(refreshWhenVisible, 15_000)
    globalThis.document?.addEventListener('visibilitychange', refreshWhenVisible)
    return () => {
      globalThis.clearInterval(timer)
      globalThis.document?.removeEventListener('visibilitychange', refreshWhenVisible)
    }
  }, [hasProcessing, reload])

  return (
    <AppShell>
      <main id="workspace-main" className="nx-ui nx-page nx-dash" tabIndex={-1}>
        <Hero rows={rows} loading={loading} lastUpdated={lastUpdated} refreshing={isRefreshing} onRefresh={refreshAll} />
        {!cases.length ? (
          <div className="nx-empty nx-card"><h3>No case yet</h3><div className="nx-empty__actions"><Link className="nx-btn nx-btn--primary" to="/cases/new">New case</Link></div></div>
        ) : (
          <>
            {unreadable > 0 ? (
              <p className="nx-callout nx-callout--warn" role="status"><CircleAlert aria-hidden="true" /><span>{formatNumber(unreadable)} of {formatNumber(cases.length)} not reporting. <button type="button" className="nx-link-button" onClick={reload}>Try again</button></span></p>
            ) : null}
            <ResumeStrip entries={resumeEntries(recent)} />
            <MetricStrip kpis={kpis} rows={rows} states={caseStateCounts} quality={quality} loading={loading} />
            <div className="nx-grid">
              <div className="nx-span-8"><TimelinePanel timelines={timelines} loading={timelineLoading} failures={activity.failures} /></div>
              <div className="nx-span-4"><NeedsPanel items={attention} byCase={reviewCases} notes={notes} loading={loading} notesLoading={typingLoading} /></div>
              <div className="nx-span-4"><EvidenceTypesPanel mix={mix} loading={typingLoading} /></div>
              <div className="nx-span-4"><RecordKindsPanel kinds={kinds} loading={typingLoading} /></div>
              <div className="nx-span-4"><AnswersPanel quality={quality} loading={answersLoading} /></div>
              <div className="nx-span-6"><SetAsidePanel data={aside} loading={loading} /></div>
              <div className="nx-span-6"><TimeToReadyPanel data={ready} loading={loading} /></div>
              <div className="nx-span-12"><CasesPanel rows={rows} loading={loading} /></div>
              <div className="nx-span-6"><QuestionsPanel recent={recent} /></div>
              <div className="nx-span-6"><LatestPanel items={latest} loading={loading} /></div>
            </div>
          </>
        )}
      </main>
    </AppShell>
  )
}
