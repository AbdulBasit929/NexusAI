// The v1 backend governs a case as a collection. Keep that compatibility
// mapping here so UI routes never invent a second case identity.
export function collectionIdForCase(caseId) {
  return String(caseId || '').trim()
}

export function caseIdForCollection(collectionId) {
  return String(collectionId || '').trim()
}
