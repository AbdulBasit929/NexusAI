import assert from 'node:assert/strict'
import test from 'node:test'

import { asrLanguageUploadMetadata, governedASRLanguage, intakeCapabilitySummary } from './analystIntakePolicy.js'

test('keeps automatic speech detection free of an implicit language claim', () => {
  assert.equal(governedASRLanguage(''), '')
  assert.deepEqual(asrLanguageUploadMetadata('auto'), {})
})

test('admits an explicit trusted Urdu language hint', () => {
  assert.equal(governedASRLanguage(' UR '), 'ur')
  assert.deepEqual(asrLanguageUploadMetadata('ur'), { asr_language: 'ur' })
})

test('fails closed for unsupported browser-supplied language choices', () => {
  assert.equal(governedASRLanguage('hi'), '')
  assert.deepEqual(asrLanguageUploadMetadata('urdu'), {})
})

test('summarizes non-planned formats from the live server capability response', () => {
  const summary = intakeCapabilitySummary({ families: [
    { support_level: 'operational', formats: ['csv', 'JSON'] },
    { support_level: 'limited', formats: ['pdf', 'csv'] },
    { support_level: 'planned', formats: ['unsupported MIME'] },
  ] }, 2)
  assert.equal(summary, 'Server-declared formats: CSV, JSON, and 1 more. Processing depth varies by evidence family.')
  assert.match(intakeCapabilitySummary(null), /reported by the server/)
})
