import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import type { Meal } from '@/api/types'
import { useRepeatYesterday } from '@/components/day/repeat-yesterday'
import { day, fakeServer, signedIn } from '@/test/app'

beforeEach(() => {
  // 9:30 on the 23rd in Los Angeles, a breakfast's hour.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-10-23T16:30:00Z'))
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

const porridge: Meal = {
  id: '0193c3a5-0000-7000-8000-0000000000b1',
  eaten_at: '2026-10-22T15:00:00Z',
  description: 'Porridge',
  client: 'web',
  total: { kcal: 207, protein: 7, carbs: 38, fat: 3 },
  items: [],
}

// Asked for twice at once (a double click, two keys, a key held down),
// yesterday's meal is logged once.
test("yesterday's meal asked for twice at once is logged once", async () => {
  let posted = 0
  fakeServer({
    ...signedIn(),
    'GET /api/v1/days/2026-10-22': () =>
      Response.json(day({ date: '2026-10-22', meals: [porridge] })),
    'POST /api/v1/meals': () => {
      posted++
      const { meals: _m, metrics: _x, ...summary } = day()
      return Response.json({ meal: porridge, day: summary, warnings: [] }, { status: 201 })
    },
  })
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  const { result } = renderHook(() => useRepeatYesterday(true), { wrapper })
  await waitFor(() => {
    expect(result.current.label(Date.now())).toBe("Repeat yesterday's breakfast")
  })

  await act(async () => {
    await Promise.all([result.current.run(), result.current.run()])
  })
  await waitFor(() => {
    expect(posted).toBe(1)
  })
  await new Promise((resolve) => setTimeout(resolve, 50))
  expect(posted).toBe(1)

  // Once done, it may be asked for again.
  await act(async () => {
    await result.current.run()
  })
  await waitFor(() => {
    expect(posted).toBe(2)
  })
})
