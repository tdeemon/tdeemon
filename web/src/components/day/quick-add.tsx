import { useLocation, useSearch } from '@tanstack/react-router'
import { useEffect, useId, useRef, useState, type RefObject } from 'react'
import { flushSync } from 'react-dom'

import { detailOf, statusOf } from '@/api/errors'
import { useLogMeal } from '@/api/meals'
import type { components } from '@/api/schema'
import type { Food, Meal } from '@/api/types'
import { Finder } from '@/components/day/finder'
import {
  entryValues,
  entryWrong,
  foodEntry,
  itemOf,
  repeatOf,
  type Entry,
  type Wrong,
} from '@/components/day/meal-entries'
import { mealName, namedBy, slotAt } from '@/components/day/meal-slot'
import { NewEntry, type Making } from '@/components/day/new-entry'
import { SaveStatus } from '@/components/day/save-status'
import { ScanEntry } from '@/components/day/scan-entry'
import { Tray } from '@/components/day/tray'
import { WeighInChip } from '@/components/day/weigh-in-chip'
import { isRecipe } from '@/components/recipes/recipe-values'
import { Sheet, SheetHead } from '@/components/sheet'
import { TimeInput } from '@/components/ui/time-input'
import { rounded, useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { touchScreen } from '@/lib/media'
import { fieldLabel, inputClass, linkTarget } from '@/lib/styles'
import { sayChange } from '@/lib/toast'
import { cn } from '@/lib/utils'
import { uuidv7 } from '@/lib/uuid'
import { clockAt, mealInstant, nightAfter, parseClock, toCome } from '@/lib/zoned'

type LogMealBody = components['schemas']['LogMealBody']

interface Day {
  // The logical day, YYYY-MM-DD, and whether it is today.
  date: string
  today: boolean
  // The user's clock: its time zone and the hour a day starts at.
  timeZone: string
  startHour: number
}

interface Start {
  // What the day's target leaves, when it has one.
  remaining: number | undefined
  // A food the meal starts with.
  initial?: Food | undefined
  // Whether the initial food waits for its amount, as a recipe does,
  // rather than being in the meal at once.
  pick?: boolean | undefined
  onLogged?: ((meal: Meal) => void) | undefined
}

// QuickAdd logs a meal of the day by hand, in a sheet titled by the meal
// of the day (Snack), its time under it: the foods of the user's that a
// search finds, or that were eaten lately or most, each added by its +
// in the amount it was eaten in last or opened for another, and
// estimates, gathered in the tray at the sheet's foot, there once it has
// an item, with their total, their amounts to change and what the day
// has left after it, and the one button that logs the meal (⌘↵ on a
// keyboard); or a meal of before, repeated in a tap, those eaten lately
// at the meal of the day first. A new food or an estimate (kcal alone,
// logged at once when it is all the meal has) is a step of its own, in
// the search's place. The meal is timed now (on another day, at noon),
// and named by what is in it, both of which can be changed. Each opening
// is a meal anew. The focus goes to
// the search, but on a touch screen, whose keyboard would cover the
// lists, until it is tapped. A food it opens with (initial) is in the
// meal; a recipe, or any food when pick is set, open for its amount
// instead, whose field has the focus, so its amount, portion or share is
// chosen, not taken over. opener is
// what opened it, which the focus
// goes back to; onLogged is told the meal logged, before the sheet
// closes, so the focus may go to the meal's row instead.
export function QuickAdd({
  open,
  onOpenChange,
  opener,
  ...meal
}: Day &
  Start & {
    open: boolean
    onOpenChange: (open: boolean) => void
    opener?: RefObject<HTMLElement | null>
  }) {
  const search = useRef<HTMLInputElement>(null)
  const amount = useRef<HTMLInputElement>(null)
  const choosing = meal.initial !== undefined && (meal.pick === true || isRecipe(meal.initial))
  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      // The meal of the day, which its time may change: the Composer's.
      title={undefined}
      initialFocus={choosing ? amount : search}
      tapToType
      finalFocus={opener}
      tall
      bare
    >
      <Composer
        {...meal}
        search={search}
        amount={amount}
        onDone={() => {
          onOpenChange(false)
        }}
      />
    </Sheet>
  )
}

