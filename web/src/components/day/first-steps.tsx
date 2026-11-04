import { useQuery } from '@tanstack/react-query'
import { Link, type LinkProps } from '@tanstack/react-router'
import {
  ChartLine,
  Check,
  ChevronRight,
  Copy,
  Scale,
  Sparkles,
  Utensils,
  type LucideIcon,
} from 'lucide-react'
import { useId, useRef, useState, type ReactNode } from 'react'

import { $api } from '@/api/client'
import { sessionQuery } from '@/api/session'
import type { Day, RecentMeal } from '@/api/types'
import { mealName } from '@/components/day/meal-slot'
import { WeighIn } from '@/components/day/weigh-in'
import { Button } from '@/components/ui/button'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import { putOff, usePutOff } from '@/lib/first-steps'
import { cn } from '@/lib/utils'

// useFirstSteps is the user whose first steps today's page shows, or
// undefined when it shows none: they show on today, while there is no
// plan, nor a proposal of one, nor a target, and the user has not put
// them off for later.
function useFirstSteps(day: Day): string | undefined {
  const me = $api.useQuery('get', '/api/v1/me')
  const plan = $api.useQuery('get', '/api/v1/plan')
  const user = useQuery(sessionQuery).data?.user.id
  const later = usePutOff(user)
  const before =
    plan.data?.phases.length === 0 &&
    plan.data.targets.length === 0 &&
    plan.data.pending === undefined
  if (me.data?.today !== day.date || day.target !== undefined || !before || later) {
    return undefined
  }
  return user
}

// earliest is when a recent meal was first eaten.
function earliest(meal: RecentMeal): string {
  return meal.eaten.reduce((first, e) => (e.at < first ? e.at : first), meal.eaten_at)
}

// FirstSteps are a new user's way to a plan, the hero of today's page in
// place of its own (children) while there is no plan: a card of the
// steps, each ticked as it is done, with how many are in words and a bar
// (the profile; a first weigh-in, in the card; an AI coach or a phone
// connected, which called with its access key; a first meal), and under
// it the step after them, in the coach's colour: what to ask the AI
// coach for a plan, to copy in a tap. Put off for later, they go until
// the plan's page brings them back; a proposal that waits, or a plan,
// ends them.
export function FirstSteps({ day, children }: { day: Day; children: ReactNode }) {
  const user = useFirstSteps(day)
  if (user === undefined) {
    return children
  }
  return <Steps day={day} user={user} />
}

// Steps are the first steps' cards.
function Steps({ day, user }: { day: Day; user: string }) {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const s = t.firstSteps
  const me = $api.useQuery('get', '/api/v1/me')
  const summary = $api.useQuery('get', '/api/v1/summary')
  const tokens = $api.useQuery('get', '/api/v1/tokens')
  const meals = $api.useQuery('get', '/api/v1/meals/recent')
  const weight = day.metrics.find((m) => m.kind === 'weight')
  const trend = summary.data?.trend
  const profile = me.data?.profile
  const caller = tokens.data?.tokens.find((k) => k.last_used_at !== undefined)
  // The first meal the user ate: of the recent ones (each eaten at one
  // time or more), the one eaten earliest; today's first until they come.
  const firstMeal =
    meals.data?.meals.toSorted((a, b) => (earliest(a) < earliest(b) ? -1 : 1))[0] ?? day.meals[0]
  const done = {
    profile: profile?.missing.length === 0,
    weighIn: weight !== undefined || trend !== undefined,
    connect: caller !== undefined,
    meal: firstMeal !== undefined,
  }
  const count = Object.values(done).filter(Boolean).length
  const steps = Object.keys(done).length

  // What was given, under a step done.
  let weighed: string | undefined
  if (weight !== undefined) {
    weighed = s.weighedToday(f.number(weight.value, 1))
  } else if (trend !== undefined) {
    weighed = s.trend(f.number(trend.kg, 1))
  }
  const profileSaid =
    profile === undefined
      ? undefined
      : [
          ...(profile.sex === undefined ? [] : [t.plan.sexes[profile.sex]]),
          ...(profile.age === undefined ? [] : [t.plan.age(profile.age)]),
          ...(profile.height_cm === undefined ? [] : [`${f.number(profile.height_cm, 1)} cm`]),
        ].join(' · ')

  return (
    <section aria-labelledby={id} className="space-y-4">
      <div className="rounded-lg border bg-card p-4 shadow-card lg:p-5">
        <div className="flex items-baseline justify-between gap-2">
          <h2 id={id} className="text-h2">
            {s.title}
          </h2>
          <span className="text-small text-muted-foreground tabular-nums">
            {s.count(count, steps)}
          </span>
        </div>
        {/* The count says it in words. */}
        <div aria-hidden className="mt-2.5 mb-1.5 h-2 overflow-hidden rounded-full bg-surface-2">
          <div
            className="h-full rounded-full bg-primary transition-[width] duration-(--duration-ring) ease-out motion-reduce:transition-none"
            style={{ width: `${String((count / steps) * 100)}%` }}
          />
        </div>
        <ol className="divide-y">
          <Step
            n={1}
            done={done.profile}
            title={done.profile ? s.profileDone : s.profile}
            to={{ to: '/plan/profile' }}
            meta={done.profile ? profileSaid : s.profileHint}
          />
          {done.weighIn ? (
            <Step n={2} done title={s.weighIn} meta={weighed} />
          ) : (
            <Step n={2} done={false}>
              <WeighIn date={day.date} label={s.weighIn} />
            </Step>
          )}
          <Step
            n={3}
            done={done.connect}
            title={done.connect ? s.connectDone : s.connect}
            to={{ to: '/settings/devices', search: { connect: 'choose' } }}
            meta={caller?.name ?? s.connectHint}
          />
          <Step
            n={4}
            done={done.meal}
            title={done.meal ? s.mealDone : s.meal}
            to={{ to: '/', search: { log: true } }}
            meta={firstMeal === undefined ? s.mealHint : mealName(firstMeal, t)}
          />
        </ol>
      </div>
      <AskCoach user={user} />
    </section>
  )
}

