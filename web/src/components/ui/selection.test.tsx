import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'

import { ChipGroup } from '@/components/ui/chip-group'
import { SegmentedControl } from '@/components/ui/segmented'

afterEach(cleanup)

const options = [
  { value: 'a', label: 'Mon' },
  { value: 'b', label: 'Tue' },
  { value: 'c', label: 'Wed' },
] as const

// checks are the labels of the options that show a check.
function checks(): string[] {
  return screen
    .getAllByRole('radio')
    .map((radio) => radio.closest('label'))
    .filter((label) => label?.querySelector('svg') != null)
    .map((label) => label?.textContent ?? '')
}

// A chip and a segment are chosen alike: the brand's soft green and a
// check, on the chosen one alone; the others have a control's edge.
test('a chip chosen has a check, the only one, on the soft green', () => {
  render(<ChipGroup legend="Day" options={options} value="b" onChange={() => undefined} />)
  expect(checks()).toEqual(['Tue'])
  const chip = screen.getByRole('radio', { name: 'Tue' }).closest('label')
  expect(chip?.className).toContain('has-checked:bg-primary-soft')
  expect(chip?.className).toContain('border-control-border')
})

test('a segment chosen has a check, the only one, on the soft green', () => {
  render(<SegmentedControl legend="Day" options={options} value="c" onChange={() => undefined} />)
  expect(checks()).toEqual(['Wed'])
  const segment = screen.getByRole('radio', { name: 'Wed' }).closest('label')
  expect(segment?.className).toContain('has-checked:bg-primary-soft')
  expect(segment?.parentElement?.className).toContain('border-control-border')
})
