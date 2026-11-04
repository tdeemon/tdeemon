import { Dialog } from '@base-ui/react/dialog'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  Bell,
  CalendarDays,
  ChartLine,
  ChefHat,
  Globe,
  History,
  House,
  KeyRound,
  Palette,
  Search,
  Settings,
  Target,
  Users,
  Utensils,
  type LucideIcon,
} from 'lucide-react'
import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type RefObject,
} from 'react'

import { $api } from '@/api/client'
import { useFindFoods } from '@/api/foods'
import { sessionQuery } from '@/api/session'
import { isRecipe } from '@/components/recipes/recipe-values'
import { Badge } from '@/components/ui/badge'
import { addDays, isDate, useFormat } from '@/i18n/format'
import { useMessages } from '@/i18n/messages'
import limits from '@/lib/limits.json'
import { matches } from '@/lib/search'
import { cn } from '@/lib/utils'

// A command of the palette's: what it shows (an icon, its name, a mark
// beside it), the keys that do it from anywhere (keys, as shown, and as
// aria-keyshortcuts says them, when they can be), the other words it is
// found by, and what it does; stale, a row of a search before the one
// typed, still on its way, which Enter does not run.
export interface Command {
  id: string
  icon: LucideIcon
  label: string
  keys?: string | undefined
  aria?: string | undefined
  words?: string | undefined
  badge?: string | undefined
  stale?: boolean | undefined
  run: () => void
}

// A group of the palette's commands, under its name.
interface Group {
  id: string
  title: string
  commands: Command[]
}

// How many foods a search shows: the palette leads to one, the foods'
// page lists them all.
const shownFoods = 6

// How many days back the days are offered, today the first.
const shownDays = 7

