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
