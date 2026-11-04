import { ChevronDown, X } from 'lucide-react'
import { useId, type ReactNode, type Ref } from 'react'

import {
  entryAmountLabel,
  inUnit,
  sayWrong,
  unitsOf,
  type Entry,
  type Values,
  type Wrong,
} from '@/components/day/meal-entries'
import { entryUnitLabel } from '@/components/recipes/recipe-values'
import { Button } from '@/components/ui/button'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { apple } from '@/lib/shortcuts'
import { inputClass } from '@/lib/styles'
import { cn } from '@/lib/utils'

// Tray is the foot of the meal being logged, there once it has an item:
// how many, their kcal and macros, the items themselves under Edit
// amounts (open), each amount changed or the item taken out, with what
// the day has after the meal (after); and the one button that logs it,
// named by the meal of the day (label), ⌘↵ or Ctrl+↵ from anywhere in
// the sheet on a keyboard. wrong is what is wrong with an item's amount,
// by its key, found as the meal is logged. status is what is said of the
// meal sent (the time, the server's answer), which shows the tray without
// an item too: a meal of before repeated.
export function Tray({
  entries,
  values,
  wrong,
  open,
  onOpen,
  onChange,
  onRemove,
  after,
  label,
  held,
  onLog,
  logButton,
  status,
}: {
  entries: Entry[]
  values: (Values | undefined)[]
  wrong: Readonly<Record<string, Wrong>>
  open: boolean
  onOpen: (open: boolean) => void
  onChange: (entry: Entry) => void
  onRemove: (key: string) => void
  after: string | undefined
  label: string
  held: boolean
  onLog: () => void
  logButton: Ref<HTMLButtonElement>
  status: ReactNode
}) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const sum = (v: keyof Values) => values.reduce((s, x) => s + (x?.[v] ?? 0), 0)
  const kcal = Math.round(sum('kcal'))
  const items = entries.length > 0

  if (!items && status === null) {
    return null
  }
  return (
    <div className="shrink-0 space-y-2.5 border-t bg-popover p-4 pb-[max(1rem,env(safe-area-inset-bottom))]">
      {items && open && (
        <div id={`${id}-items`} className="max-h-[40dvh] space-y-2 overflow-y-auto">
          <ul aria-label={t.log.basket} className="divide-y">
            {entries.map((entry, i) => (
              <TrayItem
                key={entry.key}
                entry={entry}
                values={values[i]}
                wrong={wrong[entry.key]}
                held={held}
                onChange={onChange}
                onRemove={onRemove}
              />
            ))}
          </ul>
          {after !== undefined && <p className="text-small text-muted-foreground">{after}</p>}
        </div>
      )}
      {items && (
        <div className="flex items-center gap-2">
          <p className="min-w-0 flex-1 text-small">
            <b className="font-semibold">{t.meal.items(entries.length)}</b>{' '}
            <span className="text-muted-foreground">
              {[
                '',
                t.unit.kcal(f.number(kcal)),
                `${t.day.short.protein} ${f.number(Math.round(sum('protein')))}`,
                `${t.day.short.carbs} ${f.number(Math.round(sum('carbs')))}`,
                `${t.day.short.fat} ${f.number(Math.round(sum('fat')))}`,
              ].join(' · ')}
            </span>
          </p>
          <Button
            variant="ghost"
            size="sm"
            className="shrink-0 text-primary"
            aria-expanded={open}
            aria-controls={`${id}-items`}
            onClick={() => {
              onOpen(!open)
            }}
          >
            {t.log.editAmounts}
            <ChevronDown
              aria-hidden
              className={cn(
                'transition-transform motion-reduce:transition-none',
                open && 'rotate-180',
              )}
            />
          </Button>
        </div>
      )}
      {status}
      {items && (
        <Button
          ref={logButton}
          className="h-13 w-full text-base"
          // On its way, it stays in reach of the focus.
          disabled={held}
          focusableWhenDisabled
          aria-keyshortcuts={apple() ? 'Meta+Enter' : 'Control+Enter'}
          onClick={onLog}
        >
          {t.log.logAs(label, f.number(kcal))}
          <kbd
            aria-hidden
            className="hidden rounded-[5px] border border-primary-foreground/50 px-1.5 font-mono text-caption opacity-80 lg:inline"
          >
            {apple() ? '⌘↵' : 'Ctrl ↵'}
          </kbd>
        </Button>
      )}
    </div>
  )
}

// TrayItem is an item of the meal in the tray: its name, its kcal, and
// a way to take it out; a food's amount, in a unit of the food's, which
// works it out anew.
function TrayItem({
  entry,
  values,
  wrong,
  held,
  onChange,
  onRemove,
}: {
  entry: Entry
  values: Values | undefined
  wrong: Wrong | undefined
  held: boolean
  onChange: (entry: Entry) => void
  onRemove: (key: string) => void
}) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const name = entry.kind === 'food' ? entry.food.name : entry.name

  return (
    <li className="space-y-1.5 py-2">
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-small font-medium">
          {[
            name,
            ...(entry.kind === 'food' ? [entryAmountLabel(entry, t, f)] : []),
            ...(values === undefined ? [] : [t.unit.kcal(f.number(values.kcal))]),
          ].join(' · ')}
        </span>
        <Button
          variant="ghost"
          size="icon-sm"
          className="-my-1 shrink-0"
          aria-label={t.log.remove(name)}
          disabled={held}
          onClick={() => {
            onRemove(entry.key)
          }}
        >
          <X aria-hidden />
        </Button>
      </div>
      {entry.kind === 'food' && (
        <div className="flex gap-2">
          <input
            aria-label={t.log.amountIn(name)}
            aria-invalid={wrong !== undefined}
            aria-describedby={wrong ? `${id}-wrong` : undefined}
            value={entry.amount}
            inputMode={entry.food.source === 'recipe' ? 'text' : 'decimal'}
            autoComplete="off"
            disabled={held}
            className={cn(inputClass, 'w-24 tabular-nums')}
            onChange={(e) => {
              const { exact: _typed, ...rest } = entry
              onChange({ ...rest, amount: e.currentTarget.value })
            }}
          />
          <select
            aria-label={t.log.unitIn(name)}
            value={entry.unit}
            disabled={held}
            className={cn(inputClass, 'w-auto max-w-44 min-w-0')}
            onChange={(e) => {
              onChange(inUnit(entry, e.currentTarget.value, f.parse, f.field))
            }}
          >
            {unitsOf(entry.food).map((u) => (
              <option key={u} value={u}>
                {entryUnitLabel(entry.food, u, f.parse(entry.amount) ?? Number.NaN, t)}
              </option>
            ))}
          </select>
        </div>
      )}
      {wrong && (
        <p id={`${id}-wrong`} role="alert" className="text-sm text-destructive">
          {sayWrong(wrong, t, f.number)}
        </p>
      )}
    </li>
  )
}
