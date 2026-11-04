import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

// The end-to-end tests, read as text: a test narrowed while it was
// written, and left so, would pass in CI having checked less. #258 left
// the a11y checks on the plan's screens alone, unseen until #260.
const e2e = join(import.meta.dirname, '..', 'e2e')

// narrowed returns the lines of source that run fewer tests than it has:
// a test, a group or a step of only (Playwright's forbidOnly catches it
// too, but only in CI), and a11y's screens filtered, sliced or picked.
function narrowed(source: string): string[] {
  return source
    .split('\n')
    .filter(
      (line) =>
        /\b(?:test|describe|it)(?:\.\w+)*\.only\s*\(/.test(line) ||
        /\bscreens\s*\.\s*(?:filter|slice|find|splice)\s*\(/.test(line),
    )
    .map((line) => line.trim())
}

test('narrowed finds an only and a filter of the screens, and nothing else', () => {
  expect(
    narrowed(
      [
        "test.only('a', () => {})",
        "test.describe.only('b', () => {})",
        '  await inTurn(',
        "    screens.filter((s) => s.name.startsWith('plan')),",
        'for (const s of screens.slice(0, 2)) {}',
        "test('only the day', () => {})",
        'const shown = screens.map((s) => s.name)',
        "sizes.filter((s) => s.name === 'phone')",
      ].join('\n'),
    ),
  ).toEqual([
    "test.only('a', () => {})",
    "test.describe.only('b', () => {})",
    "screens.filter((s) => s.name.startsWith('plan')),",
    'for (const s of screens.slice(0, 2)) {}',
  ])
})

test('no end-to-end test is narrowed', () => {
  const found = readdirSync(e2e)
    .filter((name) => name.endsWith('.ts'))
    .flatMap((name) =>
      narrowed(readFileSync(join(e2e, name), 'utf8')).map((line) => `${name}: ${line}`),
    )
  expect(found).toEqual([])
})
