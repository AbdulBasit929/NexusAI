import { useEffect, useRef } from 'react'
import * as echarts from 'echarts/core'
import { BarChart, SankeyChart, ScatterChart } from 'echarts/charts'
import { AriaComponent, DataZoomComponent, GridComponent, LegendComponent, MarkLineComponent, TitleComponent, TooltipComponent } from 'echarts/components'
import { SVGRenderer } from 'echarts/renderers'
import { chartTheme } from './chartTheme.js'

echarts.use([BarChart, SankeyChart, ScatterChart, GridComponent, TooltipComponent, AriaComponent, LegendComponent, DataZoomComponent, MarkLineComponent, TitleComponent, SVGRenderer])

// Thin wrapper: the option builder receives the live theme tokens, so a theme switch
// re-renders with the right colours. SVG output stays crisp when printed and zooms without blur.
// Pointer clicks are a convenience only; every chart is paired with a table whose rows are real
// links or buttons, which is the keyboard and screen-reader path.
// With `fill` the chart takes whatever height its card gives it (the card is a flex column), so two cards in a row
// can share one height; `height` is then only the smallest it will shrink to.
export default function EChart({ buildOption, height = 240, label, onSelect, onZoom, fill = false }) {
  const host = useRef(null)
  const chart = useRef(null)
  const select = useRef(onSelect)
  select.current = onSelect
  const zoomed = useRef(onZoom)
  zoomed.current = onZoom

  useEffect(() => {
    const node = host.current
    const instance = echarts.init(node, null, { renderer: 'svg' })
    chart.current = instance
    const apply = () => instance.setOption(buildOption(chartTheme()), true)
    apply()
    instance.on('click', params => select.current?.(params))
    // The visible window of a zoomable time axis, so a summary beside the chart can follow it.
    instance.on('datazoom', () => {
      const zoom = instance.getOption()?.dataZoom?.[0]
      if (zoom && Number.isFinite(zoom.startValue) && Number.isFinite(zoom.endValue)) zoomed.current?.({ from: zoom.startValue, to: zoom.endValue })
    })
    const observer = new ResizeObserver(() => instance.resize())
    observer.observe(node)
    const themeWatch = new MutationObserver(apply)
    themeWatch.observe(globalThis.document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
    const scheme = globalThis.matchMedia?.('(prefers-color-scheme: dark)')
    scheme?.addEventListener?.('change', apply)
    return () => {
      observer.disconnect()
      themeWatch.disconnect()
      scheme?.removeEventListener?.('change', apply)
      instance.dispose()
      chart.current = null
    }
  }, [buildOption])

  return <div ref={host} className={`echart${fill ? ' echart--fill' : ''}`} style={fill ? { minBlockSize: height } : { blockSize: height }} role="img" aria-label={label} />
}
