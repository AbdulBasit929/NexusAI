// Runtime configuration, read by the app at startup and overridable by the
// serving shell before this file loads.
//
// The default is the SAME-ORIGIN PROXY at /api, which is the only safe
// deployment shape: the browser holds no credential, the proxy attaches the
// forensic API key server-side, and there is no cross-origin request to need
// CORS in the first place.
//
// apiBaseUrl must remain a same-origin path. The client rejects absolute or
// protocol-relative URLs so browser-direct credential mode cannot quietly
// return through an instance override.
// `caseIds` names the collections this instance may open. A case IS a
// collection in the v1 backend (`caseScope.js`) -- there is no case entity and
// no list-cases endpoint, so the set of cases is configuration, not discovery.
// Naming them here is not simulated data: every one is a real retained
// collection and the workspace reads its status, evidence and answers live.
// An instance overrides this whole object before this file loads.
window.__INVESTIGATION_WORKSPACE_CONFIG__ = window.__INVESTIGATION_WORKSPACE_CONFIG__ || {
  apiBaseUrl: '/api',
  capabilities: { hybrid_query: true },
  caseIds: ['nexusai-multimodal-product-acceptance', 'nexusai-forensic-demo'],
}

// DO NOT add `collectionId` here alongside `caseIds`. `requestContext()` in
// apiClient.js resolves `config.collectionId || collectionIdForCase(caseId)`,
// so a configured `collectionId` PINS EVERY CASE to that one collection and
// the open case is silently ignored. Measured 2026-09-27: with it set, asking
// "How many CDR records do we have in this case?" inside
// `nexusai-forensic-demo` answered 5,000 -- the CDR count of the OTHER
// collection -- instead of 8,642. The page still said the right case name, so
// nothing on screen revealed which evidence had actually been queried.
