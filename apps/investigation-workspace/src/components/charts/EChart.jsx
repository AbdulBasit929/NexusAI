import { useEffect, useRef } from 'react'
import * as echarts from 'echarts/core'
import { BarChart, LineChart, SankeyChart, ScatterChart } from 'echarts/charts'
import { AriaComponent, DataZoomComponent, GridComponent, LegendComponent, MarkLineComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import { SVGRenderer } from 'echarts/renderers'
import { chartTheme } from './chartTheme.js'

echarts.use([BarChart, LineChart, SankeyChart, ScatterChart, GridComponent, TooltipComponent, AriaComponent, LegendComponent, DataZoomComponent, MarkLineComponent, TitleComponent, SVGRenderer])

// Thin wrapper: the option builder receives the live theme tokens, so a theme switch
// re-renders with the right colours. SVG output stays crisp when printed and zooms without blur.
// Pointer clicks are a convenience only; every chart is paired with a table whose rows are real
// links or buttons, which is the keyboard and screen-reader path.
// With `fill` the chart takes whatever height its card gives it (the card is a flex column), so two cards in a row
// can share one height; `height` is then only the smallest it will shrink to.
//
// The chart instance lives as long as the component. A new `buildOption` (for example a different selected day) redraws it
// in place; when `zoomKey` is unchanged since the last draw, the visible window of a time axis the user zoomed or moved is
// carried over, so selecting something does not throw the zoom away. A different `zoomKey` starts from the option's own window.
export default function EChart({ buildOption, height = 240, label, onSelect, onZoom, zoomKey = null, fill = false }) {
  const host = useRef(null)
  const chart = useRef(null)
  const build = useRef(buildOption)
  build.current = buildOption
  const select = useRef(onSelect)
  select.current = onSelect
  const zoomed = useRef(onZoom)
  zoomed.current = onZoom
  const lastKey = useRef(undefined)
  const key = useRef(zoomKey)
  key.current = zoomKey

  function apply() {
    const instance = chart.current
    if (!instance) return
    const next = build.current(chartTheme())
    const current = instance.getOption?.()?.dataZoom?.[0]
    if (key.current !== null && lastKey.current === key.current && Array.isArray(next.dataZoom) && current && Number.isFinite(current.startValue) && Number.isFinite(current.endValue)) {
      next.dataZoom = next.dataZoom.map(zoom => ({ ...zoom, startValue: current.startValue, endValue: current.endValue }))
    }
    lastKey.current = key.current
    instance.setOption(next, true)
  }

  useEffect(() => {
    const node = host.current
    const instance = echarts.init(node, null, { renderer: 'svg' })
    chart.current = instance
    instance.on('click', params => select.current?.(params))
    // The visible window of a zoomable time axis, so a summary beside the chart can follow it.
    instance.on('datazoom', () => {
      const zoom = instance.getOption()?.dataZoom?.[0]
      if (zoom && Number.isFinite(zoom.startValue) && Number.isFinite(zoom.endValue)) zoomed.current?.({ from: zoom.startValue, to: zoom.endValue })
    })
    const observer = new ResizeObserver(() => instance.resize())
    observer.observe(node)
    const themeWatch = new MutationObserver(() => apply())
    themeWatch.observe(globalThis.document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
    const scheme = globalThis.matchMedia?.('(prefers-color-scheme: dark)')
    const changed = () => apply()
    scheme?.addEventListener?.('change', changed)
    return () => {
      observer.disconnect()
      themeWatch.disconnect()
      scheme?.removeEventListener?.('change', changed)
      instance.dispose()
      chart.current = null
    }
  }, [])

  useEffect(() => { apply() }, [buildOption, zoomKey]) // eslint-disable-line react-hooks/exhaustive-deps

  return <div ref={host} className={`echart${fill ? ' echart--fill' : ''}`} style={fill ? { minBlockSize: height } : { blockSize: height }} role="img" aria-label={label} />
}
