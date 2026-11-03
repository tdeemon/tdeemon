import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

// The app's stylesheet, whose variants say when the sections are a rail
// or a sidebar: jsdom applies no media query, so this reads them.
const css = readFileSync(join(import.meta.dirname, 'index.css'), 'utf8')

// variant is the media query of a custom variant of index.css.
function variant(name: string): string {
  const found = new RegExp(`@custom-variant ${name} \\(@media (.+)\\);`).exec(css)
  if (found?.[1] === undefined) {
    throw new Error(`no variant ${name}`)
  }
  return found[1]
}

// matches tells whether a window of width × height px matches a query of
// (width >= …rem) and (height >= …rem) conditions, "not all and" before
// them negating the whole.
function matches(query: string, width: number, height: number): boolean {
  const all = [...query.matchAll(/\((width|height) >= ([\d.]+)rem\)/g)].every(
    ([, side, rem]) => (side === 'width' ? width : height) >= Number(rem) * 16,
  )
  return query.startsWith('not all and ') ? !all : all
}

// The windows a rail or a sidebar needs room in: a phone upright, on
// its side, a desktop zoomed to 200 %, a tablet, a laptop, a desktop, and
// a desktop's window made low.
const windows = [
  { size: [390, 844], rail: false, sidebar: false },
  { size: [844, 390], rail: false, sidebar: false },
  { size: [640, 384], rail: false, sidebar: false },
  { size: [768, 1024], rail: true, sidebar: false },
  { size: [1024, 768], rail: true, sidebar: false },
  { size: [1440, 900], rail: true, sidebar: true },
  { size: [1440, 400], rail: false, sidebar: false },
] as const

test.each(windows)(
  'a window of $size has the rail ($rail) and the sidebar ($sidebar) only with room for them',
  ({ size: [width, height], rail, sidebar }) => {
    // The rail's classes are tablet:, the bar's max-tablet:, one or the
    // other in any window; the sidebar's sidebar:, within the rail's.
    expect(matches(variant('tablet'), width, height)).toBe(rail)
    expect(matches(variant('max-tablet'), width, height)).toBe(!rail)
    expect(matches(variant('sidebar'), width, height)).toBe(sidebar)
  },
)

test("the sidebar's variant comes after the rail's, so its classes win", () => {
  expect(css.indexOf('@custom-variant sidebar')).toBeGreaterThan(
    css.indexOf('@custom-variant tablet'),
  )
})
