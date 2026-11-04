import { useQuery } from '@tanstack/react-query'
import { Moon } from 'lucide-react'
import { useState } from 'react'

import { $api } from '@/api/client'
import { sessionQuery } from '@/api/session'
import type { Day } from '@/api/types'
import { MorningWeighIn } from '@/components/day/morning-weigh-in'
import { weekEnding, weekOf } from '@/components/day/week'
import { ProposalNudge } from '@/components/plan/proposal-nudge'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { hourAt } from '@/lib/zoned'

// The evening starts at 18:00 on the user's clock, and lasts until their
// day starts.
const evening = 18

// DayNow is what today's hero has under it, by the time of day: in the
// morning, while today has no weigh-in, the weigh-in (MorningWeighIn); a
// proposal that waits for the user's answer, at any hour (ProposalNudge);
// and in the evening, while none waits, how the day ends (DayEnd). Of
// today only, with a target: before one, the first steps lead.
export function DayNow({ day }: { day: Day }) {
  const me = $api.useQuery('get', '/api/v1/me')
  const plan = $api.useQuery('get', '/api/v1/plan')
  const user = useQuery(sessionQuery).data?.user
  // When the page opened.
  const [now] = useState(Date.now)
  if (me.data?.today !== day.date) {
    return null
  }
  // The evening runs on past midnight until the user's day starts: the
  // logical day is still today then.
  const hour = user === undefined ? undefined : hourAt(now, user.timezone)
  const late =
    user !== undefined && hour !== undefined && (hour >= evening || hour < user.day_start_hour)
  const waits = plan.data?.pending !== undefined

  return (
    <>
      {day.target !== undefined && <MorningWeighIn day={day} />}
      <ProposalNudge />
      {late && !waits && day.target !== undefined && day.meals.length > 0 && <DayEnd day={day} />}
    </>
  )
}

// DayEnd is the evening's card: the week's average as it ends if today
// ends with what it ate, against its target and how far from it a day
// (weekEnding); what a meal more would do is the ring's.
function DayEnd({ day }: { day: Day }) {
  const t = useMessages()
  const f = useFormat()
  const history = $api.useQuery('get', '/api/v1/history', {
    params: { query: weekOf(day.date, day.date) },
  })
  if (history.data === undefined) {
    return null
  }
  const ending = weekEnding(history.data.days, day.date, day.total.kcal, day.target?.kcal)
  if (ending.kcal === undefined || ending.target === undefined) {
    return null
  }
  const e = t.day.ending
  const kcal = f.number(ending.kcal)
  const under = ending.target - ending.kcal
  let said = e.on(kcal)
  if (under > 0) {
    said = e.under(kcal, f.number(under))
  } else if (under < 0) {
    said = e.over(kcal, f.number(-under))
  }

  return (
    <section
      aria-labelledby="day-end"
      className="flex items-start gap-3 rounded-lg border bg-card px-4 py-3"
    >
      <Moon aria-hidden className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
      <div className="space-y-0.5">
        <h2 id="day-end" className="text-h3">
          {e.title}
        </h2>
        <p className="text-small text-muted-foreground">{said}</p>
      </div>
    </section>
  )
}