// Step is a first step: its number in a circle until it is done, then a
// tick, which a screen reader hears before its words; its title, and
// under it how it goes, or what was given once it is done. A step to do
// is a row to its page (to), or its own field (children).
function Step({
  n,
  done,
  title,
  meta,
  to,
  children,
}: {
  n: number
  done: boolean
  title?: string
  meta?: string | undefined
  to?: LinkProps
  children?: ReactNode
}) {
  const t = useMessages()
  const link = done || to === undefined ? undefined : to
  const said = meta !== undefined && meta !== ''
  const row = (
    <>
      <span
        aria-hidden
        className={cn(
          'grid size-7 shrink-0 place-items-center rounded-full text-caption transition-colors duration-(--duration-ring)',
          done ? 'bg-primary text-primary-foreground' : 'border-2 border-control-border',
        )}
      >
        {done ? <Check className="size-4" strokeWidth={2.5} /> : n}
      </span>
      <div className="min-w-0 flex-1">
        {children ?? (
          <>
            <p
              className={cn(
                'truncate',
                done ? 'font-medium text-muted-foreground' : 'font-semibold',
              )}
            >
              {title}
            </p>
            {said && <p className="truncate text-small text-muted-foreground">{meta}</p>}
          </>
        )}
      </div>
    </>
  )
  const rowClass = 'flex min-h-14 items-center gap-3 py-2'

  return (
    <li>
      <span className="sr-only">{done ? t.firstSteps.done : t.firstSteps.toDo}</span>
      {link === undefined ? (
        <div className={rowClass}>{row}</div>
      ) : (
        // The whole row is the link, named by its title and its line.
        <Link
          {...link}
          className={cn(
            rowClass,
            '-mx-2 rounded-md px-2 outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring',
          )}
        >
          {row}
          <ChevronRight aria-hidden className="size-4 shrink-0 text-muted-foreground" />
        </Link>
      )}
    </li>
  )
}

// AskCoach is the step after the first ones, the one only tdeemon has, in
// the coach's colour: what to write the user's own AI coach for a plan,
// copied in a tap (selected, for a browser that cannot copy it), where
// the plan it proposes lands, and the way to put the first steps off.
function AskCoach({ user }: { user: string }) {
  const t = useMessages()
  const s = t.firstSteps
  const id = useId()
  const quote = useRef<HTMLParagraphElement>(null)
  const [copied, setCopied] = useState<'yes' | 'failed' | undefined>(undefined)

  async function copy() {
    // Said anew at each copy, a second's too.
    setCopied(undefined)
    try {
      await navigator.clipboard.writeText(s.prompt)
      setCopied('yes')
    } catch {
      if (quote.current) {
        window.getSelection()?.selectAllChildren(quote.current)
      }
      setCopied('failed')
    }
  }

  return (
    <section
      aria-labelledby={id}
      className="rounded-lg border border-l-3 border-l-coach bg-coach-soft p-4 lg:p-5"
    >
      <h3 id={id} className="flex items-center gap-2 text-h3 text-coach">
        <Sparkles aria-hidden className="size-5 shrink-0" strokeWidth={1.75} />
        {s.ask}
      </h3>
      <p ref={quote} className="mt-2.5 rounded-md border bg-card p-3 text-small select-all">
        {s.prompt}
      </p>
      <div className="mt-2.5 flex flex-wrap gap-2">
        <Button size="sm" onClick={() => void copy()}>
          <Copy aria-hidden />
          {copied === 'yes' ? s.copied : s.copy}
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            putOff(user, true)
          }}
        >
          {s.later}
        </Button>
      </div>
      <output className="sr-only">{copied === 'yes' ? s.copiedSaid : ''}</output>
      {copied === 'failed' && (
        <p role="alert" className="mt-2 text-small text-destructive">
          {s.copyFailed}
        </p>
      )}
      <p className="mt-2 text-caption text-muted-foreground">{s.lands}</p>
    </section>
  )
}

// HowItWorks is the loop a new user's plan comes from, in four rows,
// beside the first steps on a desktop, while they show.
export function HowItWorks({ day }: { day: Day }) {
  const t = useMessages()
  const id = useId()
  const h = t.firstSteps.how
  const shown = useFirstSteps(day) !== undefined
  if (!shown) {
    return null
  }
  const rows: [LucideIcon, string, string, boolean][] = [
    [Utensils, h.log, h.logText, false],
    [Scale, h.weigh, h.weighText, false],
    [ChartLine, h.measure, h.measureText, false],
    [Sparkles, h.coach, h.coachText, true],
  ]

  return (
    <section aria-labelledby={id} className="rounded-lg border bg-card p-4 shadow-card lg:p-5">
      <h2 id={id} className="text-h2">
        {h.title}
      </h2>
      <ul className="mt-1.5 divide-y">
        {rows.map(([Icon, title, text, coach]) => (
          <li key={title} className="flex min-h-14 items-center gap-3 py-2">
            <span
              aria-hidden
              className={cn(
                'grid size-9 shrink-0 place-items-center rounded-md',
                coach ? 'bg-coach-soft text-coach' : 'bg-surface-2 text-muted-foreground',
              )}
            >
              <Icon className="size-5" strokeWidth={1.75} />
            </span>
            <div className="min-w-0">
              <p className="font-semibold">{title}</p>
              <p className="text-small text-muted-foreground">{text}</p>
            </div>
          </li>
        ))}
      </ul>
    </section>
  )
}
