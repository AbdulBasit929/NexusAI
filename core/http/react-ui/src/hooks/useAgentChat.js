import { useState, useCallback, useEffect } from 'react'
import { generateId } from '../utils/format'
import { useDebouncedEffect } from './useDebounce'

const STORAGE_KEY_PREFIX = 'localai_agent_chats_'
const CHAT_HISTORY_SCHEMA_VERSION = 1

function normalizeConversation(conversation) {
  return {
    ...conversation,
    savedAt: Number(conversation.savedAt) || null,
    persistenceAuthority: 'browser_local',
    schemaVersion: CHAT_HISTORY_SCHEMA_VERSION,
  }
}

function storageKey(agentName, scope = '') {
  return STORAGE_KEY_PREFIX + agentName + (scope ? `__case_${scope}` : '')
}

function loadConversations(agentName, scope = '') {
  try {
    const stored = localStorage.getItem(storageKey(agentName, scope))
    if (stored) {
      const data = JSON.parse(stored)
      if (data && Array.isArray(data.conversations)) {
        return { ...data, conversations: data.conversations.map(normalizeConversation) }
      }
    }
  } catch (_e) {
    localStorage.removeItem(storageKey(agentName, scope))
  }
  return null
}

function saveConversations(agentName, conversations, activeId, scope = '') {
  try {
    const data = {
      contractVersion: 'browser-agent-history/v1',
      conversations: conversations.map(c => ({
        id: c.id,
        name: c.name,
        messages: c.messages,
        createdAt: c.createdAt,
        updatedAt: c.updatedAt,
        savedAt: c.savedAt || null,
        persistenceAuthority: 'browser_local',
        schemaVersion: CHAT_HISTORY_SCHEMA_VERSION,
      })),
      activeId,
      lastSaved: Date.now(),
    }
    localStorage.setItem(storageKey(agentName, scope), JSON.stringify(data))
  } catch (err) {
    if (err.name === 'QuotaExceededError' || err.code === 22) {
      console.warn('localStorage quota exceeded for agent chats')
    }
  }
}

function createConversation() {
  return {
    id: generateId(),
    name: 'New Chat',
    messages: [],
    createdAt: Date.now(),
    updatedAt: Date.now(),
    savedAt: null,
    persistenceAuthority: 'browser_local',
    schemaVersion: CHAT_HISTORY_SCHEMA_VERSION,
  }
}

