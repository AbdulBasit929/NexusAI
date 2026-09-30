import { describe, expect, it } from 'vitest'
import { chartableResult, resultChartHeight, resultChartLabel, resultChartOption } from './resultChart.js'

const presentation = (columns, rows) => ({ result: { columns: columns.map(key => ({ key, label: key === 'm1' ? 'Calls' : key })) }, original: { enterprise: { data_grid: { rows } } } })
const theme = { text: '#111', muted: '#555', line: '#ccc', card: '#fff', data: ['#0a7', '#07a'] }

describe('chartableResult', () => {
  it('ranks a labelled measure, highest first', () => {
    const chart = chartableResult(presentation(['contact', 'm1'], [{ contact: 'A', m1: 5 }, { contact: 'B', m1: 9 }, { contact: 'C', m1: 7 }]))
    expect(chart.kind).toBe('ranked')
    expect(chart.points.map(point => point.label)).toEqual(['B', 'C', 'A'])
    expect(chart).toMatchObject({ valueName: 'Calls', labelName: 'contact', shown: 3, total: 3 })
  })

  it('draws a series over time, oldest first, when the labels are days', () => {
    const chart = chartableResult(presentation(['day', 'event_count'], [{ day: '2026-03-02T00:00:00Z', event_count: 4 }, { day: '2026-03-01T00:00:00Z', event_count: 2 }]))
    expect(chart.kind).toBe('time')
    expect(chart.points.map(point => point.label)).toEqual(['2026-03-01', '2026-03-02'])
  })

  it('caps a long ranking and says how many rows there are', () => {
    const rows = Array.from({ length: 20 }, (_, index) => ({ contact: `c${index}`, m1: index + 1 }))
    const chart = chartableResult(presentation(['contact', 'm1'], rows))
    expect(chart.points).toHaveLength(12)
    expect(chart).toMatchObject({ shown: 12, total: 20 })
    expect(chart.points[0].value).toBe(20)
  })

  it('draws nothing it cannot be sure of: an identifier that looks numeric, one row, repeated labels, negatives', () => {
    expect(chartableResult(presentation(['contact', 'phone_number'], [{ contact: 'A', phone_number: '03001234567' }, { contact: 'B', phone_number: '03007654321' }]))).toBeNull()
    expect(chartableResult(presentation(['contact', 'm1'], [{ contact: 'A', m1: 5 }]))).toBeNull()
    expect(chartableResult(presentation(['contact', 'm1'], [{ contact: 'A', m1: 5 }, { contact: 'A', m1: 6 }]))).toBeNull()
    expect(chartableResult(presentation(['contact', 'm1'], [{ contact: 'A', m1: -5 }, { contact: 'B', m1: 6 }]))).toBeNull()
    expect(chartableResult(presentation(['contact', 'm1'], [{ contact: 'A', m1: null }, { contact: 'B', m1: 6 }]))).toBeNull()
    expect(chartableResult({ result: { columns: [] }, original: null })).toBeNull()
  })

  it('describes the chart in words and builds options from the theme', () => {
    const chart = chartableResult(presentation(['contact', 'm1'], [{ contact: 'A', m1: 5 }, { contact: 'B', m1: 9 }]))
    expect(resultChartLabel(chart)).toBe('Calls by contact, highest first. B: 9; A: 5.')
    const option = resultChartOption(chart, theme)
    expect(option.yAxis.data).toEqual(['B', 'A'])
    expect(option.series[0].data).toEqual([9, 5])
    expect(option.series[0].itemStyle.color).toBe('#0a7')
    expect(resultChartHeight(chart)).toBe(120)
    const time = chartableResult(presentation(['day', 'm1'], [{ day: '2026-03-01', m1: 1 }, { day: '2026-03-02', m1: 2 }]))
    expect(resultChartOption(time, theme).xAxis.data).toEqual(['2026-03-01', '2026-03-02'])
    expect(resultChartLabel(time)).toBe('Calls by day, 2026-03-01 to 2026-03-02.')
  })
})
