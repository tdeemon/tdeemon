import { afterEach, expect, test } from 'vitest'

import { chartColors, palette, tokens } from '@/components/charts/palette'
import { themeStyle, themeTokens } from '@/test/charts'
import { blindDistance, chroma, contrast, distance, lightness, resolved } from '@/test/colour'

afterEach(() => {
  document.documentElement.classList.remove('dark')
  for (const style of document.head.querySelectorAll('style')) {
    style.remove()
  }
})

test("the dark theme's tokens are the palette's", () => {
  const { dark } = themeTokens()
  expect(
    Object.fromEntries(Object.entries(tokens).map(([key, token]) => [key, resolved(dark, token)])),
  ).toEqual(palette)
})

test("the charts' colours are the page's tokens, light or dark as the page is", () => {
  const { light, dark } = themeTokens()
  document.head.append(themeStyle())

  expect(chartColors().primary).toBe(resolved(light, '--primary'))
  expect(chartColors().grid).toBe(resolved(light, '--border'))
  document.documentElement.classList.add('dark')
  expect(chartColors()).toEqual(palette)
  expect(chartColors().blue).toBe(resolved(dark, '--protein'))
})

test("a page without the tokens draws in the palette's", () => {
  expect(chartColors()).toEqual(palette)
})

// The macros' colours are a chart's series too (a day's split, a bar of
// each): each inside the theme's band of lightness and colourful enough
// not to read as grey, each pair told apart by a colour-blind eye (a
// distance of 0.08 in OKLab under protanopia and deuteranopia) and by
// any other (0.15), and each 3:1 on what it is drawn on, as a validated
// categorical palette is.
const band: Record<string, readonly [number, number]> = {
  light: [0.43, 0.77],
  dark: [0.48, 0.67],
}
const macros = ['--protein', '--carbs', '--fat'] as const

test.each(Object.entries(themeTokens()))(
  "in the %s theme, the macros' colours are a validated palette",
  (name, theme) => {
    const [lo, hi] = band[name] ?? [0, 1]
    const colours = macros.map((m) => resolved(theme, m))
    const pairs = colours.flatMap((one, i) => colours.slice(i + 1).map((other) => [one, other]))
    const fails = [
      ...colours.filter((c) => lightness(c) < lo || lightness(c) > hi).map((c) => `${c} lightness`),
      ...colours.filter((c) => chroma(c) < 0.1).map((c) => `${c} chroma`),
      ...pairs
        .filter(([one = '', other = '']) => blindDistance(one, other) < 0.08)
        .map((p) => `${p.join('/')} colour-blind`),
      ...pairs
        .filter(([one = '', other = '']) => distance(one, other) < 0.15)
        .map((p) => `${p.join('/')} near`),
      ...colours.flatMap((c) =>
        ['--background', '--card', '--surface-2']
          .filter((bg) => contrast(c, resolved(theme, bg)) < 3)
          .map((bg) => `${c} on ${bg}`),
      ),
    ]
    expect(fails).toEqual([])
  },
)

// The measure of a colour-blind eye: red and green, far apart to any
// other, are near to it.
test('a colour-blind eye tells red from green less than another eye', () => {
  expect(blindDistance('#d62728', '#2ca02c')).toBeLessThan(distance('#d62728', '#2ca02c') / 2)
  expect(blindDistance('#3559d6', '#3559d6')).toBe(0)
})
