import { cn } from '@/lib/utils'

// A mark is how a series is drawn: a line, a line in its band, a dashed
// line, a band, dots, bars, or an outline around one. Its colour is a
// class of the token the series is drawn in: bg-…, or border-… of a
// dashed line or an outline.
export interface Mark {
  name: string
  mark: 'line' | 'lineBand' | 'dashed' | 'band' | 'dot' | 'bar' | 'outline'
  color: string
}

// The marks' shapes, as small as the legend's text.
const shapes: Record<Exclude<Mark['mark'], 'lineBand'>, string> = {
  line: 'h-0.5 w-4 rounded-full',
  dashed: 'w-4 border-t-2 border-dashed',
  band: 'h-2.5 w-4 rounded-xs opacity-25',
  dot: 'size-2 rounded-full',
  bar: 'size-2 rounded-xs',
  outline: 'size-2.5 rounded-xs border-2',
}

// Legend says what a chart's colours and marks mean, each with its name:
// HTML, which wraps on a phone, where ECharts' own would run into the
// chart. A chart's summary says it to a screen reader already.
export function Legend({ marks }: { marks: Mark[] }) {
  return (
    <ul aria-hidden className="flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted-foreground">
      {marks.map((m) => (
        <li key={m.name} className="flex items-center gap-1.5">
          {m.mark === 'lineBand' ? (
            <span className="relative flex h-2.5 w-4 shrink-0 items-center">
              <span className={cn('absolute inset-0 rounded-xs opacity-25', m.color)} />
              <span className={cn('relative h-0.5 w-full rounded-full', m.color)} />
            </span>
          ) : (
            <span className={cn('shrink-0', shapes[m.mark], m.color)} />
          )}
          {m.name}
        </li>
      ))}
    </ul>
  )
}
