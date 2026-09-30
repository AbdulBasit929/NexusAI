const presentations = Object.freeze({
  structured: { kind: 'structured', category: 'structured', label: 'Structured evidence', shortLabel: 'Structured', icon: 'fa-table-list' },
  document: { kind: 'document', category: 'documents', label: 'Document evidence', shortLabel: 'Document', icon: 'fa-file-lines' },
  text: { kind: 'text', category: 'documents', label: 'Text evidence', shortLabel: 'Text', icon: 'fa-file-lines' },
  image: { kind: 'image', category: 'images', label: 'Image evidence', shortLabel: 'Image', icon: 'fa-image' },
  audio: { kind: 'audio', category: 'audio', label: 'Audio evidence', shortLabel: 'Audio', icon: 'fa-waveform-lines' },
  video: { kind: 'video', category: 'video', label: 'Video evidence', shortLabel: 'Video', icon: 'fa-film' },
  archive: { kind: 'archive', category: 'archives', label: 'Archive evidence', shortLabel: 'Archive', icon: 'fa-file-zipper' },
  unknown: { kind: 'unknown', category: 'unknown', label: 'Unknown evidence type', shortLabel: 'Unknown', icon: 'fa-file-circle-question' },
})

const extensions = Object.freeze({
  audio: /\.(wav|mp3|m4a|aac|flac|ogg|opus|wma|amr)$/,
  video: /\.(mp4|mov|mkv|webm|avi|m4v|mpg|mpeg|mts|m2ts|3gp)$/,
  image: /\.(png|jpe?g|webp|gif|bmp|tiff?|heic|heif|dng|raw|svg)$/,
  document: /\.(pdf|docx?|rtf|odt|pptx?|odp|epub|eml|msg|html?)$/,
  text: /\.(txt|md|log|srt|vtt|ass|ssa|ya?ml)$/,
  archive: /\.(zip|7z|rar|tar|gz|tgz|bz2|xz)$/,
  structured: /\.(csv|tsv|jsonl?|ndjson|parquet|xlsx?|xlsm|ods|xml|avro|orc|arrow|feather|sqlite3?|db|sql|pcapng?|cap|evtx)$/,
})

function declaredKind(value) {
  const text = String(value || '').trim().toLowerCase()
  if (!text) return ''
  if (/(?:^|[_. -])audio|speech|transcript|asr/.test(text)) return 'audio'
  if (/(?:^|[_. -])video|cctv/.test(text)) return 'video'
  if (/(?:^|[_. -])image|photo|ocr|face|anpr/.test(text)) return 'image'
  if (/archive|compressed|zip|tar/.test(text)) return 'archive'
  if (/\bpdf\b|document|docx?|presentation|slides|email/.test(text)) return 'document'
  if (/plain.?text|textual|subtitle|markdown/.test(text) || text === 'text') return 'text'
  if (/structured|tabular|spreadsheet|records?|cdr|ipdr|subscriber|tower|location|transaction|access|network|database|generic/.test(text)) return 'structured'
  return ''
}

function mimeKind(value) {
  const mime = String(value || '').trim().toLowerCase()
  if (mime.startsWith('audio/')) return 'audio'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('image/')) return 'image'
  if (mime === 'application/pdf' || /officedocument|msword|presentation/.test(mime)) return 'document'
  if (mime.startsWith('text/')) return 'text'
  if (/zip|compressed|tar|rar|7z/.test(mime)) return 'archive'
  if (/json|csv|spreadsheet|parquet|xml|sqlite/.test(mime)) return 'structured'
  return ''
}

function filenameKind(item) {
  const filename = String(item?.original_filename || item?.source_file || item?.filename || '').split(/[\\/]/).pop().toLowerCase()
  for (const [kind, pattern] of Object.entries(extensions)) {
    if (pattern.test(filename)) return kind
  }
  return ''
}

// Authoritative server classification wins. MIME and filename are only fallbacks
// for legacy records whose classification fields have not yet been populated.
export function resolveEvidenceModality(item = {}) {
  const declared = [item.modality, item.evidence_modality, item.evidence_family, item.family, item.detected_type, item.record_type]
  for (const value of declared) {
    const kind = declaredKind(value)
    if (kind) return presentations[kind]
  }
  const kind = mimeKind(item.mime_type || item.content_type || item.metadata?.mime_type) || filenameKind(item) || 'unknown'
  return presentations[kind]
}

export function evidenceModalityIcon(item = {}) {
  return resolveEvidenceModality(item).icon
}
