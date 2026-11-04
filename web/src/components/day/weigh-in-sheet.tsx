import { useQuery } from '@tanstack/react-query'
import type { RefObject } from 'react'

import { $api } from '@/api/client'
import { dayQuery } from '@/api/day'
import { usePreviousWeight } from '@/components/day/previous-weight'
import { WeighIn } from '@/components/day/weigh-in'
import { Sheet } from '@/components/sheet'
import { useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'

// WeighInSheet is today's weigh-in from anywhere (the command palette, W):
// a sheet of its field, which takes the focus, starting with today's
// weight when it has one and hinting with the day before's. Saved, the
// toast says so, with Undo, and the sheet closes; the focus goes back to
// opener, what had it before.
export function WeighInSheet({
  open,
  onOpenChange,
  opener,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  opener: RefObject<HTMLElement | null>
}) {
  const t = useMessages()
  const f = useFormat()
  const date = $api.useQuery('get', '/api/v1/me', {}, { enabled: open }).data?.today
  const today = useQuery({ ...dayQuery(date ?? 'today'), enabled: open && date !== undefined })
  const before = usePreviousWeight(date ?? '1970-01-01', open && date !== undefined)
  const weight = today.data?.metrics.find((m) => m.kind === 'weight')?.value
  // The field, which WeighIn keeps: the sheet's first stop, not its close
  // button.
  const field = {
    get current() {
      return globalThis.document.querySelector<HTMLElement>('[data-weigh-in-sheet] input')
    },
  }
  const close = () => {
    onOpenChange(false)
  }

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={t.palette.weighIn}
      initialFocus={field}
      finalFocus={opener}
    >
      <div data-weigh-in-sheet="" className="pb-2">
        {date !== undefined && today.isFetched && (
          <WeighIn
            date={date}
            label={t.day.metric.weight}
            initial={weight === undefined ? '' : f.field(weight, 1)}
            placeholder={before?.weight === undefined ? undefined : f.field(before.weight.value, 1)}
            focus
            onSaved={close}
          />
        )}
      </div>
    </Sheet>
  )
}
