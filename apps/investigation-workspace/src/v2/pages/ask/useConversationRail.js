import { useCallback, useEffect, useState } from 'react'
import { hideConversation, isNotInstalled, listConversations, restoreConversation, updateConversation } from '../../../lib/conversationsApi.js'

// The history rail of one case, read from the conversation service: pinned first, then newest, with the words of a search matched on the server (it folds Urdu script, Roman Urdu and
// Urdu digits, so a person finds a question however it was typed). `status` is `loading`, `ready`, `missing` (migration 034 is not applied: the page keeps the history of this browser and
// says so) or `error`. A search keeps the list on screen while the next one is read.
export function useConversationRail(caseId, { query = '', enabled = true } = {}) {
  const [state, setState] = useState({ status: 'loading', items: [], next: '', error: '' })
  const [token, setToken] = useState(0)
  const reload = useCallback(() => setToken(value => value + 1), [])

  useEffect(() => {
    if (!enabled || !caseId) return undefined
    const gate = new AbortController()
    const timer = globalThis.setTimeout(() => {
      listConversations({ caseId, q: query.trim() || undefined, limit: 30, signal: gate.signal })
        .then(page => setState({ status: 'ready', items: page.items || [], next: page.next_cursor || '', error: '' }))
        .catch(error => {
          if (error.name === 'AbortError') return
          setState(previous => ({ ...previous, status: isNotInstalled(error) ? 'missing' : 'error', error: error.message || '' }))
        })
    }, query.trim() ? 250 : 0)
    return () => { gate.abort(); globalThis.clearTimeout(timer) }
  }, [caseId, query, enabled, token])

  const loadMore = useCallback(async () => {
    if (!state.next) return
    const page = await listConversations({ caseId, q: query.trim() || undefined, limit: 30, cursor: state.next })
    setState(previous => ({ ...previous, items: [...previous.items, ...(page.items || [])], next: page.next_cursor || '' }))
  }, [caseId, query, state.next])

  const change = useCallback(async (threadId, patch) => { await updateConversation({ caseId, threadId, patch }); reload() }, [caseId, reload])
  return {
    ...state,
    reload,
    loadMore,
    pin: (threadId, pinned) => change(threadId, { pinned }),
    rename: (threadId, title) => change(threadId, { title: title.trim() ? title.trim() : null }),
    archive: (threadId, archived) => change(threadId, { archived }),
    hide: async (threadId) => { await hideConversation({ caseId, threadId }); reload() },
    restore: async (threadId) => { await restoreConversation({ caseId, threadId }); reload() },
  }
}
