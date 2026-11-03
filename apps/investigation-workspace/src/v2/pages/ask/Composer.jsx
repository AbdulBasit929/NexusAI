import { useEffect, useLayoutEffect, useRef } from 'react'
import { ArrowUp, Check, ChevronDown, Layers, Square } from 'lucide-react'
import { evidenceFamilies, familyDefinition } from '../../../components/ScopeChips.jsx'
import { Menu } from '../../shell/Menu.jsx'

// The question box. Enter sends, Shift+Enter starts a new line (and never sends while an input method is composing); the box grows to six lines;
// the scope of the next question is one button away and stays visible; Stop appears while a question runs. Alt+A focuses it from anywhere.
export function Composer({ value, onChange, onSubmit, busy, onStop, scope, onScope, inputRef, large = false, placeholder = 'Ask about this case…', unavailable = '' }) {
  const own = useRef(null)
  const area = inputRef || own
  const family = familyDefinition(scope)

  useLayoutEffect(() => {
    const node = area.current
    if (!node) return
    node.style.blockSize = 'auto'
    const limit = large ? 200 : 160
    node.style.blockSize = `${Math.min(node.scrollHeight + 2, limit)}px`
  }, [value, area, large])

  useEffect(() => {
    function key(event) {
      if (event.altKey && event.key.toLowerCase() === 'a') { event.preventDefault(); area.current?.focus() }
    }
    globalThis.document.addEventListener('keydown', key)
    return () => globalThis.document.removeEventListener('keydown', key)
  }, [area])

  function submit() {
    const text = value.trim()
    if (!text || busy || unavailable) return
    onSubmit(text)
  }
  function keydown(event) {
    if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) { event.preventDefault(); submit() }
  }

  return (
    <form className={`nx-composer${large ? ' nx-composer--large' : ''}`} onSubmit={event => { event.preventDefault(); submit() }} aria-label="Ask a question about this case">
      <textarea
        ref={area}
        className="nx-composer__input"
        rows={large ? 2 : 1}
        value={value}
        placeholder={unavailable ? 'Questions cannot be asked in this scope' : placeholder}
        aria-label="Your question"
        dir="auto"
        disabled={Boolean(unavailable)}
        onChange={event => onChange(event.target.value)}
        onKeyDown={keydown}
      />
      <div className="nx-composer__bar">
        <Menu label="Evidence scope for the next question" align="start" up buttonClassName="nx-chip" trigger={<><Layers aria-hidden="true" />{scope === 'all' ? 'All evidence' : family.label}<ChevronDown aria-hidden="true" /></>}>
          {close => (
            <>
              <div className="nx-menu__label">Search in</div>
              {evidenceFamilies.map(item => (
                <button key={item.id} type="button" role="menuitemradio" aria-checked={scope === item.id} className="nx-menu__item" onClick={() => { onScope(item.id); close() }}>
                  {item.id === 'all' ? 'All evidence' : item.label}{scope === item.id ? <Check className="nx-menu__check" aria-hidden="true" /> : null}
                </button>
              ))}
            </>
          )}
        </Menu>
        <span className="nx-composer__hint"><kbd className="nx-kbd">Enter</kbd> to ask <kbd className="nx-kbd">Shift</kbd>+<kbd className="nx-kbd">Enter</kbd> new line</span>
        {busy ? (
          <button type="button" className="nx-btn" onClick={onStop}><Square aria-hidden="true" />Stop</button>
        ) : (
          <button type="submit" className="nx-btn nx-btn--primary" disabled={!value.trim() || Boolean(unavailable)} aria-label="Ask"><ArrowUp aria-hidden="true" />Ask</button>
        )}
      </div>
      {unavailable ? <p className="nx-composer__note" role="note">{unavailable}</p> : null}
    </form>
  )
}
