import { useIsMutating } from '@tanstack/react-query'
import { ChevronLeft } from 'lucide-react'
import { useEffect, useId, useRef, useState, type ReactNode } from 'react'

import type { components } from '@/api/schema'
import { EstimateFields } from '@/components/day/estimate-fields'
import {
  entryWrong,
  sayWrong,
  unitsOf,
  type Entry,
  type Wrong,
} from '@/components/day/meal-entries'
import { FoodForm } from '@/components/foods/food-form'
import { Button } from '@/components/ui/button'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { uuidv7 } from '@/lib/uuid'

type FoodFields = components['schemas']['FoodFields']

// What the next step of the quick add makes: a food, an estimate, or an
// entry of a barcode (ScanEntry).
export type Making = 'food' | 'estimate' | 'scan'

// NewEntry is the quick add's next step, in the search's place: a food of
// the user's made in short (FoodForm's compact form) and added in the
// amount eaten now, or an estimate of a plate, its name and values; each
// added with the one button at its foot, Add to meal. name is what was
// searched for, which the food or the estimate is named to start with; a
// food may start from more (start, a barcode's product), which said says
// where it came from. onBack goes back to the search, and onAdd takes the
// entry, checked. A recipe's ingredient (ingredient) is added to the
// recipe, in the amount the recipe has. An estimate is a quick one: its
// kcal first, with the focus, its name and macros left out if need be
// (the app's name, 0 g); with onLog, as the meal holds nothing else, its
// primary button logs the meal of it alone (Enter too), named by what
// logAs makes of its kcal, and Add to meal beside it goes on to more.
export function NewEntry({
  making,
  name,
  start,
  said,
  onBack,
  onAdd,
  logAs,
  onLog,
  ingredient = false,
}: {
  making: Exclude<Making, 'scan'>
  name: string
  start?: Partial<FoodFields>
  said?: ReactNode
  onBack: () => void
  onAdd: (entry: Entry) => void
  logAs?: ((kcal: string | undefined) => string) | undefined
  onLog?: ((entry: Entry) => void) | undefined
  ingredient?: boolean
}) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const form = `${id}-form`
  const body = useRef<HTMLDivElement>(null)
  const estimateForm = useRef<HTMLFormElement>(null)
  // Whether the estimate goes on to the meal (Add to meal), not logged.
  const adding = useRef(false)
  const [estimate, setEstimate] = useState<Entry & { kind: 'estimate' }>(() => ({
    key: uuidv7(),
    kind: 'estimate',
    name,
    kcal: '',
    protein: '',
    carbs: '',
    fat: '',
  }))
  const [wrong, setWrong] = useState<Wrong | undefined>(undefined)
  // A food on its way: Add to meal waits for it.
  const saving = useIsMutating({ mutationKey: ['post', '/api/v1/foods/new'] }) > 0
  // An estimate that logs the meal of it alone.
  const logs = making === 'estimate' && onLog !== undefined && logAs !== undefined
  // The kcal typed, as the button says them: none while they are none.
  const parsedKcal = f.parse(estimate.kcal)
  const typedKcal = parsedKcal === undefined ? undefined : f.number(Math.round(parsedKcal))
  // The focus goes to the first field, as the button that opened the step
  // is hidden under it.
  useEffect(() => {
    body.current?.querySelector<HTMLElement>('input')?.focus()
  }, [])

  return (
    <>
      <div className="space-y-1 px-4 pt-2">
        <Button variant="ghost" size="sm" className="-ml-2.5" onClick={onBack}>
          <ChevronLeft aria-hidden />
          {t.log.back}
        </Button>
        <h3 className="text-h3">{making === 'food' ? t.log.newFood : t.log.anEstimateOf}</h3>
      </div>
      <div ref={body} className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4">
        {said !== undefined && (
          <p className="rounded-lg bg-surface-2 p-3 text-sm text-muted-foreground">{said}</p>
        )}
        {making === 'food' ? (
          <FoodForm
            compact={{
              id: form,
              name,
              ingredient,
              ...(start === undefined ? {} : { start }),
              // In the unit of what its values are for.
              onAdd: (food, amount) => {
                const unit = unitsOf(food)[0] ?? 'g'
                onAdd({ key: uuidv7(), kind: 'food', food, amount: f.field(amount), unit })
              },
            }}
          />
        ) : (
          <form
            ref={estimateForm}
            id={form}
            noValidate
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault()
              const filled = quickFilled(estimate, t.log.quickName)
              const why = entryWrong(filled, f.parse)
              if (why !== undefined) {
                setWrong(why)
                return
              }
              // Logged at once, but by Add to meal.
              const add = adding.current
              adding.current = false
              if (onLog !== undefined && !add) {
                onLog(filled)
              } else {
                onAdd(filled)
              }
            }}
          >
            <EstimateFields
              quick
              entry={estimate}
              wrong={wrong !== undefined}
              describedBy={wrong ? `${id}-wrong` : undefined}
              onChange={(e) => {
                if (e.kind === 'estimate') {
                  setEstimate(e)
                }
              }}
            />
            {wrong && (
              <p id={`${id}-wrong`} role="alert" className="text-sm text-destructive">
                {sayWrong(wrong, t, f.number)}
              </p>
            )}
          </form>
        )}
      </div>
      <div className="flex gap-2 border-t p-4 pb-[max(1rem,env(safe-area-inset-bottom))]">
        {/* No submit button of its own: Enter in a field presses the
            primary, Log. */}
        {logs && (
          <Button
            variant="outline"
            className="h-13 shrink-0 text-base"
            onClick={() => {
              adding.current = true
              estimateForm.current?.requestSubmit()
            }}
          >
            {t.log.addToMeal}
          </Button>
        )}
        <Button
          type="submit"
          form={form}
          className="h-13 min-w-0 flex-1 text-base"
          disabled={saving}
          focusableWhenDisabled
        >
          {ingredient ? t.recipes.addToRecipe : logs ? logAs(typedKcal) : t.log.addToMeal}
        </Button>
      </div>
    </>
  )
}

// or0 is a value as it is added: 0 when left out.
const or0 = (v: string) => (v.trim() === '' ? '0' : v)

// quickFilled is a quick estimate as it is added: its macros left out
// are 0 g, and its name left out the app's (name).
function quickFilled(
  estimate: Entry & { kind: 'estimate' },
  name: string,
): Entry & { kind: 'estimate' } {
  return {
    ...estimate,
    name: estimate.name.trim() === '' ? name : estimate.name,
    protein: or0(estimate.protein),
    carbs: or0(estimate.carbs),
    fat: or0(estimate.fat),
  }
}
