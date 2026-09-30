import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import { LineagePanel } from './EvidenceViewers.jsx'

describe('evidence lineage', () => {
  test('shows recorded producer versions and names missing producer data explicitly', () => {
    render(<LineagePanel detail={{
      item: { original_filename: 'source.pdf', version_id: 'evidence-version-1' },
      processing_runs: [{ run_id: 'run-1', model_id: 'recorded-reader', model_revision: 'revision-7' }],
      derived_artifacts: [
        { artifact_id: 'artifact-1', artifact_type: 'ocr', run_id: 'run-1', confidence: 0.91 },
        { artifact_id: 'artifact-2', artifact_type: 'transcript', confidence: 0.42 },
      ],
    }} />)

    expect(screen.getByText('recorded-reader')).toBeInTheDocument()
    expect(screen.getByText('revision-7')).toBeInTheDocument()
    expect(screen.getByText('Producer not recorded')).toBeInTheDocument()
    expect(screen.getByText('Version not recorded')).toBeInTheDocument()
    expect(screen.getAllByText('Recorded producer')).toHaveLength(2)
  })
})
