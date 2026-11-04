import { useQuery, useQueryClient } from '@tanstack/react-query'

import { $api } from '@/api/client'
import { dayQuery } from '@/api/day'
import { detailOf, statusOf } from '@/api/errors'
import { useLogMeal } from '@/api/meals'
import { sessionQuery } from '@/api/session'
import { repeatOf } from '@/components/day/meal-entries'
import { mealName, slotAt } from '@/components/day/meal-slot'
import { addDays, useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import { sayChange, sayFailed } from '@/lib/toast'

// useRepeatYesterday is yesterday's meal of the meal of the day it is
// at an instant, as today's chips offer it (Yesterday's breakfast),
// logged again from anywhere: the command palette's and its key's, R.
// label is its name at an instant, none while today or yesterday is not
// known, or yesterday had no such meal: yesterday is asked for while
// shown (the palette open); run logs the one of now, if there is one,
// asking for yesterday first when it must. Done, a toast says so, with
// Undo; refused, why.
export function useRepeatYesterday(shown: boolean): {
  label: (now: number) => string | undefined
  run: () => Promise<void>
} {
  const t = useMessages()
  const f = useFormat()
  const today = $api.useQuery('get', '/api/v1/me').data?.today
  const timeZone = useQuery(sessionQuery).data?.user.timezone ?? 'UTC'
  const queryClient = useQueryClient()
  const before = dayQuery(addDays(today ?? '1970-01-01', -1))
  const yesterday = useQuery({ ...before, enabled: shown && today !== undefined })
  const log = useLogMeal(today ?? 'today', (answer) => {
    sayChange(
      {
        title: t.meal.repeated(
          mealName(answer.meal, t),
          t.unit.kcal(f.number(answer.meal.total.kcal)),
        ),
        lines: answer.warnings.map((w) => t.meal.warnings[w.code]),
      },
      { last: { action: 'meal.create', entity: answer.meal.id }, date: today ?? 'today' },
    )
  })
  // mealAt is yesterday's meal of the meal of the day of an instant.
  const mealAt = (now: number, meals = yesterday.data?.meals) => {
    const slot = slotAt(now, timeZone)
    const meal =
      today === undefined
        ? undefined
        : meals?.findLast((m) => slotAt(Date.parse(m.eaten_at), timeZone) === slot)
    return meal && { meal, slot }
  }

  return {
    label: (now) => {
      const at = mealAt(now)
      return at && t.palette.repeat(t.quick.yesterday[at.slot])
    },
    run: async () => {
      if (today === undefined || log.isPending) {
        return
      }
      let meals
      try {
        meals = (await queryClient.query({ ...before, staleTime: 'static' })).meals
      } catch (error) {
        sayFailed(problem(t, error))
        return
      }
      const at = mealAt(Date.now(), meals)
      if (at === undefined) {
        return
      }
      log.mutate(
        { body: repeatOf(at.meal) },
        {
          onError: (error) => {
            const status = statusOf(error)
            sayFailed(
              status === 409 || status === 422 ? t.log.refused(detailOf(error)) : problem(t, error),
            )
          },
        },
      )
    },
  }
}
