import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterEach } from 'vitest'

afterEach(cleanup)

// a page that loads a chart or a map in a lazy chunk needs more than the default second when the whole suite runs in parallel
configure({ asyncUtilTimeout: 5000 })

globalThis.ResizeObserver ??= class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
