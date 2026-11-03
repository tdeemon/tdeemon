import { expect, test } from 'vitest'

import { themeTokens } from '@/test/charts'
import { contrast, distance, resolved } from '@/test/colour'

// The colours that mean something, and the macros', which only name a
// macro: one of the first is never one of the second, nor near it, or a
// day over its target would look like fat, and a warning like carbs.
const meaning = ['--primary', '--warning', '--destructive', '--over'] as const
const macros = ['--protein', '--carbs', '--fat'] as const

// apart is how far two colours must be, in OKLab (0 to 1 from black to
// white): a tenth, five times what an eye just tells apart.
const apart = 0.1

const themes = Object.entries(themeTokens())

test.each(themes)(
  'in the %s theme, no colour that means something is near a macro’s',
  (_, theme) => {
    const near = meaning.flatMap((m) =>
      macros
        .map((macro) => [m, macro, distance(resolved(theme, m), resolved(theme, macro))] as const)
        .filter(([, , d]) => d < apart),
    )
    expect(near).toEqual([])
  },
)

test.each(themes)(
  'in the %s theme, over is none of the other colours that mean something',
  (_, theme) => {
    const near = meaning
      .filter((m) => m !== '--over')
      .map((m) => [m, distance(resolved(theme, '--over'), resolved(theme, m))] as const)
      .filter(([, d]) => d < apart)
    expect(near).toEqual([])
  },
)

// Over draws an arc, a bar's part and a large number: 3:1 on what is
// behind them.
test.each(themes)(
  'in the %s theme, over keeps 3:1 on the page, a card and surface-2',
  (_, theme) => {
    const over = resolved(theme, '--over')
    const low = ['--background', '--card', '--surface-2']
      .map((s) => [s, contrast(over, resolved(theme, s))] as const)
      .filter(([, c]) => c < 3)
    expect(low).toEqual([])
  },
)

// The test's own measure, on colours whose distance is known: the dark
// theme's carbs and warning were one colour before they were told apart.
test('the measure tells one colour from another, and the same from itself', () => {
  expect(distance('#f0b45c', '#f0b45c')).toBe(0)
  expect(distance('#000000', '#ffffff')).toBeCloseTo(1, 3)
  expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 3)
})

// What text is drawn on: the page, a card, surface-2 and a popover.
const grounds = ['--background', '--card', '--surface-2', '--popover'] as const

// Text keeps 4.5:1 (WCAG 1.4.3) on each ground, and on its own soft
// background where it has one.
const text = [
  ...[
    '--foreground',
    '--muted-foreground',
    '--primary',
    '--coach',
    '--warning',
    '--destructive',
  ].flatMap((fg) => grounds.map((bg) => [fg, bg] as const)),
  ['--primary-foreground', '--primary'],
  ['--primary', '--primary-soft'],
  ['--foreground', '--primary-soft'],
  ['--coach', '--coach-soft'],
  ['--warning', '--warning-soft'],
  ['--muted-foreground', '--warning-soft'],
  ['--foreground', '--highlight'],
  ['--muted-foreground', '--highlight'],
  ['--primary', '--highlight'],
] as const

// A control's edge and a mark keep 3:1 (WCAG 1.4.11) on what is behind
// them: an input's, a chip's or an outline button's edge on the page, a
// card, surface-2 (a segmented control's) and a popover; a macro's bar,
// over's arc and a chart's lines on the page, a card and surface-2 (a
// bar's track).
const graphic = [
  ...grounds.map((bg) => ['--control-border', bg] as const),
  ...['--protein', '--carbs', '--fat', '--over', '--chart-grid'].flatMap((fg) =>
    ['--background', '--card', '--surface-2'].map((bg) => [fg, bg] as const),
  ),
] as const

test.each(themes)('in the %s theme, text keeps 4.5:1 on what it is drawn on', (_, theme) => {
  const low = text
    .map(([fg, bg]) => [fg, bg, contrast(resolved(theme, fg), resolved(theme, bg))] as const)
    .filter(([, , c]) => c < 4.5)
  expect(low).toEqual([])
})

test.each(themes)(
  'in the %s theme, a control’s edge and a mark keep 3:1 on what is behind them',
  (_, theme) => {
    const low = graphic
      .map(([fg, bg]) => [fg, bg, contrast(resolved(theme, fg), resolved(theme, bg))] as const)
      .filter(([, , c]) => c < 3)
    expect(low).toEqual([])
  },
)

// border only divides: it is no control's edge, which control-border is.
test.each(themes)('in the %s theme, a control’s edge is not a divider’s', (_, theme) => {
  expect(resolved(theme, '--input')).toBe(resolved(theme, '--control-border'))
  expect(contrast(resolved(theme, '--border'), resolved(theme, '--card'))).toBeLessThan(3)
})

// The coach's violet marks only what an AI did: it is none of the
// colours that mean something else, nor a macro's.
test.each(themes)('in the %s theme, the coach’s colour is none of the others', (_, theme) => {
  const near = [...meaning, ...macros]
    .map((m) => [m, distance(resolved(theme, '--coach'), resolved(theme, m))] as const)
    .filter(([, d]) => d < apart)
  expect(near).toEqual([])
})

// A token missing, or no colour, is an error, not a measure that passes
// every bound as NaN would.
test('a token the theme does not have, or no colour, is an error', () => {
  const theme = { '--a': 'var(--b)', '--b': '#0b7a5e', '--c': 'var(--gone)', '--d': '1px' }
  expect(resolved(theme, '--a')).toBe('#0b7a5e')
  expect(() => resolved(theme, '--gone')).toThrow('no token --gone')
  expect(() => resolved(theme, '--c')).toThrow('no token --gone')
  expect(() => resolved(theme, '--d')).toThrow('no #rrggbb')
})
