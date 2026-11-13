import { useCallback, useEffect, useMemo, useState } from 'react'
import { CircleAlert } from 'lucide-react'
import { configuredCaseIds } from '../../../lib/apiClient.js'
import { dashboardKpis, dashboardRowState, latestEvidenceActivity, parseTimestamp } from '../../../lib/dashboardCases.js'
import { useCaseActivity } from '../../../lib/useCaseActivity.js'
import { formatNumber } from '../../../lib/format.js'
import { summariseCase, useConfiguredCaseOverviews } from '../../../lib/useCaseOverview.js'
import { listDrafts, useQuestionHistoryAcross } from '../../../lib/workspaceState.js'
import { AppShell } from '../../shell/AppShell.jsx'
import { useEngineDailyAcross, useEngineHealthAcross, useTypingReportsAcross } from './data.js'
import { AnswersPanel, DataQualityPanel, EvidenceTypesPanel, RecordKindsPanel, TimeToReadyPanel, TimelinePanel } from './insights.jsx'
import { answerQuality, caseTimelines, dailyQuestions, dataNotes, modalityMix, needsYou, recordKinds, setAside, timeBasis, timeToReady } from './model.js'
import { CasesPanel } from './panels.jsx'
import { Indicators } from './indicators.jsx'
import { FirstRun, Hero, LatestPanel, NeedsPanel, ResumePanel, ServiceDown, latestItems, resumeEntries } from './sections.jsx'

// The most cases any one dashboard read covers: the heavy reads (the typing report, the timeline) are made for each case, and a workspace with a very long list should not start dozens of them.
const READ_LIMIT = 12

// The dashboard. A rail on the right says what needs you, where to pick up, and what just happened; the main column says where the workspace stands (five figures, the cases) and then draws
// it: when things happened, how the questions were answered, what the evidence holds, how good the data is, how long files take. Every figure is a count a service reported, a share of two such
// counts, or a median of dated jobs; a case that could not report is named, never counted as zero. The cheap reads (status, case names, answers) start at once; the heavy ones (typing
// report, timeline) start when every case has reported, and each panel fills as its read arrives.
export default function DashboardPage() {
  const cases = configuredCaseIds()
  const covered = useMemo(() => cases.slice(0, READ_LIMIT), [cases.join('|')]) // eslint-disable-line react-hooks/exhaustive-deps
  const recent = useQuestionHistoryAcross(cases)
  const { states: overviews, reload } = useConfiguredCaseOverviews(cases)
  useEffect(() => { globalThis.document.title = 'Dashboard · NexusAI' }, [])

  const rows = useMemo(() => cases.map((caseId, index) => {
    const state = overviews[caseId]
    return { caseId, index, state, status: dashboardRowState(state), summary: state?.data ? summariseCase(state.data) : null, activity: state?.data ? latestEvidenceActivity(state.data) : null }
  }), [cases, overviews])
  const kpis = useMemo(() => dashboardKpis(rows), [rows])
  const loading = rows.some(row => row.status === 'loading')
  const unreadable = rows.filter(row => row.status === 'unavailable').length
  const latest = useMemo(() => latestItems(rows, 5), [rows])
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
  const health = useMemo(() => Object.values(engine.items).filter(Boolean), [engine.items])
  const quality = useMemo(() => answerQuality(health), [health])
  // the per-day reads are made only for the cases that were asked something this week
  const asked = useMemo(() => health.filter(item => Number(item.totals?.questions) > 0).map(item => item.collection_id), [health])
  const dailyReads = useEngineDailyAcross(asked, { enabled: asked.length > 0, refreshToken: token })
  const daily = useMemo(() => (asked.length ? dailyQuestions(Object.values(dailyReads.items).filter(Boolean)) : null), [asked, dailyReads.items])
  const mix = useMemo(() => modalityMix(reports), [reports])
  const kinds = useMemo(() => recordKinds(reports, rows), [reports, rows])
  const notes = useMemo(() => dataNotes(reports), [reports])
  const basis = useMemo(() => timeBasis(reports), [reports])
  const aside = useMemo(() => setAside(rows), [rows])
  const ready = useMemo(() => timeToReady(rows), [rows])
  const timelines = useMemo(() => caseTimelines(activity.results), [activity.results])
  const indexed = useMemo(() => rows.reduce((sum, row) => sum + (Number(row.state?.data?.summary?.kb_assets_total) || 0), 0), [rows])
  const drafts = useMemo(() => listDrafts(cases), [cases, token]) // eslint-disable-line react-hooks/exhaustive-deps
  const needs = useMemo(() => needsYou({ rows, notes, history: recent, drafts }), [rows, notes, recent, drafts])
  // "Still reading" is also "not started yet": a read that has produced neither a result nor a failure has not finished, so a panel shows its skeleton rather than "nothing" for a moment.
  const typingLoading = loading || typing.pending || (!reports.length && !typing.failed.length)
  const timelineLoading = loading || activity.status !== 'ready' || (!activity.results.length && !activity.failures.length)
  const answersLoading = !quality.reported && !engine.failed.length

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
        {!cases.length || (!loading && unreadable === cases.length) ? (
          <>
            <Hero rows={rows} loading={loading} lastUpdated={lastUpdated} refreshing={isRefreshing} onRefresh={refreshAll} />
            {!cases.length ? <FirstRun /> : <ServiceDown onRetry={refreshAll} />}
          </>
        ) : (
          <>
            <div className="nx-first">
              <Hero rows={rows} loading={loading} lastUpdated={lastUpdated} refreshing={isRefreshing} onRefresh={refreshAll} />
              {unreadable > 0 ? (
                <p className="nx-callout nx-callout--warn" role="status"><CircleAlert aria-hidden="true" /><span>{formatNumber(unreadable)} of {formatNumber(cases.length)} not reporting. <button type="button" className="nx-link-button" onClick={reload}>Try again</button></span></p>
              ) : null}
              <Indicators kpis={kpis} rows={rows} quality={quality} daily={daily} needs={needs} loading={loading} />
              <div className="nx-board nx-board--hero" data-band="hero">
                <div className="nx-cell nx-cell--timeline"><TimelinePanel timelines={timelines} loading={timelineLoading} failures={activity.failures} /></div>
                <div className="nx-cell nx-cell--answers"><AnswersPanel quality={quality} daily={daily} loading={answersLoading} /></div>
                <div className="nx-cell nx-cell--types"><EvidenceTypesPanel mix={mix} indexed={indexed} loading={typingLoading} /></div>
                <div className="nx-cell nx-cell--kinds"><RecordKindsPanel kinds={kinds} loading={typingLoading} /></div>
                <div className="nx-cell nx-cell--quality"><DataQualityPanel aside={aside} basis={basis} loading={loading} basisLoading={typingLoading} /></div>
              </div>
            </div>
            <div className="nx-board nx-board--work" data-band="work">
              <div className="nx-cell"><NeedsPanel needs={needs} loading={loading} notesLoading={typingLoading} /></div>
              <div className="nx-cell"><ResumePanel entries={resumeEntries(recent)} /></div>
              <div className="nx-cell"><LatestPanel items={latest} loading={loading} /></div>
              <div className="nx-cell"><TimeToReadyPanel data={ready} loading={loading} /></div>
            </div>
            <CasesPanel rows={rows} loading={loading} />
          </>
        )}
      </main>
    </AppShell>
  )
}
