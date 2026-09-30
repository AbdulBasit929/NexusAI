import { useEffect, useRef, useState } from 'react'

export function AskInput({ busy = false, initialValue = '', value, onChange, onAsk, inputRef, label = 'Ask a question about this case', placeholder = 'Ask about the selected evidence…', ariaLabel = 'Ask about case evidence', clearOnAsk = true }) {
  const [internal, setInternal] = useState(initialValue)
  const localRef = useRef(null)
  const query = value === undefined ? internal : value
  function setQuery(next) { if (value === undefined) setInternal(next); onChange?.(next) }
  useEffect(() => {
    function shortcut(event) {
      // `key` can be layout-transformed while Alt is held. `code` keeps the
      // physical shortcut stable across analyst keyboard layouts.
      if (event.altKey && (event.code === 'KeyA' || event.key.toLocaleLowerCase() === 'a')) {
        event.preventDefault(); (inputRef?.current || localRef.current)?.focus()
      }
    }
    globalThis.document.addEventListener('keydown', shortcut)
    return () => globalThis.document.removeEventListener('keydown', shortcut)
  }, [inputRef])
  useEffect(() => {
    const node = inputRef?.current || localRef.current
    if (!node) return
    node.style.height = 'auto'
    node.style.height = `${Math.min(node.scrollHeight, 176)}px`
  }, [query, inputRef])
  function submit(event) {
    event.preventDefault()
    const value = query.trim()
    if (!value || busy) return
    onAsk(value)
    if (clearOnAsk) setQuery('')
  }
  function handleKeyDown(event) {
    if (event.key !== 'Enter' || event.shiftKey || event.nativeEvent?.isComposing) return
    event.preventDefault()
    event.currentTarget.form?.requestSubmit()
  }
  return (
    <form className="ask-form" onSubmit={submit} aria-label={ariaLabel}>
      <label htmlFor="case-question">{label}</label>
      <div className="ask-form__row">
        <textarea
          ref={node => { localRef.current = node; if (inputRef) inputRef.current = node }}
          id="case-question"
          name="question"
          rows="1"
          value={query}
          onChange={event => setQuery(event.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          aria-busy={busy}
          aria-describedby="case-question-hint"
        />
        <button type="submit" className="ask-form__submit" disabled={busy || !query.trim()} aria-label={busy ? 'Checking evidence' : 'Ask'}>
          <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d="M12 19V5m0 0-6 6m6-6 6 6" /></svg>
          <span>{busy ? 'Checking' : 'Ask'}</span>
        </button>
      </div>
      <p id="case-question-hint" className="ask-form__hint">Enter to ask · Shift + Enter for a new line · Alt + A to focus</p>
    </form>
  )
}
