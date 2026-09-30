import assert from 'node:assert/strict'
import test from 'node:test'

import { investigationWorkspaceIdentity } from './investigationWorkspace.js'

test('keeps one neutral, task-oriented investigation workspace identity', () => {
  assert.equal(investigationWorkspaceIdentity.name, 'Investigation Workspace')
  assert.equal(investigationWorkspaceIdentity.name.includes('NexusAI'), false)
  assert.equal(investigationWorkspaceIdentity.descriptor, 'Evidence questions and answers')
  assert.equal(/model|agent|tool|template|admin/i.test(investigationWorkspaceIdentity.descriptor), false)
})
