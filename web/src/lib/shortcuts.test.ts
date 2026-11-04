import { cleanup, fireEvent, renderHook } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { useShortcuts } from '@/lib/shortcuts'

// tick lets a sequence begun end, as the browser's next task does.
const tick = () => new Promise((r) => setTimeout(r))

afterEach(async () => {
  cleanup()
  await tick()
})

// press sends a key to the page, and says whether a shortcut took it.
function press(key: string): boolean {
  return !fireEvent.keyDown(document.body, { key })
}

// G, then a letter, is the sequence's, whichever page's shortcuts were
// heard first: a page's own A does not run.
test("the second letter of a sequence is no other shortcut's, in any order", async () => {
  const a = vi.fn<() => void>()
  const gd = vi.fn<() => void>()
  // The page's first, then the shell's, as when both come in one commit.
  renderHook(() => {
    useShortcuts({ a })
  })
  renderHook(() => {
    useShortcuts({ 'g d': gd })
  })
  expect(press('g')).toBe(true)
  press('a')
  expect(a).not.toHaveBeenCalled()
  await tick()
  expect(press('g')).toBe(true)
  press('d')
  expect(gd).toHaveBeenCalledOnce()
})

// A key that is no letter is its own after G: the focus moves on Tab.
test('after G, Tab and Enter are left to the page', async () => {
  renderHook(() => {
    useShortcuts({ 'g d': vi.fn<() => void>() })
  })
  press('g')
  expect(press('Tab')).toBe(false)
  await tick()
  press('g')
  expect(press('Enter')).toBe(false)
})

// Typing on a page (typed) takes a letter or a digit no page takes as a
// shortcut, nor as a sequence's second key, with no key held but Shift.
test('typed takes a letter or a digit that is no shortcut, outside a field', async () => {
  const typed = vi.fn<(key: string) => void>()
  const n = vi.fn<() => void>()
  const a = vi.fn<() => void>()
  renderHook(() => {
    useShortcuts({ n, 'g d': vi.fn<() => void>() }, typed)
  })
  // A page's own key, mounted apart, is no text either, in either case.
  renderHook(() => {
    useShortcuts({ a })
  })

  expect(press('o')).toBe(true)
  expect(press('Č')).toBe(true)
  expect(press('7')).toBe(true)
  expect(typed.mock.calls).toEqual([['o'], ['Č'], ['7']])
  typed.mockClear()

  press('n')
  press('A')
  press('g')
  press('d')
  press(' ')
  press('Enter')
  fireEvent.keyDown(document.body, { key: 'o', ctrlKey: true })
  fireEvent.keyDown(document.body, { key: 'o', metaKey: true })
  fireEvent.keyDown(document.body, { key: 'o', altKey: true })
  fireEvent.keyDown(document.body, { key: 'o', isComposing: true })
  fireEvent.keyDown(document.body, { key: 'Process' })
  const field = document.createElement('input')
  document.body.append(field)
  fireEvent.keyDown(field, { key: 'o' })
  field.remove()
  await tick()
  expect(n).toHaveBeenCalledOnce()
  expect(typed).not.toHaveBeenCalled()
})

// A key held down repeats its keydown: its action runs once.
test('a key held down runs its action once', () => {
  const n = vi.fn<() => void>()
  renderHook(() => {
    useShortcuts({ n })
  })
  press('n')
  fireEvent.keyDown(document.body, { key: 'n', repeat: true })
  fireEvent.keyDown(document.body, { key: 'n', repeat: true })
  expect(n).toHaveBeenCalledOnce()
})

// A page's shortcut is taken only while the page is there: once the plan
// is left, its A is text on the day.
test("a page's keys are typed again once it is gone", () => {
  const typed = vi.fn<(key: string) => void>()
  renderHook(() => {
    useShortcuts({}, typed)
  })
  const plan = renderHook(() => {
    useShortcuts({ a: vi.fn<() => void>() })
  })
  press('a')
  expect(typed).not.toHaveBeenCalled()
  plan.unmount()
  expect(press('a')).toBe(true)
  expect(typed.mock.calls).toEqual([['a']])
})
