import { useEffect, useRef, useState } from 'react'
import { Maximize2, Pause, Play, RotateCcw, RotateCw, Volume2, VolumeX } from 'lucide-react'
import { formatClock, peaksFrom } from '../../lib/viewerTools.js'

const RATES = [0.75, 1, 1.25, 1.5, 2]
const MAX_DECODE_BYTES = 40 * 1024 * 1024
const BARS = 200

// One player for audio and video, with our own controls so they look and behave the same in every browser: play, skip 10 s,
// a seek bar carrying the moments the service derived (markers) and, for audio, the waveform; speed and volume. Keyboard:
// Space plays, the arrow keys skip 5 s, M mutes, F goes full screen for video. The waveform is read from the file itself when
// it is small enough to decode in the browser, and is simply absent otherwise, never estimated.
export function MediaPlayer({ kind, src, label, markers = [], initialTime = null, mediaRef, durationHint = 0, onTime }) {
  const local = useRef(null)
  const ref = mediaRef || local
  const frame = useRef(null)
  const [playing, setPlaying] = useState(false)
  const [current, setCurrent] = useState(0)
  const [duration, setDuration] = useState(durationHint || 0)
  const [rate, setRate] = useState(1)
  const [muted, setMuted] = useState(false)
  const [peaks, setPeaks] = useState(null)
  const isVideo = kind === 'video'
  const Media = isVideo ? 'video' : 'audio'

  useEffect(() => {
    const node = ref.current
    if (!node || initialTime === null || initialTime === undefined) return undefined
    const seek = () => { node.currentTime = Number(initialTime); setCurrent(Number(initialTime)) }
    node.addEventListener('loadedmetadata', seek, { once: true })
    return () => node.removeEventListener('loadedmetadata', seek)
  }, [ref, src, initialTime])

  useEffect(() => {
    if (isVideo || !src) return undefined
    let cancelled = false
    ;(async () => {
      try {
        const blob = await (await globalThis.fetch(src)).blob()
        const Context = globalThis.AudioContext || globalThis.webkitAudioContext
        if (cancelled || blob.size > MAX_DECODE_BYTES || !Context) return
        const buffer = await blob.arrayBuffer()
        const context = new Context()
        const decoded = await context.decodeAudioData(buffer)
        if (!cancelled) setPeaks(peaksFrom(decoded.getChannelData(0), BARS))
        context.close?.()
      } catch { /* no waveform: the seek bar alone is honest */ }
    })()
    return () => { cancelled = true }
  }, [isVideo, src])

  function jump(seconds) {
    const node = ref.current
    if (node) node.currentTime = Math.min(duration || Infinity, Math.max(0, node.currentTime + seconds))
  }
  function toggle() {
    const node = ref.current
    if (!node) return
    if (node.paused) node.play().catch(() => {})
    else node.pause()
  }
  function changeRate(value) {
    setRate(value)
    if (ref.current) ref.current.playbackRate = value
  }
  function fullscreen() {
    frame.current?.requestFullscreen?.().catch(() => {})
  }
  function onKey(event) {
    if (['INPUT', 'SELECT', 'TEXTAREA', 'BUTTON'].includes(event.target.tagName)) return
    if (event.key === ' ') { event.preventDefault(); toggle() }
    else if (event.key === 'ArrowLeft') { event.preventDefault(); jump(-5) }
    else if (event.key === 'ArrowRight') { event.preventDefault(); jump(5) }
    else if (event.key.toLowerCase() === 'm') setMuted(value => !value)
    else if (event.key.toLowerCase() === 'f' && isVideo) fullscreen()
  }

  const progress = duration ? Math.min(1, current / duration) : 0
  return (
    <div ref={frame} className={`mp mp--${kind}`} role="group" aria-label={`${label}. Space plays or pauses, arrow keys skip five seconds.`} tabIndex={0} onKeyDown={onKey}>
      <Media
        ref={ref}
        className="evidence-media mp__media"
        src={src}
        muted={muted}
        preload="metadata"
        playsInline
        onClick={isVideo ? toggle : undefined}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onLoadedMetadata={event => setDuration(event.currentTarget.duration || durationHint || 0)}
        onTimeUpdate={event => { setCurrent(event.currentTarget.currentTime); onTime?.(event.currentTarget.currentTime) }}
      />
      <div className="mp__bar">
        <div className="mp__track">
          {!isVideo && peaks ? (
            <svg className="mp__wave" viewBox={`0 0 ${BARS} 40`} preserveAspectRatio="none" aria-hidden="true">
              {peaks.map((peak, index) => <rect key={index} x={index} y={20 - Math.max(1, peak * 19)} width="0.72" height={Math.max(2, peak * 38)} className={index / BARS <= progress ? 'is-played' : undefined} />)}
            </svg>
          ) : <span className="mp__rail" aria-hidden="true"><i style={{ inlineSize: `${progress * 100}%` }} /></span>}
          <input type="range" className="mp__seek" min="0" max={duration || 0} step="0.1" value={Math.min(current, duration || 0)} aria-label="Seek" aria-valuetext={`${formatClock(current)} of ${formatClock(duration)}`} onChange={event => { if (ref.current) ref.current.currentTime = Number(event.target.value) }} />
          {duration > 0 && markers.length ? (
            <div className="event-scrubber" aria-label="Derived events on the timeline">
              {markers.map(marker => <button key={marker.id} type="button" className={marker.cited ? 'arrival-highlight is-cited' : undefined} style={{ insetInlineStart: `${Math.min(100, Math.max(0, (marker.time / duration) * 100))}%` }} onClick={() => { if (ref.current) ref.current.currentTime = marker.time }} aria-label={`${marker.label} at ${marker.time} seconds`} title={`${marker.label} · ${formatClock(marker.time)}`} />)}
            </div>
          ) : null}
        </div>
        <div className="mp__controls">
          <button type="button" className="mp__play" onClick={toggle} aria-label={playing ? 'Pause' : 'Play'}>{playing ? <Pause aria-hidden="true" /> : <Play aria-hidden="true" />}</button>
          <button type="button" className="mp__icon" onClick={() => jump(-10)} aria-label="Back 10 seconds"><RotateCcw aria-hidden="true" /></button>
          <button type="button" className="mp__icon" onClick={() => jump(10)} aria-label="Forward 10 seconds"><RotateCw aria-hidden="true" /></button>
          <span className="mp__time" aria-hidden="true">{formatClock(current)} <i>/</i> {formatClock(duration)}</span>
          <label className="mp__rate"><span className="visually-hidden">Playback speed</span>
            <select value={rate} onChange={event => changeRate(Number(event.target.value))}>{RATES.map(value => <option key={value} value={value}>{value}×</option>)}</select>
          </label>
          <button type="button" className="mp__icon" onClick={() => setMuted(value => !value)} aria-pressed={muted} aria-label={muted ? 'Unmute' : 'Mute'}>{muted ? <VolumeX aria-hidden="true" /> : <Volume2 aria-hidden="true" />}</button>
          {isVideo ? <button type="button" className="mp__icon" onClick={fullscreen} aria-label="Full screen"><Maximize2 aria-hidden="true" /></button> : null}
        </div>
      </div>
    </div>
  )
}
