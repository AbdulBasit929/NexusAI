import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import YAML from 'yaml'
import { MAX_DATA_FILE_BYTES, MAX_IMAGE_BYTES, uploadPreflight, uploadHttpError, uploadConnectionError } from './uploadPolicy.js'

test('browser size hints never replace the server admission decision', () => {
  for (const name of ['video.mp4', 'data.csv', 'archive.zip', 'source.bin']) {
    assert.equal(uploadPreflight({ name, size: MAX_DATA_FILE_BYTES }).blocked, false)
    assert.equal(uploadPreflight({ name, size: MAX_DATA_FILE_BYTES + 1 }).blocked, false)
  }
  assert.equal(uploadPreflight({ name: 'video-v3.mp4', size: 184407144 }).blocked, false)
  assert.equal(MAX_DATA_FILE_BYTES, 314572800)
})

test('image size remains an advisory hint including uppercase extensions', () => {
  for (const name of ['image.png', 'image.JPG', 'source.TIFF']) {
    assert.equal(uploadPreflight({ name, size: MAX_IMAGE_BYTES }).blocked, false)
    assert.equal(uploadPreflight({ name, size: MAX_IMAGE_BYTES + 1 }).blocked, false)
  }
  assert.match(uploadPreflight({ name: 'image.png', size: MAX_IMAGE_BYTES + 1 }).message, /server/)
})

test('selection text explains submission; empty files retain server admission', () => {
  assert.match(uploadPreflight({ name: 'file.csv', size: 1 }).message, /Click Add selected data/)
  assert.match(uploadPreflight({ name: 'file.csv', size: 0 }).message, /server/)
})

test('413 always explains size refusal and preserves status and body', () => {
  for (const payload of [null, { error: 'Request Entity Too Large' }, { error: { message: 'large' } }]) {
    const error = uploadHttpError(413, payload)
    assert.match(error.message, /HTTP 413/)
    assert.match(error.message, /300 MiB/)
    assert.equal(error.status, 413)
    assert.equal(error.body, payload)
  }
})

test('other HTTP errors preserve server explanation and safe fallback', () => {
  assert.equal(uploadHttpError(422, { error: { message: 'bad format' } }).message, 'bad format')
  assert.equal(uploadHttpError(403, { error: 'denied' }).message, 'denied')
  assert.equal(uploadHttpError(500, null).message, 'HTTP 500')
})

test('connection errors acknowledge uncertain registration, without claiming confirmed size rejection', () => {
  assert.match(uploadConnectionError().message, /may have/)
  assert.match(uploadConnectionError().message, /Check Data before retrying/)
  assert.match(uploadConnectionError().message, /local file was not removed/)
})

test('prepared request envelope exceeds the file ceiling without touching other services', () => {
  const config = YAML.parse(fs.readFileSync(new URL('../../../../../docker-compose.nxmmr-upload-300m.yaml', import.meta.url), 'utf8'))
  assert.deepEqual(Object.keys(config.services), ['api'])
  assert.deepEqual(config.services.api, { environment: { LOCALAI_UPLOAD_LIMIT: '320' } })
  assert.ok(Number(config.services.api.environment.LOCALAI_UPLOAD_LIMIT) * 1024 * 1024 > MAX_DATA_FILE_BYTES + 1024 * 1024)
})
