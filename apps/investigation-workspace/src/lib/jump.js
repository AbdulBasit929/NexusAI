// An in-page jump that lands well: smooth scroll (instant under reduced motion), focus moves to the target so keyboard
// and screen-reader users arrive there too, and the target flashes once so the eye finds it. Returns false when the
// target is not on the page, so the caller can let the browser handle the link normally.
export function jumpToElement(id) {
  const target = globalThis.document?.getElementById(String(id).replace(/^#/, ''))
  if (!target) return false
  const reduced = Boolean(globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches)
  target.scrollIntoView?.({ behavior: reduced ? 'auto' : 'smooth', block: 'start' })
  target.setAttribute('tabindex', '-1')
  target.focus({ preventScroll: true })
  target.classList.remove('dash-flash')
  void target.offsetWidth
  target.classList.add('dash-flash')
  globalThis.setTimeout(() => target.classList.remove('dash-flash'), 1100)
  return true
}