export function useAgentChat(agentName, scope = '', enabled = true) {
  const requestedScopeKey = `${agentName}::${scope}`
  const [conversations, setConversations] = useState(() => {
    const stored = loadConversations(agentName, scope)
    if (stored && stored.conversations.length > 0) return stored.conversations
    return [createConversation()]
  })

  const [activeId, setActiveId] = useState(() => {
    const stored = loadConversations(agentName, scope)
    if (stored && stored.activeId) return stored.activeId
    return conversations[0]?.id
  })
  const [loadedScopeKey, setLoadedScopeKey] = useState(() => enabled ? requestedScopeKey : '')

  const scopeReady = enabled && loadedScopeKey === requestedScopeKey
  const activeConversation = scopeReady ? conversations.find(c => c.id === activeId) || conversations[0] : undefined

  useEffect(() => {
    if (!enabled) return
    const stored = loadConversations(agentName, scope)
    const next = stored?.conversations?.length ? stored.conversations : [createConversation()]
    setConversations(next)
    setActiveId(stored?.activeId || next[0]?.id)
    setLoadedScopeKey(requestedScopeKey)
  }, [agentName, scope, enabled, requestedScopeKey])

  useDebouncedEffect(() => {
    if (scopeReady) saveConversations(agentName, conversations, activeId, scope)
  }, [agentName, scope, conversations, activeId, scopeReady])

  // Save immediately on unmount
  useEffect(() => {
    return () => {
      if (scopeReady) saveConversations(agentName, conversations, activeId, scope)
    }
  }, [agentName, scope, conversations, activeId, scopeReady])

  const addConversation = useCallback(() => {
    if (!scopeReady) return null
    const conv = createConversation()
    setConversations(prev => [conv, ...prev])
    setActiveId(conv.id)
    return conv
  }, [scopeReady])

  const switchConversation = useCallback((id) => {
    if (!scopeReady) return
    setActiveId(id)
  }, [scopeReady])

  const deleteConversation = useCallback((id) => {
    if (!scopeReady) return
    setConversations(prev => {
      if (prev.length <= 1) return prev
      const filtered = prev.filter(c => c.id !== id)
      const newActiveId = id === activeId && filtered.length > 0 ? filtered[0].id : activeId
      if (id === activeId) {
        setActiveId(newActiveId)
      }
      saveConversations(agentName, filtered, newActiveId, scope)
      return filtered
    })
  }, [activeId, agentName, scope, scopeReady])

  const deleteAllConversations = useCallback(() => {
    if (!scopeReady) return
    const conv = createConversation()
    setConversations([conv])
    setActiveId(conv.id)
    saveConversations(agentName, [conv], conv.id, scope)
  }, [agentName, scope, scopeReady])

  const renameConversation = useCallback((id, name) => {
    if (!scopeReady) return
    setConversations(prev => prev.map(c =>
      c.id === id ? { ...c, name, updatedAt: Date.now() } : c
    ))
  }, [scopeReady])

  const toggleSavedConversation = useCallback((id) => {
    if (!scopeReady) return
    setConversations(prev => prev.map(c => c.id === id
      ? { ...c, savedAt: c.savedAt ? null : Date.now(), updatedAt: Date.now() }
      : c
    ))
  }, [scopeReady])

  const addMessage = useCallback((msg) => {
    if (!scopeReady) return
    setConversations(prev => prev.map(c => {
      if (c.id !== activeId) return c
      const updated = {
        ...c,
        messages: [...c.messages, msg],
        updatedAt: Date.now(),
      }
      // Auto-name from first user message
      if (c.messages.length === 0 && msg.sender === 'user') {
        const text = msg.content || ''
        updated.name = text.slice(0, 40) + (text.length > 40 ? '...' : '')
      }
      return updated
    }))
  }, [activeId, scopeReady])

  // Add a message to a specific conversation by ID, regardless of which is active.
  // Used by SSE handlers to pin responses to the conversation that initiated the request.
  const addMessageToConversation = useCallback((conversationId, msg) => {
    if (!scopeReady) return
    setConversations(prev => prev.map(c => {
      if (c.id !== conversationId) return c
      const updated = {
        ...c,
        messages: [...c.messages, msg],
        updatedAt: Date.now(),
      }
      if (c.messages.length === 0 && msg.sender === 'user') {
        const text = msg.content || ''
        updated.name = text.slice(0, 40) + (text.length > 40 ? '...' : '')
      }
      return updated
    }))
  }, [scopeReady])

  const clearMessages = useCallback(() => {
    if (!scopeReady) return
    setConversations(prev => {
      const updated = prev.map(c =>
        c.id === activeId ? { ...c, messages: [], updatedAt: Date.now() } : c
      )
      // Save immediately so a page refresh doesn't restore the old messages
      saveConversations(agentName, updated, activeId, scope)
      return updated
    })
  }, [activeId, agentName, scope, scopeReady])

  const getMessages = useCallback(() => {
    return activeConversation?.messages || []
  }, [activeConversation])

  return {
    conversations: scopeReady ? conversations : [],
    activeConversation,
    activeId,
    scopeReady,
    addConversation,
    switchConversation,
    deleteConversation,
    deleteAllConversations,
    renameConversation,
    toggleSavedConversation,
    addMessage,
    addMessageToConversation,
    clearMessages,
    getMessages,
  }
}
