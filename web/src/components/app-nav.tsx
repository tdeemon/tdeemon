import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  CalendarDays,
  ChartLine,
  Plus,
  Search,
  Settings,
  Target,
  Utensils,
  type LucideIcon,
} from 'lucide-react'
import { useId, type ReactNode } from 'react'

import { $api } from '@/api/client'
import { sessionQuery } from '@/api/session'
import { BrandMark } from '@/components/brand-mark'
import { UserMark } from '@/components/settings-avatar'
import { ThemeToggle } from '@/components/theme-toggle'
import { useMessages } from '@/i18n/messages'
import { cn } from '@/lib/utils'

// A section's link: on a phone a tab of the bar, and in a tablet's rail
// the same, its icon over its name; on a desktop a row of the sidebar.
// TanStack Router's Link marks the current one aria-current="page".
const sectionClass =
  'group flex min-h-12 flex-col items-center justify-center gap-0.5 rounded-md text-caption text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring aria-[current=page]:font-semibold aria-[current=page]:text-primary tablet:min-h-14 tablet:hover:text-foreground tablet:aria-[current=page]:hover:text-primary sidebar:h-10 sidebar:min-h-0 sidebar:flex-row sidebar:justify-start sidebar:gap-2.5 sidebar:px-3 sidebar:text-sm sidebar:font-medium sidebar:text-foreground sidebar:hover:bg-muted sidebar:aria-[current=page]:bg-primary-soft sidebar:aria-[current=page]:text-primary'

// A section's dot says something waits there: on the icon in the bar and
// the rail, after the name in the sidebar; to a screen reader, mark
// describes the link, whose name stays the section's. In the sidebar,
// the keys that lead there (G, then its letter) show as the pointer or
// the keyboard's focus is on it; the shortcuts' list says them to a
// screen reader.
function Section({
  to,
  icon: Icon,
  exact = false,
  mark,
  letter,
  children,
}: {
  to: '/' | '/foods' | '/progress' | '/plan' | '/settings'
  icon: LucideIcon
  exact?: boolean
  mark?: string | undefined
  letter: string
  children: ReactNode
}) {
  const markId = useId()
  return (
    <Link
      to={to}
      // The day is current on any day's page, /?date=… too, and only
      // there, not on every path under /.
      activeOptions={exact ? { exact: true, includeSearch: false } : {}}
      aria-describedby={mark === undefined ? undefined : markId}
      className={sectionClass}
    >
      <span className="relative flex shrink-0">
        <Icon
          aria-hidden
          strokeWidth={1.75}
          className="size-5.5 sidebar:size-4.5 sidebar:text-muted-foreground sidebar:group-aria-[current=page]:text-primary"
        />
        {mark !== undefined && (
          <span
            aria-hidden
            className="absolute -top-0.5 -right-1 size-2.5 rounded-full bg-primary ring-2 ring-card sidebar:hidden"
          />
        )}
      </span>
      {children}
      <kbd
        aria-hidden
        className="ml-auto hidden rounded-[5px] border px-1.5 font-mono text-caption text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100 sidebar:inline"
      >
        G {letter}
      </kbd>
      {mark !== undefined && (
        <>
          <span
            aria-hidden
            className="ml-auto hidden size-2 rounded-full bg-primary sidebar:block"
          />
          <span id={markId} hidden>
            {mark}
          </span>
        </>
      )}
    </Link>
  )
}

// The way a meal is logged from the bar's middle, and from the top of the
// rail: a button of the brand's colour, its icon alone.
const logFab =
  'flex size-13 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-[0_6px_16px_-4px] shadow-primary/50 outline-none hover:bg-primary/85 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-card'

