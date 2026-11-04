import { useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useId, useRef, useState, type SubmitEvent } from 'react'

import { $api } from '@/api/client'
import { detailOf, statusOf } from '@/api/errors'
import type { components } from '@/api/schema'
import { week, type Weekday } from '@/components/autopilot/weekday'
import { useWarningText } from '@/components/plan/change-words'
import { targetNow } from '@/components/plan/phases'
import {
  targetFieldOf,
  useViolationAt,
  type DayType,
  type TargetField,
} from '@/components/plan/plan-edit'
import { useDate, useViolationText, violationsOf } from '@/components/plan/proposal-text'
import { Button } from '@/components/ui/button'
import { WeekSegments } from '@/components/ui/week-segments'
import { rounded, useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import limits from '@/lib/limits.json'
import { fieldLabel, inputClass, linkTarget } from '@/lib/styles'
import { sayChange } from '@/lib/toast'
import { cn } from '@/lib/utils'

type Plan = components['schemas']['PlanBody']
type DayTarget = components['schemas']['DayTargetBody']

const dayTypes: readonly DayType[] = ['training', 'rest']
const fields: readonly TargetField[] = ['kcal', 'protein', 'fat']

type Typed = Record<DayType, Record<TargetField, string>>

// TargetsForm changes the plan's targets by type of day from today on, or,
// before the plan's first target, that target from its day:
// the kcal, the protein and the fat of each, the carbs following what the
// kcal leave; and the training days, from today on. Without them it is
// one target, every day's. Only what changed is sent. What it can check it checks before it sends; what the
// limits refuse the server says, each at its field, and one of the
// user's own may be loosened in the limits, as their form asks. Saved, the
// plan has a revision of the user's, which the history takes back.
export function TargetsForm({ plan, onDone }: { plan: Plan; onDone: () => void }) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const queryClient = useQueryClient()
  const at = useViolationAt()
  const violation = useViolationText()
  const warning = useWarningText()
  // A day of another year than today's says its year.
  const date = useDate()
  const e = t.plan.edit
  // The target edited: today's, else the first one planned, from its day.
  const now =
    targetNow(plan.targets, plan.today) ?? plan.targets.find((tg) => tg.effective_from > plan.today)
  const from =
    now !== undefined && now.effective_from > plan.today ? now.effective_from : plan.today
  // A target planned after the one edited still takes over on its day.
  const later = plan.targets.find((tg) => tg.effective_from > from)
  const [typed, setTyped] = useState<Typed>(() => ({
    training: typedOf(now?.training, f.field),
    rest: typedOf(now?.rest, f.field),
  }))
  // The training days, from today on; without them, a target of every
  // day, the rest days', and the training days' kept as they are.
  const [trainingDays, setTrainingDays] = useState<Weekday[]>(() =>
    week.filter((d) => plan.training_weekdays.includes(d)),
  )
  // A target of the training days' own (not the rest days') holds
  // without training weekdays too: a day of a workout logged, or marked
  // so, is a training day. It is shown, never kept out of sight.
  const ownTraining =
    now !== undefined &&
    (now.training.kcal !== now.rest.kcal ||
      now.training.protein !== now.rest.protein ||
      now.training.fat !== now.rest.fat)
  const split = trainingDays.length > 0 || ownTraining
  const shown = split ? dayTypes : (['rest'] as const)
  const daysChanged =
    trainingDays.length !== plan.training_weekdays.length ||
    trainingDays.some((d) => !plan.training_weekdays.includes(d))
  // Whether a wrong field says so: from the first save on, so typing does
  // not scold.
  const [checked, setChecked] = useState(false)
  const form = useRef<HTMLFormElement>(null)
  const change = $api.useMutation('post', '/api/v1/plan/changes', {
    onSuccess: (answer, request) => {
      void queryClient.invalidateQueries()
      sayChange({
        title: request.body.targets === undefined ? e.saved.days : e.saved.targets,
        lines: answer.preview.checks.flatMap((ck) => ck.warnings.map(warning)),
      })
      onDone()
    },
  })

  const parsed = (d: DayType) => ({
    kcal: f.parse(typed[d].kcal),
    protein: f.parse(typed[d].protein),
    fat: f.parse(typed[d].fat),
  })
  // One target, every day's, is the training days' too.
  const values = {
    training: split ? parsed('training') : parsed('rest'),
    rest: parsed('rest'),
  }
  // A day's target as it is sent: the carbs are what the kcal leave, and
  // the server takes no other.
  const sent = (d: DayType) => ({
    kcal: values[d].kcal ?? 0,
    protein: values[d].protein ?? 0,
    fat: values[d].fat ?? 0,
  })
  const wrong = (d: DayType, k: TargetField): string | undefined => {
    const n = values[d][k]
    if (n === undefined) {
      return e.problem.number
    }
    if (k === 'kcal' && n <= 0) {
      return e.problem.positive
    }
    if (k === 'kcal' && n > limits.plan.maxKcal) {
      return e.problem.most(f.number(limits.plan.maxKcal))
    }
    return undefined
  }
  const carbsOf = (d: DayType): number | undefined => {
    const { kcal, protein, fat } = values[d]
    return kcal === undefined || protein === undefined || fat === undefined
      ? undefined
      : (kcal - 4 * protein - 9 * fat) / 4
  }
  // What the server refused, each at its field, and the rest under the
  // form; forgotten as soon as a field changes.
  const refused = change.isError ? violationsOf(change.error) : []
  const refusedAt = (d: DayType, k: TargetField) =>
    refused.filter((v) => {
      const of = targetFieldOf(v)
      return of?.field === k && of.days.includes(d)
    })
  const refusedForm = refused.filter((v) => targetFieldOf(v) === undefined)
  const loosen = refused.some((v) => v.level === 'user')

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    setChecked(true)
    const bad = shown.some(
      (d) => fields.some((k) => wrong(d, k) !== undefined) || (carbsOf(d) ?? 0) < 0,
    )
    if (bad) {
      // The first wrong field takes the focus, after the render that marks it.
      requestAnimationFrame(() => {
        form.current?.querySelector<HTMLInputElement>('[aria-invalid="true"]')?.focus()
      })
      return
    }
    // Only what changed is sent: the server takes no change of nothing.
    const targetChanged =
      now === undefined ||
      dayTypes.some((d) =>
        fields.some(
          (k) =>
            rounded(sent(d)[k], k === 'kcal' ? 0 : 1) !== rounded(now[d][k], k === 'kcal' ? 0 : 1),
        ),
      )
    if (!targetChanged && !daysChanged) {
      onDone()
      return
    }
    change.mutate({
      body: {
        ...(targetChanged && {
          targets: [{ from, training: sent('training'), rest: sent('rest') }],
        }),
        ...(daysChanged && { training_weekdays: trainingDays }),
      },
    })
  }

  return (
    <form
      ref={form}
      onSubmit={submit}
      noValidate
      className="space-y-4"
      aria-describedby={`${id}-from`}
    >
      <p id={`${id}-from`} className="text-small text-muted-foreground">
        {from === plan.today ? e.fromToday : e.fromDay(date(from))}
        {later !== undefined && ` ${e.laterTarget(date(later.effective_from))}`}
      </p>
      <WeekSegments
        multiple
        legend={t.plan.targets.trainingDays}
        describedBy={`${id}-days`}
        value={trainingDays}
        onChange={(chosen) => {
          change.reset()
          setTrainingDays(chosen)
        }}
      >
        <p id={`${id}-days`} className="text-small text-muted-foreground">
          {trainingDays.length > 0 ? e.daysHint : split ? e.workoutHint : e.noDaysHint}
        </p>
      </WeekSegments>
      {shown.map((d) => {
        const carbs = carbsOf(d)
        return (
          <fieldset key={d} className="min-w-0 space-y-3 rounded-lg border p-3">
            <legend className="px-1 font-medium">
              {split ? t.plan.targets[d] : t.plan.targets.everyDay}
            </legend>
            <div className="grid grid-cols-3 gap-2">
              {fields.map((k) => {
                const local = checked ? wrong(d, k) : undefined
                const server = refusedAt(d, k).map(at)
                const said = [...(local === undefined ? [] : [local]), ...server]
                const errorId = `${id}-${d}-${k}`
                return (
                  <div key={k} className="min-w-0 space-y-1">
                    <label className="block space-y-1">
                      <span className={cn(fieldLabel, 'block')}>{e[k]}</span>
                      <input
                        value={typed[d][k]}
                        inputMode="decimal"
                        autoComplete="off"
                        aria-invalid={said.length > 0}
                        aria-describedby={said.length > 0 ? errorId : undefined}
                        className={cn(inputClass, 'tabular-nums')}
                        onChange={(ev) => {
                          const value = ev.currentTarget.value
                          change.reset()
                          setTyped((was) => ({ ...was, [d]: { ...was[d], [k]: value } }))
                        }}
                      />
                    </label>
                    {said.length > 0 && (
                      <p id={errorId} className="text-caption text-destructive">
                        {said.join(' ')}
                      </p>
                    )}
                  </div>
                )
              })}
            </div>
            <p
              className={cn(
                'text-small',
                carbs !== undefined && carbs < 0 && checked
                  ? 'text-destructive'
                  : 'text-muted-foreground',
              )}
            >
              {carbs !== undefined && carbs < 0 && checked
                ? e.problem.over
                : e.carbs(carbs === undefined ? '–' : f.grams(Math.max(carbs, 0)))}
            </p>
          </fieldset>
        )
      })}
      {change.isError && (
        <div role="alert" className="space-y-1 text-sm text-destructive">
          {refused.length > 0 ? (
            <>
              <p>{e.refused}</p>
              {refusedForm.map((v) => (
                <p key={`${v.rule} ${v.day_type ?? ''}`}>{violation(v)}</p>
              ))}
            </>
          ) : (
            <p>
              {statusOf(change.error) === 422
                ? t.plan.refused(detailOf(change.error))
                : problem(t, change.error)}
            </p>
          )}
        </div>
      )}
      {loosen && (
        <Link to="/plan/limits" className={cn(linkTarget, 'block text-sm text-primary underline')}>
          {e.loosen}
        </Link>
      )}
      <Button type="submit" className="w-full" disabled={change.isPending}>
        {change.isPending ? e.saving : e.save}
      </Button>
    </form>
  )
}

// typedOf is a day's target as its fields start: each value as it is
// stored, or empty for none.
function typedOf(
  d: DayTarget | undefined,
  field: (n: number, digits?: number) => string,
): Record<TargetField, string> {
  return {
    kcal: d === undefined ? '' : field(d.kcal, 0),
    protein: d === undefined ? '' : field(d.protein, 1),
    fat: d === undefined ? '' : field(d.fat, 1),
  }
}
