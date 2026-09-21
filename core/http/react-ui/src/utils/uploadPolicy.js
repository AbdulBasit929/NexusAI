export const MAX_DATA_FILE_BYTES = 300 * 1024 * 1024
export const MAX_IMAGE_BYTES = 64 * 1024 * 1024
const IMAGE_EXTENSIONS = new Set(['bmp', 'gif', 'heic', 'heif', 'jpeg', 'jpg', 'png', 'tif', 'tiff', 'webp'])

export function uploadPreflight(file) {
  const extension = String(file.name.split('.').pop() || '').toLowerCase()
  if (IMAGE_EXTENSIONS.has(extension) && file.size > MAX_IMAGE_BYTES) {
    return { blocked: false, message: 'Large image selected. The server will apply its current image safety and upload limits.' }
  }
  if (file.size > MAX_DATA_FILE_BYTES) {
    return { blocked: false, message: 'Large file selected. The server will make the authoritative admission decision.' }
  }
  if (file.size === 0) return { blocked: false, message: 'Empty file; the server will make the final admission decision.' }
  return { blocked: false, message: 'Selected, not uploaded. Click Add selected data to begin.' }
}

export function uploadHttpError(status, payload) {
  const supplied = typeof payload?.error === 'string' ? payload.error : payload?.error?.message
  const message = status === 413
    ? 'The server rejected this upload as too large (HTTP 413). General/video files support up to 300 MiB only when the matching server limit is deployed; images retain their 64 MiB safety limit. Your local file was not removed.'
    : (supplied || `HTTP ${status}`)
  const error = new Error(message)
  error.status = status
  error.body = payload
  return error
}

export function uploadConnectionError() {
  return new Error('The upload connection failed. The server may have closed the connection because of its upload-size limit, or the connection was interrupted. Check Data before retrying; registration may be uncertain. Your local file was not removed.')
}
