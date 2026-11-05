import { useLayoutEffect, useRef } from 'react'
import { MOTION_KEY } from '../shell/appearance.js'

// A real figure that counts up once, the first time it appears: well under a second, easing out, and never when the analyst asked for less motion. The final number is the text of the element
// from the start (a screen reader and a search find it; nothing is ever shown that is not the real value), the motion only changes what is painted in the first 420 ms.
// Reduced motion is read when the figure is created, from the system setting, from the page and from the viewer's own stored choice (the shell writes that to the page only after the first paint),
// so there is no frame of animation before it is honoured.
const reduced = () => {
  try { return Boolean(globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches) || globalThis.document?.documentElement?.dataset.motion === 'reduce' || globalThis.localStorage?.getItem(MOTION_KEY) === 'reduce' } catch { return false }
}

export function CountUp({ value, format = number => String(number), className, duration = 420 }) {
  const node = useRef(null)
  const played = useRef(false)
  const text = Number.isFinite(value) ? format(value) : String(value ?? '—')
  useLayoutEffect(() => {
    if (played.current || !Number.isFinite(value) || value <= 0 || reduced()) return undefined
    played.current = true
    const element = node.current
    if (!element) return undefined
    const start = globalThis.performance.now()
    let frame
    const tick = now => {
      const progress = Math.min(1, (now - start) / duration)
      element.textContent = progress < 1 ? format(Math.round(value * (1 - (1 - progress) ** 3))) : text
      if (progress < 1) frame = globalThis.requestAnimationFrame(tick)
    }
    element.textContent = format(0)
    frame = globalThis.requestAnimationFrame(tick)
    return () => { globalThis.cancelAnimationFrame(frame); element.textContent = text }
  }, [value]) // eslint-disable-line react-hooks/exhaustive-deps
  return <span ref={node} className={className}>{text}</span>
}
