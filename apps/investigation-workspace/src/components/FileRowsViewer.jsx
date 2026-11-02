import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react'
import { getFileRows } from '../lib/apiClient.js'
import { readSpreadsheetDate } from '../lib/spreadsheetDates.js'
import { LanguageText } from './AnalystComponents.jsx'

// The file as the analyst was given it (api/forensic_records/evidence_file_rows.go): its rows, a page at a time, under the file's own column names and in the file's own column order, with the row
// number of the file on the left. The row an answer cited is marked and brought into view. Nothing here is the system's own column: no identifier, no hash. When the service has no such route (an
// older service), the viewer shows what it was given instead, so the page never goes blank.
const PAGE = 50
const number = value => new Intl.NumberFormat('en-US').format(Number(value) || 0)

// The row to start from so that the cited row sits a little below the top of the page.
const startFor = cited => (cited > 0 ? Math.max(1, cited - 8) : 1)

export function FileRowsViewer({ caseId, evidenceId, citedRow, onRowSelect, fetchRows = getFileRows, fallback = null }) {
  const cited = Number.parseInt(citedRow, 10) || 0
  const [selected, setSelected] = useState(0)
  const [from, setFrom] = useState(startFor(cited))
  const [state, setState] = useState({ loading: true })
  const [hideEmpty, setHideEmpty] = useState(false)
  const [jump, setJump] = useState(cited ? String(cited) : '')
  const scroller = useRef(null)
  const citedRef = useRef(null)
  const scrolledTo = useRef(0)

  useEffect(() => { setFrom(startFor(cited)); setJump(cited ? String(cited) : ''); scrolledTo.current = 0 }, [evidenceId, cited])
  useEffect(() => {
    const controller = new AbortController()
    setState(previous => ({ ...previous, loading: true, error: undefined }))
    fetchRows({ caseId, evidenceId, from, limit: PAGE, signal: controller.signal })
      .then(data => setState({ data, loading: false }))
      .catch(error => { if (error?.name !== 'AbortError') setState({ error, loading: false }) })
    return () => controller.abort()
  }, [caseId, evidenceId, from, fetchRows])

  const data = state.data
  // bring the cited row into view once per citation, inside the table's own frame (never scrolling the page)
  useEffect(() => {
    const frame = scroller.current
    const row = citedRef.current
    if (!frame || !row || !cited || scrolledTo.current === cited) return
    scrolledTo.current = cited
    frame.scrollTop = Math.max(0, row.offsetTop - frame.clientHeight / 3)
  }, [data, cited])

  const visible = useMemo(() => {
    if (!data?.columns?.length) return { columns: [], rows: [] }
    const keep = data.columns.map((_, index) => !hideEmpty || data.rows.some(row => row.cells[index]))
    return {
      columns: data.columns.filter((_, index) => keep[index]),
      rows: data.rows.map(row => ({ ...row, cells: row.cells.filter((_, index) => keep[index]) })),
    }
  }, [data, hideEmpty])

  // a row the analyst picks is handed to the inspector under the file's own column names
  function pick(row) {
    setSelected(row.row_number)
    if (!onRowSelect) return
    const values = { __row: row.row_number }
    visible.columns.forEach((column, index) => { values[column.name] = row.cells[index] === 'withheld' ? '' : row.cells[index] })
    onRowSelect(values, [{ key: '__row', label: 'Row in the file' }, ...visible.columns.map(column => ({ key: column.name, label: column.name }))])
  }
  const go = useCallback(row => setFrom(Math.max(1, row)), [])
  function onJump(event) {
    event.preventDefault()
    const row = Number.parseInt(jump, 10)
    if (row > 0) { scrolledTo.current = 0; go(startFor(row)) }
  }

  if (state.error && !data) return fallback
  if (data && !data.rows?.length && data.total === 0) return fallback
  const first = data?.rows?.[0]?.row_number
  const last = data?.rows?.[data.rows.length - 1]?.row_number
  const records = data?.row_label === 'Record'
  const hasPrevious = data && first > data.first_row
  const hasNext = data && last < data.last_row
  const pageStart = row => Math.max(data.first_row, row)
  return (
    <section className="frv" aria-labelledby="frv-title">
      <header className="frv__head">
        <div>
          <h2 id="frv-title">{records ? 'The records of the file' : 'The rows of the file'}</h2>
          <p className="frv__count" aria-live="polite">
            {!data ? 'Reading the rows of the file…' : records
              ? <>Records <b>{number(first)}–{number(last)}</b> · <b>{number(data.total)}</b> records in this file, under its own headings</>
              : <>Rows <b>{number(first)}–{number(last)}</b> of <b>{number(data.total)}</b>, as the file has them</>}
          </p>
        </div>
        <div className="frv__tools">
          <form className="frv__jump" onSubmit={onJump}>
            <label><span>{records ? 'Go to record' : 'Go to row'}</span><input inputMode="numeric" value={jump} onChange={event => setJump(event.target.value)} placeholder="e.g. 93" /></label>
            <button type="submit">Go</button>
          </form>
          <label className="frv__toggle"><input type="checkbox" checked={hideEmpty} onChange={event => setHideEmpty(event.target.checked)} /><span>Hide empty columns</span></label>
          <div className="frv__pager" role="group" aria-label="Pages of the file">
            <button type="button" disabled={!hasPrevious} onClick={() => go(data.first_row)} aria-label="First rows"><ChevronsLeft aria-hidden="true" /></button>
            <button type="button" disabled={!hasPrevious} onClick={() => go(pageStart(first - PAGE))} aria-label="Previous rows"><ChevronLeft aria-hidden="true" /></button>
            <button type="button" disabled={!hasNext} onClick={() => go(last + 1)} aria-label="Next rows"><ChevronRight aria-hidden="true" /></button>
            <button type="button" disabled={!hasNext} onClick={() => go(data.last_row - PAGE + 1)} aria-label="Last rows"><ChevronsRight aria-hidden="true" /></button>
          </div>
        </div>
      </header>
      {state.error ? <p className="frv__error" role="alert">These rows could not be read. Try again, or go to another row.</p> : null}
      <div className={`frv__frame${state.loading ? ' is-loading' : ''}`} ref={scroller} role="region" aria-label="Rows of the file" tabIndex={0}>
        {data ? (
          <table className="frv__table">
            <thead>
              <tr>
                <th scope="col" className="frv__rowhead">{records ? 'Record' : 'Row'}</th>
                {visible.columns.map(column => (
                  <th scope="col" key={column.name} title={column.letter ? `Column ${column.letter} of the workbook` : undefined}>
                    {column.letter ? <span className="frv__letter">{column.letter}</span> : null}
                    <LanguageText>{column.name}</LanguageText>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {visible.rows.map(row => {
                const isCited = row.row_number === cited
                const classes = [isCited ? 'is-cited' : '', row.row_number === selected ? 'is-selected' : ''].filter(Boolean).join(' ')
                return (
                  <tr key={row.row_number} ref={isCited ? citedRef : undefined} className={classes || undefined} aria-current={isCited ? 'true' : undefined} tabIndex={onRowSelect ? 0 : undefined}
                    onClick={onRowSelect ? () => pick(row) : undefined} onKeyDown={onRowSelect ? event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); pick(row) } } : undefined}>
                    <th scope="row" className="frv__rowhead">{row.row_number}{isCited ? <span className="frv__flag">Cited</span> : null}</th>
                    {row.cells.map((cell, index) => {
                      const date = cell && cell !== 'withheld' ? readSpreadsheetDate(visible.columns[index].name, cell) : ''
                      const title = cell === 'withheld' ? 'Not shown: removed to protect personal details' : date ? `The file holds ${cell}, a spreadsheet date` : undefined
                      return (
                        <td key={visible.columns[index].name} className={cell === 'withheld' ? 'is-withheld' : date ? 'is-date' : cell ? undefined : 'is-empty'} title={title}>
                          {cell === 'withheld' ? 'withheld' : date ? <LanguageText>{date}</LanguageText> : cell ? <LanguageText>{cell}</LanguageText> : '—'}
                        </td>
                      )
                    })}
                  </tr>
                )
              })}
            </tbody>
          </table>
        ) : <p className="frv__waiting" role="status">Reading the rows of the file…</p>}
      </div>
      {data && (data.duplicate_rows || data.rejected_rows) ? (
        <p className="frv__note">
          {data.duplicate_rows ? `${number(data.duplicate_rows)} repeated ${data.duplicate_rows === 1 ? 'row was' : 'rows were'} not stored. ` : ''}
          {data.rejected_rows ? `${number(data.rejected_rows)} ${data.rejected_rows === 1 ? 'row was' : 'rows were'} not read.` : ''}
        </p>
      ) : null}
    </section>
  )
}
