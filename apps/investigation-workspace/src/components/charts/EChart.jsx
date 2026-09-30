import { useEffect, useRef } from 'react'
import * as echarts from 'echarts/core'
import { BarChart, SankeyChart } from 'echarts/charts'
import { AriaComponent, DataZoomComponent, GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { SVGRenderer } from 'echarts/renderers'
import { chartTheme } from './chartTheme.js'

echarts.use([BarChart, SankeyChart, GridComponent, TooltipComponent, AriaComponent, LegendComponent, DataZoomComponent, SVGRenderer])

// Thin wrapper: the option builder receives the live theme tokens, so a theme switch
// re-renders with the right colours. SVG output stays crisp when printed and zooms without blur.
// Pointer clicks are a convenience only; every chart is paired with a table whose rows are real
// links or buttons, which is the keyboard and screen-reader path.
export default function EChart({ buildOption, height = 240, label, onSelect }) {
  const host = useRef(null)
  const chart = useRef(null)
  const select = useRef(onSelect)
  select.current = onSelect

  useEffect(() => {
    const node = host.current
    const instance = echarts.init(node, null, { renderer: 'svg' })
    chart.current = instance
    const apply = () => instance.setOption(buildOption(chartTheme()), true)
    apply()
    instance.on('click', params => select.current?.(params))
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

  return <div ref={host} className="echart" style={{ blockSize: height }} role="img" aria-label={label} />
}
