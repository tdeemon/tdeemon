import { useRef, useState } from 'react'
import { flushSync } from 'react-dom'

import { useCheckIn } from '@/api/day'
import type { Day } from '@/api/types'
import { DayNote } from '@/components/day/day-note'
import { Exclusion } from '@/components/day/exclusion'
import { useNothingMeasured } from '@/components/day/nothing-measured'
import { SaveStatus } from '@/components/day/save-status'
import { Stepper } from '@/components/day/stepper'
import { Button } from '@/components/ui/button'
import type { Scale } from '@/i18n/en'
import { reasonOf, useMessages } from '@/i18n/messages'
import { sayChange } from '@/lib/toast'

// CheckIn is what the user says of a day: their mood and hunger, a note,
// and whether calibration counts it; the day's type is the hero's
// (DayTypeChip). Each is saved as it is given. A day to come (future)
// takes its exclusion only. On a phone (row), and while the user has
// measured nothing yet and the day says nothing, it is a row that opens
// it: what was said of the day, or what it takes (how the day was, until
// anything is measured).
export function CheckIn({
  day,
  future = false,
  row = false,
}: {
  day: Day
  future?: boolean
  row?: boolean
}) {
  const t = useMessages()
  const nothing = useNothingMeasured(day.date, day.metrics)
  // said is a reason a day is left out for, as the language says it.
  const said = (r: string) => {
    const code = reasonOf(r)
    return code === undefined ? r : t.day.reasons[code]
  }
  const told =
    day.mood !== undefined ||
    day.hunger !== undefined ||
    (day.note ?? '') !== '' ||
    day.status === 'excluded' ||
    day.training_toggle !== undefined
  const [asked, setAsked] = useState(false)
  // The heading, which takes the focus once the row opens.
  const heading = useRef<HTMLHeadingElement>(null)

  if (!asked && !future && (row || (nothing && !told))) {
    const saying = [
      ...(day.mood === undefined || day.mood === 0
        ? []
        : [t.day.said(t.day.mood, t.day.moods[day.mood - 1] ?? '')]),
      ...(day.hunger === undefined || day.hunger === 0
        ? []
        : [t.day.said(t.day.hunger, t.day.hungers[day.hunger - 1] ?? '')]),
      ...((day.note ?? '') === '' ? [] : [t.day.withNote]),
      ...(day.status === 'excluded' ? [t.day.excluded(said(day.excluded_reason ?? ''))] : []),
    ]
    const any = saying.length > 0
    let line = t.day.checkInHint
    if (any) {
      line = saying.join(' · ')
    } else if (nothing) {
      line = t.day.checkInAsk
    }
    return (
      <section
        aria-labelledby="day-check-in"
        className="flex items-center justify-between gap-3 rounded-lg border bg-card py-3 pr-3 pl-4"
      >
        <div className="min-w-0 space-y-0.5">
          <h2 id="day-check-in" className="text-h3">
            {t.day.checkIn}
          </h2>
          <p className="text-small text-muted-foreground">{line}</p>
        </div>
        <Button
          variant="outline"
          size="sm"
          aria-label={any ? t.day.checkInChangeOf : t.day.checkInAddOf}
          className="shrink-0 font-semibold"
          onClick={() => {
            flushSync(() => {
              setAsked(true)
            })
            heading.current?.focus()
          }}
        >
          {any ? t.day.checkInChange : t.day.checkInAdd}
        </Button>
      </section>
    )
  }

  return (
    <section aria-labelledby="day-check-in" className="space-y-4 rounded-lg border bg-card p-4">
      <div className="flex items-baseline justify-between gap-2">
        <h2 id="day-check-in" ref={heading} tabIndex={-1} className="text-h3 outline-none">
          {t.day.checkIn}
        </h2>
        <span className="text-caption text-muted-foreground">{t.day.savedAsYouGo}</span>
      </div>
      {!future && (
        <>
          <ScaleField day={day} field="mood" label={t.day.mood} words={t.day.moods} tone="good" />
          <ScaleField
            day={day}
            field="hunger"
            label={t.day.hunger}
            words={t.day.hungers}
            tone="signal"
          />
          <DayNote day={day} />
        </>
      )}
      <Exclusion day={day} />
    </section>
  )
}

// ScaleField is the mood or the hunger of the day, on its scale.
function ScaleField({
  day,
  field,
  label,
  words,
  tone,
}: {
  day: Day
  field: 'mood' | 'hunger'
  label: string
  words: Scale
  tone: 'good' | 'signal'
}) {
  const t = useMessages()
  const change = useCheckIn(day.date, () => {
    sayChange(
      { title: t.toast.checkIn },
      { last: { action: 'day.checkin', entity: day.date }, date: day.date },
    )
  })
  const sent = change.variables?.body[field]
  // The step picked last shows while it is saved, and after it failed,
  // until it is sent again; then the day's.
  const value = (change.isPending || change.isError) && sent !== undefined ? sent : day[field]

  return (
    <Stepper
      label={label}
      words={words}
      value={value}
      tone={tone}
      status={<SaveStatus change={change} />}
      onPick={(n) => {
        change.mutate({
          params: { path: { date: day.date } },
          body: field === 'mood' ? { mood: n } : { hunger: n },
        })
      }}
    />
  )
}
