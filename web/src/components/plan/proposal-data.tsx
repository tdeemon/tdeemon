import { $api } from '@/api/client'
import type { components } from '@/api/schema'
import { kcalShift } from '@/components/plan/change-words'
import { useDate } from '@/components/plan/proposal-text'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'

type Plan = components['schemas']['PlanBody']
type Proposal = components['schemas']['ProposalBody']
type Preview = components['schemas']['PreviewBody']

// ProposalData is what the data say beside a proposal, from calibration's
// summary, in a line: the TDEE and how far it is to be trusted, the pace
// measured against the phase's range, and the rate the proposal would
// make: of each period it sets, from its day, when it sets more than one.
// When the proposal moves the
// kcal where the data do not ask for it (the pace is in range, or they
// ask the other way), it says so, so the user weighs the coach's words
// against what was measured.
export function ProposalData({
  proposal,
  plan,
  preview,
}: {
  proposal: Proposal
  plan: Plan
  preview: Preview | undefined
}) {
  const t = useMessages()
  const f = useFormat()
  const date = useDate()
  const summary = $api.useQuery('get', '/api/v1/summary')
  if (summary.data === undefined) {
    return null
  }
  const { verdict, tdee } = summary.data
  const d = t.plan.pending.data
  const pct = (n: number) => f.percent(n, 2)
  const measured = verdict.measured_pct_week
  const target = verdict.target_pct_week
  const tolerance = verdict.tolerance_pct_week
  // The rate of each day the proposal starts something from, once a day.
  const expected = (preview?.checks ?? []).flatMap((ck) =>
    ck.expected_pct_week === undefined || ck.expected_kg_week === undefined
      ? []
      : [{ day: ck.day, pct: ck.expected_pct_week, kg: ck.expected_kg_week }],
  )
  const periods = expected
    .filter((e, i) => expected.findIndex((x) => x.day === e.day) === i)
    .toSorted((a, b) => a.day.localeCompare(b.day))
  const shift = kcalShift(proposal, plan.targets, plan.training_weekdays.length)
  let against: string | undefined
  if (shift !== 0 && verdict.rate === 'on_target') {
    against = d.inRange
  } else if (shift < 0 && verdict.gap_kcal > 0) {
    against = d.more
  } else if (shift > 0 && verdict.gap_kcal < 0) {
    against = d.fewer
  }

  const noTdee = !tdee.measured || tdee.kcal === undefined
  const noPace =
    verdict.rate === 'none' ||
    measured === undefined ||
    target === undefined ||
    tolerance === undefined
  const said = [
    noTdee && noPace
      ? d.nothing
      : !tdee.measured || tdee.kcal === undefined
        ? d.noTdee
        : tdee.trusted
          ? d.trusted(f.number(tdee.kcal), f.number(tdee.half_width ?? 0))
          : d.notTrusted(f.number(tdee.kcal)),
    ...(noTdee && noPace
      ? []
      : [
          verdict.rate === 'none' ||
          measured === undefined ||
          target === undefined ||
          tolerance === undefined
            ? d.noVerdict
            : d.rate[verdict.rate](pct(measured), pct(target - tolerance), pct(target + tolerance)),
        ]),
    ...(periods.length === 0
      ? []
      : [
          periods.length === 1 && periods[0] !== undefined
            ? d.expected(pct(periods[0].pct), f.number(periods[0].kg, 2))
            : d.expectedPeriods(
                periods
                  .map((e) => d.expectedFrom(date(e.day), pct(e.pct), f.number(e.kg, 2)))
                  .join(', '),
              ),
        ]),
  ]

  return (
    <div className="space-y-1 text-small">
      <p className="text-muted-foreground">
        <b className="font-semibold text-foreground">{d.title}</b> {said.join(' · ')}
      </p>
      {against !== undefined && <p className="font-medium text-warning">{against}</p>}
    </div>
  )
}
