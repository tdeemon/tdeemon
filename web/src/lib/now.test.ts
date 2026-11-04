import { act, renderHook } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { useNow } from '@/lib/now'

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('the time now is asked again each minute', () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-23T17:59:30Z'))
  const { result } = renderHook(() => useNow())
  expect(result.current).toBe(Date.parse('2026-10-23T17:59:30Z'))

  act(() => {
    vi.advanceTimersByTime(60_000)
  })
  expect(result.current).toBe(Date.parse('2026-10-23T18:00:30Z'))
})

test('a page shown again asks the time at once, one hidden does not', () => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-23T08:00:00Z'))
  const { result } = renderHook(() => useNow())
  vi.setSystemTime(new Date('2026-10-23T19:00:00Z'))

  const hidden = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
  act(() => {
    document.dispatchEvent(new Event('visibilitychange'))
  })
  expect(result.current).toBe(Date.parse('2026-10-23T08:00:00Z'))

  hidden.mockReturnValue('visible')
  act(() => {
    document.dispatchEvent(new Event('visibilitychange'))
  })
  expect(result.current).toBe(Date.parse('2026-10-23T19:00:00Z'))
})