// CommandPalette is the app's command palette (⌘K or Ctrl+K, the
// sidebar's Search…, or typing on a day's page): a field, and under it
// what it finds and does, in groups, each row with its icon, its name and
// the keys that do it from anywhere. With nothing typed, the actions
// (actions, the frame's: log a meal, a weigh-in, yesterday's meal of the
// hour again, undo, the shortcuts) and the sections; typed, those whose
// words it starts, then the user's foods and recipes it finds (the
// server's search), the last week's days, or a date typed (YYYY-MM-DD),
// and the settings' pages. It is the ARIA combobox of a listbox: the
// field keeps the focus, ↑ and ↓ move the active row (aria-activedescendant),
// Enter or a click runs it, Escape (or ⌘K again) closes it, and a live
// region says how many rows there are. A modal dialog, it keeps the focus
// in itself, and gives it back where it was as it closes; a command runs
// once it has closed, so one that opens a sheet takes the focus from
// there. initial is the field's text as it opens. It only fades.
export function CommandPalette({
  open,
  onOpenChange,
  initial,
  actions,
  opener,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  initial: string
  actions: (now: number) => Command[]
  opener: RefObject<HTMLElement | null>
}) {
  const t = useMessages()
  // The command chosen, run once the palette has closed.
  const chosen = useRef<(() => void) | undefined>(undefined)
  // Each opening's own body: opened again while it fades out, the
  // palette starts anew, not with the words of before.
  const [opening, setOpening] = useState({ open, count: 0 })
  if (opening.open !== open) {
    setOpening({ open, count: open ? opening.count + 1 : opening.count })
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={onOpenChange}
      onOpenChangeComplete={(isOpen) => {
        const run = chosen.current
        chosen.current = undefined
        if (!isOpen && run) {
          run()
        }
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-40 bg-black/40 transition-opacity duration-(--duration-base) data-ending-style:opacity-0 data-starting-style:opacity-0" />
        <Dialog.Popup
          data-palette=""
          // Back to what had the focus before it (opener): the field
          // took it before the dialog could see that.
          finalFocus={opener}
          className="fixed top-[max(1rem,12dvh)] left-1/2 z-50 flex max-h-[min(36rem,calc(100dvh-2rem))] w-[min(35rem,calc(100vw-2rem))] -translate-x-1/2 flex-col gap-1.5 rounded-xl bg-card p-3 shadow-sheet transition-opacity duration-(--duration-base) outline-none data-ending-style:opacity-0 data-starting-style:opacity-0"
        >
          <Dialog.Title className="sr-only">{t.palette.title}</Dialog.Title>
          <PaletteBody
            key={opening.count}
            initial={initial}
            actions={actions}
            onChoose={(command) => {
              chosen.current = command.run
              onOpenChange(false)
            }}
            onClose={() => {
              onOpenChange(false)
            }}
          />
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

// Palette is what the palette holds while it is open: its field, its
// rows and the count of them. It starts anew each time.
function PaletteBody({
  initial,
  actions,
  onChoose,
  onClose,
}: {
  initial: string
  actions: (now: number) => Command[]
  onChoose: (command: Command) => void
  onClose: () => void
}) {
  const t = useMessages()
  const id = useId()
  const [query, setQuery] = useState(initial)
  // When it opened: what the actions are of (yesterday's meal of the
  // hour).
  const [now] = useState(Date.now)
  const [active, setActive] = useState(0)
  const list = useRef<HTMLDivElement>(null)
  const field = useRef<HTMLInputElement>(null)
  // The field takes the focus as it shows, before the dialog's own
  // (a frame later): a key typed fast after the one that opened the
  // palette is the field's, not the page's.
  useLayoutEffect(() => {
    field.current?.focus()
  }, [])
  const placed = useRef(false)
  const groups = useGroups(query, actions(now))
  const rows = groups.flatMap((g) => g.commands)
  // The active row, of those there are now.
  const at = Math.min(active, rows.length - 1)
  const optionId = (i: number) => `${id}-option-${String(i)}`

  // The active row stays in sight as the arrows move it.
  useEffect(() => {
    list.current?.querySelector(`[id="${optionId(at)}"]`)?.scrollIntoView({ block: 'nearest' })
  })

  function onKey(event: KeyboardEvent<HTMLInputElement>) {
    if (event.nativeEvent.isComposing) {
      return
    }
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      onClose()
      return
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      if (rows.length > 0) {
        const step = event.key === 'ArrowDown' ? 1 : -1
        setActive((Math.max(at, 0) + step + rows.length) % rows.length)
      }
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      const row = rows[at]
      if (row && row.stale !== true) {
        onChoose(row)
      }
    }
  }

  let index = 0
  return (
    <>
      {/* Its edge is the brand's while the field has the focus, as it
          has while the palette is open. */}
      <div className="flex h-12 shrink-0 items-center gap-2.5 rounded-lg border border-control-border bg-card px-3 focus-within:border-primary focus-within:ring-1 focus-within:ring-primary">
        <Search aria-hidden className="size-5 shrink-0 text-muted-foreground" />
        <input
          ref={field}
          role="combobox"
          aria-expanded
          aria-controls={`${id}-list`}
          aria-autocomplete="list"
          aria-activedescendant={at >= 0 ? optionId(at) : undefined}
          aria-label={t.palette.field}
          aria-describedby={`${id}-count`}
          autoComplete="off"
          autoCorrect="off"
          spellCheck={false}
          enterKeyHint="go"
          placeholder={t.palette.placeholder}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setActive(0)
          }}
          onKeyDown={onKey}
          // The field's first focus puts the caret after what was typed
          // to open it.
          onFocus={(e) => {
            if (!placed.current) {
              placed.current = true
              e.currentTarget.setSelectionRange(query.length, query.length)
            }
          }}
          className="h-full min-w-0 flex-1 bg-transparent text-base outline-none placeholder:text-muted-foreground"
        />
      </div>
      <div
        ref={list}
        id={`${id}-list`}
        // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- a <select> cannot be the combobox's popup of rows with icons and keys
        role="listbox"
        aria-label={t.palette.title}
        hidden={rows.length === 0}
        className="min-h-0 overflow-y-auto overscroll-contain"
      >
        {groups.map((group) => (
          // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- an <optgroup> belongs to a <select>, which this listbox cannot be
          <div key={group.id} role="group" aria-labelledby={`${id}-${group.id}`} className="pt-1">
            {/* The group's name, which names it to a screen reader. */}
            <div
              id={`${id}-${group.id}`}
              aria-hidden
              className="px-2 pt-1.5 pb-1 text-caption font-semibold text-muted-foreground"
            >
              {group.title}
            </div>
            {group.commands.map((command) => {
              const i = index++
              const Icon = command.icon
              return (
                // The field takes the keys (aria-activedescendant); a row,
                // a click.
                // oxlint-disable-next-line jsx-a11y/click-events-have-key-events -- the combobox's field takes the keys of its options
                <div
                  key={command.id}
                  id={optionId(i)}
                  // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role -- an <option> belongs to a <select>, which this listbox cannot be
                  role="option"
                  aria-selected={i === at}
                  aria-keyshortcuts={command.aria}
                  tabIndex={-1}
                  className={cn(
                    'flex min-h-11 cursor-pointer items-center gap-3 rounded-[10px] px-2 text-small',
                    i === at && 'bg-surface-2',
                  )}
                  onMouseMove={() => {
                    if (i !== at) {
                      setActive(i)
                    }
                  }}
                  onClick={() => {
                    onChoose(command)
                  }}
                >
                  <Icon aria-hidden className="size-4.5 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1 truncate">{command.label}</span>
                  {command.badge !== undefined && <Badge tone="primary">{command.badge}</Badge>}
                  {command.keys !== undefined && (
                    <kbd
                      aria-hidden
                      className="shrink-0 rounded-[5px] border border-control-border px-1.5 font-mono text-caption text-muted-foreground"
                    >
                      {command.keys}
                    </kbd>
                  )}
                </div>
              )
            })}
          </div>
        ))}
      </div>
      {rows.length === 0 && (
        <p className="px-2 py-3 text-small text-muted-foreground">{t.palette.none}</p>
      )}
      {/* A live region (its role, status): the count as it changes. */}
      <output id={`${id}-count`} className="sr-only">
        {t.palette.results(rows.length)}
      </output>
    </>
  )
}

// useGroups are the palette's groups of what query finds, each of the
// rows it has: with nothing typed the actions and the sections; typed,
// those whose words it starts (matches), the foods and recipes the
// server finds, the days and the settings' pages.
function useGroups(query: string, actions: Command[]): Group[] {
  const t = useMessages()
  const f = useFormat()
  const navigate = useNavigate()
  const q = query.trim()
  const foods = useFindFoods(q)
  const today = $api.useQuery('get', '/api/v1/me').data?.today
  const user = useQuery(sessionQuery).data?.user
  const admin = user?.role === 'admin'
  // A section's keys, unless the user turned the shortcuts of one key off.
  const singleKeys = user?.single_key_shortcuts !== false
  const s = t.shortcuts.sections
  const found = (c: Command) => matches(q, [c.label, c.words])

  const go = (letter: string, to: string, icon: LucideIcon, label: string): Command => ({
    id: `go-${to}`,
    icon,
    label,
    ...(singleKeys && { keys: `G ${letter}` }),
    run: () => {
      void navigate({ to })
    },
  })
  const sections = [
    go('D', '/', CalendarDays, s.today),
    go('F', '/foods', Utensils, s.foods),
    go('P', '/progress', ChartLine, s.progress),
    go('L', '/plan', Target, s.plan),
    go('S', '/settings', Settings, s.settings),
  ]
  if (q === '') {
    return [
      { id: 'actions', title: t.palette.actions, commands: actions },
      { id: 'go', title: t.palette.goTo, commands: sections },
    ].filter((g) => g.commands.length > 0)
  }

  const page = (to: string, icon: LucideIcon, label: string): Command => ({
    id: `page-${to}`,
    icon,
    label,
    run: () => {
      void navigate({ to })
    },
  })
  const settings = [
    page('/settings/appearance', Palette, t.settings.appearance),
    page('/settings/account', Globe, t.settings.account),
    page('/settings/devices', KeyRound, t.settings.devices),
    page('/settings/notifications', Bell, t.settings.notifications.title),
    page('/settings/household', House, t.settings.household),
    ...(admin ? [page('/settings/users', Users, t.settings.users)] : []),
    page('/settings/changes', History, t.changes.title),
  ]

  // The last week's days, today first, and a date typed of the history's.
  const dates =
    today === undefined ? [] : Array.from({ length: shownDays }, (_, i) => addDays(today, -i))
  if (today !== undefined && isDate(q) && q >= limits.history.since && q <= today) {
    if (!dates.includes(q)) {
      dates.push(q)
    }
  }
  const days = dates.map((date): Command => {
    const said = f.date(date, today)
    const named =
      date === today
        ? t.day.today
        : date === addDays(today ?? '', -1)
          ? t.meal.yesterday
          : undefined
    return {
      id: `day-${date}`,
      icon: CalendarDays,
      label: named === undefined ? said : `${named} · ${said}`,
      words: date,
      run: () => {
        void navigate({ to: '/', search: date === today ? {} : { date } })
      },
    }
  })
  const foodRows = (foods.data?.foods ?? []).slice(0, shownFoods).map((food): Command => ({
    id: `food-${food.id}`,
    icon: isRecipe(food) ? ChefHat : Utensils,
    label: food.brand === undefined ? food.name : `${food.name} · ${food.brand}`,
    badge: isRecipe(food) ? t.palette.recipe : undefined,
    stale: foods.isPlaceholderData,
    run: () => {
      void navigate({ to: '/foods/$id', params: { id: food.id } })
    },
  }))

  return [
    { id: 'actions', title: t.palette.actions, commands: actions.filter(found) },
    { id: 'go', title: t.palette.goTo, commands: sections.filter(found) },
    { id: 'foods', title: t.palette.foods, commands: foodRows },
    {
      id: 'days',
      title: t.palette.days,
      commands: days.filter(found),
    },
    { id: 'settings', title: t.palette.settings, commands: settings.filter(found) },
  ].filter((g) => g.commands.length > 0)
}
