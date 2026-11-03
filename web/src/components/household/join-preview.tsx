import { useState } from 'react'

import { $api } from '@/api/client'
import { statusOf } from '@/api/errors'
import { codeOf, useAccept, type HouseholdInvite, type MergeReport } from '@/api/household'
import { LoadError } from '@/components/load-error'
import { Skeleton } from '@/components/skeleton'
import { Button } from '@/components/ui/button'
import type { Messages } from '@/i18n/en'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import { sayChange } from '@/lib/toast'

// refusal says why joining was refused: the invite gone, a code of the
// server's, or what failed.
function refusal(t: Messages, error: unknown): string {
  if (statusOf(error) === 404) {
    return t.household.gone
  }
  const code = codeOf(error, 'path.id')
  const refusals: Record<string, string> = t.household.joinRefusals
  return (code !== undefined ? refusals[code] : undefined) ?? t.household.failed(problem(t, error))
}

// Counts is a line of the report: what it is, and how many foods and
// recipes; none, it is not shown.
function Counts({ label, counts }: { label: string; counts: MergeReport['merged'] }) {
  const t = useMessages()
  if (counts.foods === 0 && counts.recipes === 0) {
    return null
  }
  return (
    <div className="flex items-baseline justify-between gap-3 py-2">
      <dt className="text-small text-muted-foreground">{label}</dt>
      <dd className="shrink-0 font-semibold tabular-nums">
        {t.household.counts(counts.foods, counts.recipes)}
      </dd>
    </div>
  )
}

// Report is what joining does to the user's foods: how many become one
// with the household's, come to them and go to it, and each renamed or
// losing a barcode, by name.
function Report({ report }: { report: MergeReport }) {
  const t = useMessages()
  return (
    <div className="space-y-4">
      <dl className="divide-y rounded-lg border bg-card px-4">
        <Counts label={t.household.merged} counts={report.merged} />
        <Counts label={t.household.incoming} counts={report.received} />
        <Counts label={t.household.given} counts={report.given} />
      </dl>
      {report.renamed.length > 0 && (
        <section className="space-y-1">
          <h3 className="text-small font-semibold">{t.household.renamed}</h3>
          <ul className="space-y-1 text-small">
            {report.renamed.map((r) => (
              <li key={r.from} className="break-words">
                {r.from} → {r.to}
              </li>
            ))}
          </ul>
        </section>
      )}
      {report.dropped.length > 0 && (
        <section className="space-y-1">
          <h3 className="text-small font-semibold">{t.household.dropped}</h3>
          <ul className="space-y-1 text-small">
            {report.dropped.map((d) => (
              <li key={`${d.food}/${d.code}`} className="break-words">
                {t.household.barcodeOf(d.code, d.food)}
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  )
}

// JoinPreview is what accepting invite does, from the merge's own dry run,
// and the way to do it. Its accept does the merge it showed: one that
// changed since is refused, and the new one shown, to accept again.
export function JoinPreview({ invite, onDone }: { invite: HouseholdInvite; onDone: () => void }) {
  const t = useMessages()
  const preview = $api.useQuery(
    'get',
    '/api/v1/household/invites/{id}/preview',
    { params: { path: { id: invite.id } } },
    { gcTime: 0 },
  )
  const accept = useAccept()
  const [changed, setChanged] = useState(false)

  if (preview.isPending) {
    return <Skeleton blocks={['h-32', 'h-10']} />
  }
  // A preview that failed, the first or a new one after the merge
  // changed: the invite gone, a refusal, or what failed, with a way to ask
  // again. An old one is no merge to accept.
  const failed =
    preview.error === null ? null : statusOf(preview.error) === 404 ||
      statusOf(preview.error) === 409 ? (
      <p role="alert" className="text-sm text-destructive">
        {refusal(t, preview.error)}
      </p>
    ) : (
      <LoadError
        title={t.loadFailed.household}
        error={preview.error}
        onRetry={() => void preview.refetch()}
      />
    )
  if (preview.data === undefined) {
    return failed
  }
  const { report, fingerprint } = preview.data

  // join waits for the answer itself, not by the call's callbacks, which
  // go with the sheet when the household, joined, has the invite no more.
  async function join() {
    setChanged(false)
    try {
      await accept.mutateAsync({ params: { path: { id: invite.id } }, body: { fingerprint } })
    } catch (error) {
      if (codeOf(error, 'body.fingerprint') === 'merge_changed') {
        accept.reset()
        setChanged(true)
        void preview.refetch()
      }
      // Any other is said below.
      return
    }
    sayChange({ title: t.household.joined(invite.by.name) })
    onDone()
  }

  return (
    <div className="space-y-4">
      <p className="text-small text-muted-foreground">{t.household.previewHint}</p>
      {failed}
      {changed && failed === null && (
        <output className="block rounded-lg border border-warning/40 bg-warning-soft p-3 text-sm">
          {t.household.changed}
        </output>
      )}
      <Report report={report} />
      {accept.isError && (
        <p role="alert" className="text-sm text-destructive">
          {refusal(t, accept.error)}
        </p>
      )}
      <Button
        className="w-full"
        disabled={accept.isPending || preview.isFetching || failed !== null}
        onClick={() => void join()}
      >
        {accept.isPending ? t.household.joining : t.household.join}
      </Button>
    </div>
  )
}
