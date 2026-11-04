import type { components } from '@/api/schema'
import { priorOf, useChangeWords, type TargetValues } from '@/components/plan/change-words'
import { useDate, useLimitWords, usePlanWords } from '@/components/plan/proposal-text'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'

export type Proposal = components['schemas']['ProposalBody']
type Changes = components['schemas']['ChangesBody']
type Target = components['schemas']['TargetPeriodBody']
type DayTarget = components['schemas']['DayTargetBody']

const macros = ['protein', 'carbs', 'fat'] as const

// A Row of the table: what it is, the value now and the one to be, and
// how far apart they are ('' for nothing to tell); under its name, what
// else it changes (a type of day's macros).
interface Row {
  key: string
  name: string
  sub?: string | undefined
  now: string
  next: string
  delta: string
}

// A Table is the rows of the changes from one day.
interface Table {
  day: string
  rows: Row[]
}

// ProposalDiff says what a change of the plan changes, and only that: a
// phase in a line of words (new, deleted, or each value it changes), and
// the rest as tables of now → new with the difference, a table for each
// day a change starts on: each type of day's kcal, with the macros it
// changes under it (all of them for a target new or deleted; a target the
// same on both types of day, once, of every day), the training days and
// the user's limits. A target of a later day is told against the one the
// change sets before it. A revision says it as before → after (past).
export function ProposalDiff({
  changes,
  targets,
  today,
  past = false,
}: {
  changes: Changes
  targets: readonly Target[]
  today: string
  past?: boolean
}) {
  const t = useMessages()
  const f = useFormat()
  const date = useDate()
  const plan = usePlanWords()
  const words = useChangeWords()
  const limit = useLimitWords()
  const c = t.plan.change
  const d = t.plan.diff
  const s = t.day.short
  const none = d.none
  const signed = (n: number) => (Math.round(n) === 0 ? none : f.signed(n))

  // dayRow is a type of day's row: its kcal, and under it each macro the
  // change changes, or all of them for a target new or deleted.
  const dayRow = (
    from: string,
    days: 'training' | 'rest' | 'every',
    before: DayTarget | undefined,
    after: DayTarget | undefined,
  ): Row => {
    const whole = before ?? after
    const sub =
      before !== undefined && after !== undefined
        ? macros
            .filter((m) => f.grams(before[m]) !== f.grams(after[m]))
            .map((m) => c.grams(s[m], f.grams(before[m]), f.grams(after[m])))
            .join(' · ')
        : whole === undefined
          ? ''
          : `${macros.map((m) => `${s[m]} ${f.grams(whole[m])}`).join(' · ')} g`
    return {
      key: `${from} ${days}`,
      name: d.row(d.days[days], d.what.kcal),
      sub: sub === '' ? undefined : sub,
      now: before === undefined ? none : f.number(before.kcal),
      next: after === undefined ? none : f.number(after.kcal),
      delta: before === undefined || after === undefined ? '' : signed(after.kcal - before.kcal),
    }
  }
  // unsplit tells a target the same on both types of day, as shown.
  const same = (a: DayTarget, b: DayTarget) =>
    f.number(a.kcal) === f.number(b.kcal) && macros.every((m) => f.grams(a[m]) === f.grams(b[m]))
  const unsplit = (v: TargetValues | undefined) => v === undefined || same(v.training, v.rest)

  const tables = new Map<string, Row[]>()
  const add = (day: string, rows: Row[]) => {
    tables.set(day, [...(tables.get(day) ?? []), ...rows])
  }
  // The target the change sets before each, of the days it changes.
  let set: TargetValues | undefined
  for (const ch of changes.targets.toSorted((a, b) => a.from.localeCompare(b.from))) {
    const before = ch.before ?? set ?? priorOf(ch, targets)
    const after = ch.after
    set = after
    add(
      ch.from,
      unsplit(before) && unsplit(after)
        ? [dayRow(ch.from, 'every', before?.rest, after?.rest)]
        : (['training', 'rest'] as const).map((days) =>
            dayRow(ch.from, days, before?.[days], after?.[days]),
          ),
    )
  }
  const w = changes.training_weekdays
  if (w !== undefined) {
    add(today, [
      {
        key: 'weekdays',
        name: c.weekdays,
        now: w.before.length === 0 ? none : plan.days(w.before),
        next: w.after.length === 0 ? none : plan.days(w.after),
        delta: signed(w.after.length - w.before.length),
      },
    ])
  }
  if (changes.limits.length > 0) {
    add(
      today,
      changes.limits.map((ch) => ({
        key: `limit ${ch.name}`,
        name: limit.label(ch.name),
        now: limit.value(ch.name, ch.before),
        next: limit.value(ch.name, ch.after),
        delta: ch.loosens ? c.loosened : c.tightened,
      })),
    )
  }
  const sorted: Table[] = [...tables.entries()]
    .toSorted(([a], [b]) => a.localeCompare(b))
    .map(([day, rows]) => ({ day, rows }))

  return (
    <div className="space-y-3">
      {changes.phases.length > 0 && (
        <ul className="space-y-2 text-sm">
          {changes.phases.map((ch) => (
            <li key={`phase ${ch.start}`}>
              {ch.before === undefined || ch.after === undefined ? (
                <>
                  <span className="block font-medium">
                    {ch.before === undefined
                      ? c.newPhase(date(ch.start))
                      : c.deletedPhase(date(ch.start))}
                  </span>
                  {ch.after !== undefined && <span className="block">{plan.phase(ch.after)}</span>}
                  {ch.before !== undefined && (
                    <span className="block text-muted-foreground">
                      {c.was(plan.phase(ch.before))}
                    </span>
                  )}
                </>
              ) : (
                <>
                  <span className="block font-medium">{c.phase(date(ch.start))}</span>
                  <span className="block">{words.phase(ch.before, ch.after).join(' · ')}</span>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
      {sorted.map((table) => (
        <table
          key={table.day}
          className="w-full table-fixed border-collapse text-small tabular-nums"
        >
          <colgroup>
            <col />
            <col className="w-[4.5rem]" />
            <col className="w-[4.5rem]" />
            <col className="w-14" />
          </colgroup>
          <thead>
            <tr className="border-b text-caption text-muted-foreground">
              <th scope="col" className="py-1 pr-2 text-left font-medium">
                {d.from(date(table.day))}
              </th>
              <th scope="col" className="px-1.5 py-1 text-right font-medium">
                {past ? d.before : d.now}
              </th>
              <th scope="col" className="px-1.5 py-1 text-right font-medium">
                {past ? d.after : d.next}
              </th>
              <th scope="col" className="py-1 pl-1.5 text-right font-medium">
                <abbr title={d.deltaName} className="no-underline">
                  Δ
                </abbr>
              </th>
            </tr>
          </thead>
          <tbody>
            {table.rows.map((row) => (
              <tr key={row.key} className="border-b last:border-b-0">
                <th scope="row" className="py-1.5 pr-2 text-left font-medium">
                  {row.name}
                  {row.sub !== undefined && (
                    <span className="block text-caption font-normal text-muted-foreground">
                      {row.sub}
                    </span>
                  )}
                </th>
                <td className="px-1.5 py-1.5 text-right align-top text-muted-foreground">
                  {row.now}
                </td>
                <td className="px-1.5 py-1.5 text-right align-top font-semibold">{row.next}</td>
                <td className="py-1.5 pl-1.5 text-right align-top font-semibold text-coach">
                  {row.delta}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ))}
    </div>
  )
}