// AppNav is the navigation between the app's sections, one for every
// width. On a phone it is a bar at the bottom, in reach of a thumb, with
// Log a meal in its middle; the settings are a phone's header's, the
// user's mark (SettingsAvatar). From 600 px wide (and 480 px high, or it
// would not hold them: index.css's tablet) it is a rail of 72 px at the
// left, which scrolls if it must: Log a meal first, the sections, and the
// settings and the user at its foot. From 1280 px it is a sidebar of
// 240 px: the brand, the way to
// search, Log a meal with its key, the sections with theirs, and at its
// foot the settings, the user and the theme. Each hides what is the
// others'. Log a meal is no section: it asks onLog for the meal to log
// over the page, from the button tapped; search asks onSearch. The
// plan's section has a dot while a proposal waits there for the user's
// answer. The bar steps aside for a sheet, which slides up over it, and
// is none on a form's page (bar false), whose own buttons end it.
export function AppNav({
  onLog,
  onSearch,
  bar = true,
}: {
  onLog: (from: HTMLElement) => void
  onSearch: () => void
  bar?: boolean
}) {
  const t = useMessages()
  const session = useQuery(sessionQuery)
  const name = session.data?.user.name ?? ''
  const waits = $api.useQuery('get', '/api/v1/plan').data?.pending !== undefined

  return (
    <nav
      aria-label={t.nav.label}
      className={cn(
        // A phone's bar, which slides down while a sheet is open (fades,
        // with less motion).
        'fixed inset-x-0 bottom-0 z-30 border-t bg-card/95 pr-[env(safe-area-inset-right)] pb-[env(safe-area-inset-bottom)] pl-[env(safe-area-inset-left)] backdrop-blur-md transition-[translate,opacity] duration-(--duration-slow) ease-sheet motion-reduce:transition-opacity sheet-open:max-tablet:translate-y-full motion-reduce:sheet-open:max-tablet:translate-y-0 motion-reduce:sheet-open:max-tablet:opacity-0',
        !bar && 'max-tablet:hidden',
        // A tablet's rail.
        'tablet:inset-y-0 tablet:right-auto tablet:flex tablet:w-[calc(4.5rem+env(safe-area-inset-left))] tablet:flex-col tablet:gap-4 tablet:overflow-y-auto tablet:border-t-0 tablet:border-r tablet:bg-card tablet:pt-4 tablet:pr-1 tablet:pb-[max(1rem,env(safe-area-inset-bottom))] tablet:pl-[calc(0.25rem+env(safe-area-inset-left))] tablet:backdrop-blur-none',
        // A desktop's sidebar.
        'sidebar:w-60 sidebar:gap-6 sidebar:px-4 sidebar:py-6',
      )}
    >
      <span className="hidden items-center gap-2.5 px-2 text-h3 sidebar:flex">
        <BrandMark />
        tdeemon
      </span>
      <div className="hidden flex-col gap-3 tablet:flex">
        <button
          type="button"
          // The foods' search for now, as / is.
          aria-keyshortcuts="/"
          className="hidden h-9 items-center gap-2 rounded-md border border-control-border bg-card px-2.5 text-small text-muted-foreground outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring sidebar:flex"
          onClick={onSearch}
        >
          <Search aria-hidden className="size-4" />
          {t.nav.search}
          <kbd
            aria-hidden
            className="ml-auto rounded-[5px] border border-control-border px-1.5 font-mono text-caption"
          >
            /
          </kbd>
        </button>
        <button
          type="button"
          aria-keyshortcuts="n"
          className={cn(
            logFab,
            'mx-auto sidebar:mx-0 sidebar:h-10 sidebar:w-full sidebar:gap-2 sidebar:rounded-md sidebar:font-semibold sidebar:shadow-none',
          )}
          onClick={(e) => {
            onLog(e.currentTarget)
          }}
        >
          <Plus aria-hidden strokeWidth={2.25} className="size-6 sidebar:size-5" />
          <span className="sr-only sidebar:not-sr-only">{t.nav.logMeal}</span>
          <kbd
            aria-hidden
            className="hidden rounded-[5px] border border-primary-foreground/40 px-1.5 font-mono text-caption opacity-80 sidebar:inline"
          >
            N
          </kbd>
        </button>
      </div>
      <ul className="grid grid-cols-5 items-center px-1 pt-1 tablet:flex tablet:flex-col tablet:items-stretch tablet:gap-1 tablet:p-0">
        <li>
          <Section to="/" icon={CalendarDays} exact letter="D">
            {t.nav.today}
          </Section>
        </li>
        <li>
          <Section to="/foods" icon={Utensils} letter="F">
            {t.nav.foods}
          </Section>
        </li>
        <li className="flex justify-center tablet:hidden">
          <button
            type="button"
            aria-label={t.nav.logMeal}
            className={logFab}
            onClick={(e) => {
              onLog(e.currentTarget)
            }}
          >
            <Plus aria-hidden strokeWidth={2.25} className="size-6" />
          </button>
        </li>
        <li>
          <Section to="/progress" icon={ChartLine} letter="P">
            {t.nav.progress}
          </Section>
        </li>
        <li>
          <Section to="/plan" icon={Target} mark={waits ? t.nav.waits : undefined} letter="L">
            {t.nav.plan}
          </Section>
        </li>
      </ul>
      {/* A phone's settings are its header's; the theme theirs. */}
      <div className="mt-auto hidden flex-col gap-1 tablet:flex">
        <Section to="/settings" icon={Settings} letter="S">
          {t.nav.settings}
        </Section>
        <div className="mt-2 flex items-center justify-center gap-2.5 border-t pt-3 sidebar:justify-start sidebar:px-3">
          <span data-testid="user" className="flex min-w-0 items-center gap-2.5 sidebar:flex-1">
            <UserMark className="sidebar:size-8" />
            <span className="sr-only truncate text-small sidebar:not-sr-only">{name}</span>
          </span>
          <span className="hidden sidebar:flex">
            <ThemeToggle />
          </span>
        </div>
      </div>
    </nav>
  )
}
