import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AddDataDropzone } from './AddDataDropzone.jsx'
import { ProcessingWait } from './ProcessingWait.jsx'

// Evidence intake has two phases with different honesty requirements, and the
// tests exist to keep them apart:
//
//   TRANSFER   real, measurable -- may show a bar
//   PROCESSING no honest percentage -- must show a STATE, never a bar
//
// A simulated upload or an invented percentage would be a truthfulness defect
// in a forensic product, not a cosmetic one.

function fileNamed(name, size = 12) {
  const file = new File(['x'.repeat(size)], name, { type: 'text/csv' })
  return file
}

describe('AddDataDropzone', () => {
  it('sends the chosen file to the real intake endpoint and reports acceptance', async () => {
    const uploader = vi.fn().mockResolvedValue({ evidence_id: 'ev-1' })
    const onAccepted = vi.fn()
    render(<AddDataDropzone caseId="case-a" uploader={uploader} onAccepted={onAccepted} />)

    await userEvent.upload(
      screen.getByLabelText(/choose evidence files/i),
      fileNamed('calls.csv'),
    )

    await waitFor(() => expect(uploader).toHaveBeenCalledTimes(1))
    expect(uploader.mock.calls[0][0]).toMatchObject({ caseId: 'case-a', sourceEntry: 'calls.csv' })
    expect(onAccepted).toHaveBeenCalledWith({ evidence_id: 'ev-1' })
    await screen.findByText(/accepted — processing has started/i)
  })

  it('shows a refusal as a real answer, with its reference, not as a crash', async () => {
    const refusal = Object.assign(new Error('That file is larger than this case accepts.'), {
      reference: 'HTTP-413',
    })
    const uploader = vi.fn().mockRejectedValue(refusal)
    render(<AddDataDropzone caseId="case-a" uploader={uploader} />)

    await userEvent.upload(screen.getByLabelText(/choose evidence files/i), fileNamed('huge.png'))

    await screen.findByText(/not accepted/i)
    expect(await screen.findByText(/larger than this case accepts/i)).toBeTruthy()
    expect(screen.getByText(/HTTP-413/)).toBeTruthy()
  })

  it('never claims progress it has not measured', async () => {
    // A queued file has transferred nothing, so no progress element may exist
    // for it: a bar sitting at zero still asserts "this is underway".
    let release
    const uploader = vi.fn(() => new Promise(resolve => { release = resolve }))
    const { container } = render(<AddDataDropzone caseId="case-a" uploader={uploader} />)

    await userEvent.upload(screen.getByLabelText(/choose evidence files/i), fileNamed('a.csv'))
    await waitFor(() => expect(uploader).toHaveBeenCalled())

    // While sending, the bar reflects the reported fraction only.
    const bars = container.querySelectorAll('progress')
    expect(bars.length).toBeLessThanOrEqual(1)

    release?.({ evidence_id: 'ev-2' })
    await screen.findByText(/accepted — processing has started/i)
    // Once accepted, transfer is done and processing has begun -- and
    // processing has no honest percentage, so no bar may remain.
    expect(container.querySelectorAll('progress').length).toBe(0)
  })

  it('aborts the actual in-flight upload when the analyst cancels it', async () => {
    const uploader = vi.fn(({ signal }) => new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => {
        const error = new Error('cancelled')
        error.name = 'AbortError'
        reject(error)
      })
    }))
    render(<AddDataDropzone caseId="case-a" uploader={uploader} />)

    await userEvent.upload(screen.getByLabelText(/choose evidence files/i), fileNamed('cancel-me.csv'))
    await waitFor(() => expect(uploader).toHaveBeenCalled())
    await userEvent.click(screen.getByRole('button', { name: 'Cancel cancel-me.csv' }))

    expect(await screen.findByText('Upload cancelled')).toBeTruthy()
    expect(uploader.mock.calls[0][0].signal.aborted).toBe(true)
    expect(screen.getByRole('button', { name: 'Remove cancel-me.csv' })).toBeTruthy()
  })
})

describe('ProcessingWait', () => {
  const evidence = statuses => ({
    recent_evidence: statuses.map((processing_status, index) => ({
      evidence_id: `ev-${index}`,
      source_file: `file-${index}.csv`,
      modality: 'structured_records',
      processing_status,
    })),
  })

  it('says what is not ready yet, and never calls unprocessed evidence a result', async () => {
    const fetcher = vi.fn().mockResolvedValue(evidence(['completed', 'running', 'queued']))
    render(<ProcessingWait caseId="case-a" fetcher={fetcher} pollMs={10_000} />)

    expect(await screen.findByText(/2 of 3 evidence items are still being processed/i)).toBeTruthy()
    expect(screen.queryByText(/no results/i)).toBeNull()
  })

  it('distinguishes failed from merely unfinished', async () => {
    const fetcher = vi.fn().mockResolvedValue(evidence(['completed', 'failed']))
    render(<ProcessingWait caseId="case-a" fetcher={fetcher} pollMs={10_000} />)

    expect(await screen.findByText(/could not be processed/i)).toBeTruthy()
  })

  it('reports readiness once every item has settled', async () => {
    const onReady = vi.fn()
    const fetcher = vi.fn().mockResolvedValue(evidence(['completed', 'completed']))
    render(<ProcessingWait caseId="case-a" fetcher={fetcher} pollMs={10_000} onReady={onReady} />)

    expect(await screen.findByText(/all 2 evidence items have finished processing/i)).toBeTruthy()
    await waitFor(() => expect(onReady).toHaveBeenCalled())
  })

  it('distinguishes an empty case from an unready one', async () => {
    const fetcher = vi.fn().mockResolvedValue({ recent_evidence: [] })
    render(<ProcessingWait caseId="case-a" fetcher={fetcher} pollMs={10_000} />)

    expect(await screen.findByText(/no evidence has been added to this case yet/i)).toBeTruthy()
  })

  it('keeps checking when the status read itself fails', async () => {
    // Failing to READ the status is not a failure of the processing, and
    // stopping would strand the analyst on a stale screen.
    const fetcher = vi.fn().mockRejectedValue(Object.assign(new Error('nope'), { reference: 'HTTP-503' }))
    render(<ProcessingWait caseId="case-a" fetcher={fetcher} pollMs={10_000} />)

    expect(await screen.findByText(/status could not be read just now; still checking/i)).toBeTruthy()
  })
})
