import { useEffect, useRef } from 'react'
import * as echarts from 'echarts/core'
import { BarChart, LineChart } from 'echarts/charts'
import { AriaComponent, GridComponent, TooltipComponent } from 'echarts/components'
import { SVGRenderer } from 'echarts/renderers'
import { readChartTheme } from './theme.js'

echarts.use([BarChart, LineChart, GridComponent, TooltipComponent, AriaComponent, SVGRenderer])

// One thin wrapper for every chart of the new design. The caller gives a function that builds the option from the live theme tokens; the chart
// redraws when that function changes, when the theme or the density changes, and when its box is resized. SVG, so it prints and zooms sharply.
// Every chart is paired by its caller with a table of the same marks (the keyboard and screen-reader route); the click is a convenience only.
export default function Chart({ build, height = 260, label, onSelect }) {
  const host = useRef(null)
  const chart = useRef(null)
  const builder = useRef(build)
  builder.current = build
  const select = useRef(onSelect)
  select.current = onSelect

  function apply() {
    if (!chart.current) return
    chart.current.setOption(builder.current(readChartTheme()), true)
  }

  useEffect(() => {
    const node = host.current
    const instance = echarts.init(node, null, { renderer: 'svg' })
    chart.current = instance
    instance.on('click', params => select.current?.(params))
    const resize = new ResizeObserver(() => instance.resize())
    resize.observe(node)
    const watch = new MutationObserver(() => apply())
    watch.observe(globalThis.document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'data-motion', 'data-chrome'] })
    const scheme = globalThis.matchMedia?.('(prefers-color-scheme: dark)')
    scheme?.addEventListener?.('change', apply)
    return () => {
      resize.disconnect()
      watch.disconnect()
      scheme?.removeEventListener?.('change', apply)
      instance.dispose()
      chart.current = null
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { apply() }, [build]) // eslint-disable-line react-hooks/exhaustive-deps

  return <div ref={host} className="nx-chart" style={{ blockSize: height }} role="img" aria-label={label} />
}