// Composer is a meal being logged. Its id is made as it is sent, so the
// meal sent again (no answer came) is the same meal, not another: it
// holds still until it is sent again, or given up. Logged, it is done,
// and a toast says so, with what the server warned of it and Undo, and,
// over another page than its day's, the way to the day. Its
// next step (a new food, an estimate) takes the search's place, which
// stays as it was under it, and the tray's: one button, Add to meal (or,
// an estimate alone, Log snack).
function Composer({
  remaining,
  initial,
  pick,
  onLogged,
  search,
  amount,
  onDone,
  date,
  today,
  timeZone,
  startHour,
}: Day &
  Start & {
    search: RefObject<HTMLInputElement | null>
    amount: RefObject<HTMLInputElement | null>
    onDone: () => void
  }) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  // Whether the meal sent is one of before, logged again, which its toast
  // says.
  const repeating = useRef(false)
  // Whether the page is the meal's day's: today's is / itself.
  const onDay = useLocation({ select: (l) => l.pathname === '/' })
  const shown = useSearch({ strict: false, select: (s) => s.date })
  const dayShown = onDay && (shown === date || (today && shown === undefined))
  const log = useLogMeal(date, (answer) => {
    const said = repeating.current ? t.meal.repeated : t.toast.logged
    sayChange(
      {
        title: said(mealName(answer.meal, t), t.unit.kcal(f.number(answer.meal.total.kcal))),
        lines: answer.warnings.map((w) => t.meal.warnings[w.code]),
        to: dayShown ? undefined : date,
      },
      { last: { action: 'meal.create', entity: answer.meal.id }, date },
    )
    onLogged?.(answer.meal)
  })
  // A recipe it opens with, or any food when pick is set, waits in the
  // finder for its amount.
  const choosing = initial !== undefined && (pick === true || isRecipe(initial))
  const [entries, setEntries] = useState<Entry[]>(() =>
    initial === undefined || choosing ? [] : [foodEntry(initial, f.field)],
  )
  const [details, setDetails] = useState(false)
  // Whether the tray shows the items, for their amounts; and what is
  // wrong with one, by its key, once the meal is logged.
  const [trayOpen, setTrayOpen] = useState(false)
  const [wrongs, setWrongs] = useState<Readonly<Record<string, Wrong>>>({})
  const [description, setDescription] = useState('')
  // Now on today's, noon on another day's. Today's time left as it is
  // is the instant the meal is sent (eatenAt): the sheet may have been
  // open a while.
  const [time, setTime] = useState(() => (today ? clockAt(Date.now(), timeZone) : '12:00'))
  const [timeSet, setTimeSet] = useState(false)
  // The time as typed, on the language's clock, and whether it is none
  // (parseClock).
  const [timeText, setTimeText] = useState(() => f.clock(time))
  const [badTime, setBadTime] = useState(false)
  // A time still to come, which the server refuses: a meal is logged once
  // eaten.
  const [later, setLater] = useState(false)
  const status = statusOf(log.error)
  const refused = log.isError && (status === 409 || status === 422)
  // Sent, and no answer said whether it was taken: the meal stays as sent.
  const held = log.isPending || (log.isError && !refused)
  const night = nightAfter(time, startHour)
  const at = mealInstant(date, time, timeZone, startHour)
  const slotOf = slotAt(Date.parse(at), timeZone)
  const slot = t.meal.slots[slotOf]
  // When the sheet opened: what was eaten in the two weeks before counts
  // for what it offers first.
  const [now] = useState(Date.now)
  const [query, setQuery] = useState('')
  const [making, setMaking] = useState<Making | undefined>(undefined)
  // The button that opened the next step, which the focus goes back to.
  const maker = useRef<HTMLElement | null>(null)
  const logButton = useRef<HTMLButtonElement>(null)
  // What the sheet holds, whose keys are heard (onKey).
  const keys = useRef<HTMLDivElement>(null)
  const onKey = useRef<(e: KeyboardEvent) => void>(() => undefined)
  useEffect(() => {
    const element = keys.current
    const listener = (e: KeyboardEvent) => {
      onKey.current(e)
    }
    element?.addEventListener('keydown', listener)
    return () => {
      element?.removeEventListener('keydown', listener)
    }
  }, [])

  const values = entries.map((e) => entryValues(e, f.parse))
  const kcal = values.reduce((sum, v) => sum + (v?.kcal ?? 0), 0)
  // What the day has left after the meal: what it leaves less the meal's
  // kcal as they are shown, so the two add up.
  const after = remaining === undefined ? undefined : remaining - rounded(kcal)

  function send(body: LogMealBody) {
    log.mutate({ body }, { onSuccess: onDone })
  }

  // eatenAt is when a meal is sent as eaten: today's time left as it is,
  // the server's when it takes it, from the meal's id, made as it is sent;
  // or undefined when the time is still to come, which is said.
  function eatenAt(): { eaten_at?: string } | undefined {
    const typed = parseClock(timeText)
    setBadTime(typed === undefined)
    if (typed === undefined) {
      return undefined
    }
    const ahead = !(today && !timeSet) && toCome(at)
    setLater(ahead)
    if (ahead) {
      return undefined
    }
    return today && !timeSet ? {} : { eaten_at: at }
  }

  // submit logs the meal of these entries. Each was checked as it came,
  // and an amount changed in the tray since is checked again: one wrong
  // is said there, and nothing is sent.
  function submit(these: Entry[]) {
    const wrong = Object.fromEntries(
      these.flatMap((e) => {
        const why = e.kind === 'food' ? entryWrong(e, f.parse) : undefined
        return why === undefined ? [] : [[e.key, why] as const]
      }),
    )
    setWrongs(wrong)
    if (Object.keys(wrong).length > 0) {
      setTrayOpen(true)
      return
    }
    const when = eatenAt()
    const items = these.flatMap((e) => {
      const item = itemOf(e, f.parse)
      return item === undefined ? [] : [item]
    })
    if (items.length === 0 || when === undefined) {
      return
    }
    const what = description.trim()
    repeating.current = false
    send({ id: uuidv7(), ...(what ? { description: what } : {}), ...when, items })
  }

  function repeat(meal: Meal) {
    const when = eatenAt()
    if (when !== undefined) {
      repeating.current = true
      send({ ...repeatOf(meal), ...when })
    }
  }

  // added takes an entry into the meal from the lists, whose search
  // stays as it was (Finder): the next of the same results is a tap away.
  function added(entry: Entry) {
    setEntries((es) => [...es, entry])
  }

  // made takes an entry into the meal from the next step, back to the
  // search, emptied, with the focus; on a touch screen, whose keyboard
  // would cover the lists, the meal's button has it.
  function made(entry: Entry) {
    flushSync(() => {
      setEntries((es) => [...es, entry])
      setQuery('')
      setMaking(undefined)
    })
    const next = touchScreen() ? logButton.current : search.current
    next?.focus()
  }

  // back goes back to the search from the next step, to the button that
  // opened it.
  function back() {
    flushSync(() => {
      setMaking(undefined)
    })
    maker.current?.focus()
  }

  const label = t.log.logSlot[slotOf]
  const tray = (
    <Tray
      entries={entries}
      values={values}
      wrong={wrongs}
      open={trayOpen}
      onOpen={setTrayOpen}
      onChange={(entry) => {
        setEntries((es) => es.map((e) => (e.key === entry.key ? entry : e)))
        setWrongs(({ [entry.key]: _was, ...rest }) => rest)
      }}
      onRemove={(key) => {
        setEntries((es) => es.filter((e) => e.key !== key))
        setWrongs(({ [key]: _was, ...rest }) => rest)
      }}
      after={
        after === undefined
          ? undefined
          : after >= 0
            ? t.log.after(f.number(Math.round(after)))
            : t.log.afterOver(f.number(Math.round(-after)))
      }
      label={label}
      held={held}
      onLog={() => {
        submit(entries)
      }}
      logButton={logButton}
      status={
        later || badTime || refused || log.isError ? (
          <div className="space-y-1">
            {(later || badTime) && (
              <p role="alert" className="text-sm text-destructive">
                {badTime ? t.meal.timeFormat(f.clock('19:30')) : t.meal.timeToCome}
              </p>
            )}
            {refused ? (
              <p role="alert" className="text-sm text-destructive">
                {t.log.refused(detailOf(log.error))}
              </p>
            ) : (
              <SaveStatus
                change={log}
                // Again, as it was sent, under its id: the server takes the same
                // meal sent again for the one it has.
                onRetry={() => {
                  if (log.variables) {
                    send(log.variables.body)
                  }
                }}
              />
            )}
          </div>
        ) : null
      }
    />
  )

  // The ids of the foods in the meal, whose + the lists check.
  const chosen = new Set(entries.flatMap((e) => (e.kind === 'food' ? [e.food.id] : [])))

  // ⌘↵ or Ctrl+↵ logs the meal from anywhere in the sheet but a next
  // step, which has a button of its own.
  useEffect(() => {
    onKey.current = (e: KeyboardEvent) => {
      if (
        e.key === 'Enter' &&
        (e.metaKey || e.ctrlKey) &&
        making === undefined &&
        entries.length > 0 &&
        !held
      ) {
        e.preventDefault()
        submit(entries)
      }
    }
  })

  return (
    <div ref={keys} className="contents">
      <SheetHead
        title={slot}
        subtitle={
          <span className="flex flex-wrap items-baseline gap-x-1">
            <span>{t.log.at(today ? t.log.today : f.short(date), f.time(at, timeZone))}</span>
            <span aria-hidden>·</span>
            <button
              type="button"
              aria-expanded={details}
              aria-controls={`${id}-details`}
              aria-label={t.log.changeOf}
              disabled={held || making !== undefined}
              className={cn(
                linkTarget,
                'font-semibold text-primary underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-60',
              )}
              onClick={() => {
                setDetails((d) => !d)
              }}
            >
              {t.log.change}
            </button>
          </span>
        }
      />
      {making === 'scan' ? (
        <ScanEntry onBack={back} onAdd={made} />
      ) : (
        making !== undefined && (
          <NewEntry
            making={making}
            name={query.trim()}
            onBack={back}
            // With nothing else in the meal, an estimate's button logs it:
            // its kcal, typed, and logged; Add to meal goes on to more.
            onAdd={made}
            {...(making === 'estimate' && entries.length === 0
              ? {
                  logAs: (energy: string) => t.log.logAs(label, energy),
                  // In the tray, where what is said of it is.
                  onLog: (entry: Entry) => {
                    made(entry)
                    submit([entry])
                  },
                }
              : {})}
          />
        )
      )}
      {/* Under the next step, as it was. */}
      <div hidden={making !== undefined} className="contents">
        <fieldset disabled={held} className="min-h-0 min-w-0 flex-1 space-y-3 overflow-y-auto p-4">
          {details && (
            <div id={`${id}-details`} className="space-y-1">
              <div className="flex gap-3">
                <label className="flex-1 space-y-1">
                  <span className={cn(fieldLabel, 'block')}>{t.log.description}</span>
                  <input
                    value={description}
                    maxLength={200}
                    // The name the meal gets without one: what is in it.
                    placeholder={namedBy(
                      entries.map((e) => (e.kind === 'food' ? e.food.name : e.name.trim())),
                      t,
                    )}
                    autoComplete="off"
                    className={inputClass}
                    onChange={(e) => {
                      setDescription(e.currentTarget.value)
                    }}
                  />
                </label>
                <label className="w-28 space-y-1">
                  <span className={cn(fieldLabel, 'block')}>{t.log.time}</span>
                  <TimeInput
                    value={timeText}
                    aria-invalid={later || badTime}
                    aria-describedby={night ? `${id}-night` : undefined}
                    onChange={(e) => {
                      const typed = e.currentTarget.value
                      setTimeText(typed)
                      setBadTime(false)
                      setLater(false)
                      const clock = parseClock(typed)
                      if (clock !== undefined) {
                        setTime(clock)
                        setTimeSet(true)
                      }
                    }}
                    onBlur={() => {
                      // As the time it reads, on the language's clock:
                      // 7:30 PM, or 19:30, for 1930.
                      setTimeText((typed) => {
                        const clock = parseClock(typed)
                        return clock === undefined ? typed : f.clock(clock)
                      })
                    }}
                  />
                </label>
              </div>
              {night && (
                <p id={`${id}-night`} className="text-xs text-muted-foreground">
                  {t.log.nightAfter(f.clock(time))}
                </p>
              )}
            </div>
          )}
          <Finder
            search={search}
            query={query}
            onQuery={setQuery}
            at={{ slot: slotOf, now, timeZone, date }}
            onAdd={added}
            onRepeat={repeat}
            initial={choosing ? initial : undefined}
            amount={amount}
            chosen={chosen}
            onUnchoose={(food) => {
              setEntries((es) => es.filter((e) => e.kind !== 'food' || e.food.id !== food.id))
            }}
            stay
            onMake={(m, button) => {
              maker.current = button
              setMaking(m)
            }}
          />
          <WeighInChip date={date} />
        </fieldset>
        {tray}
      </div>
    </div>
  )
}
