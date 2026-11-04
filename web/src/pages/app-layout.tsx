import { useQuery } from '@tanstack/react-query'
import { Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { Keyboard, Plus, Repeat2, Scale, Undo2 } from 'lucide-react'
import { useRef, useState } from 'react'

import { pushDeviceQuery } from '@/api/push'
import { sessionQuery } from '@/api/session'
import { AppNav } from '@/components/app-nav'
import { CommandPalette, type Command } from '@/components/command-palette'
import { LogToday } from '@/components/day/log-today'
import { useRepeatYesterday } from '@/components/day/repeat-yesterday'
import { WeighInSheet } from '@/components/day/weigh-in-sheet'
import { ShortcutsDialog } from '@/components/shortcuts-dialog'
import { StaleBar } from '@/components/stale-bar'
import { useMessages } from '@/i18n/messages'
import { formPage } from '@/lib/form-page'
import { LogHereContext, type LogHere } from '@/lib/log-here'
import { apple, useShortcuts, useSingleKeyShortcuts } from '@/lib/shortcuts'
import { useUndoLatest } from '@/lib/undo'
import { cn } from '@/lib/utils'

// The frame of the app's sections: the navigation between them (a bar at
// the bottom on a phone, none on a form's page; a rail on a tablet; a
// sidebar on a desktop) and the page, whose end keeps clear of the bar
// and whose sides of a phone's notch in landscape (viewport-fit=cover in
// index.html gives the page the whole screen);
// over the page, a bar while what it shows could not be asked for again.
// The keyboard's first stop skips the sections to the page. Log a meal
// opens the meal to log on today over the page, which stays as it was;
// today's page logs it itself (LogHere).
// The keyboard has a shortcut to the command palette (⌘K or Ctrl+K), to
// log a meal (N), a weigh-in (W), to find a food (/), to undo the latest
// change (⌘Z or Ctrl+Z, which a toast says), to each section (G, then its
// letter), and to the list of them (?). On a day's page, a letter or a
// digit typed outside a field, and none of those keys, opens the palette
// with it (useShortcuts' typed). The palette's actions are these, each
// with its key; yesterday's meal again has none, as no one key writes a
// meal from anywhere.
export function AppLayout() {
  const t = useMessages()
  const navigate = useNavigate()
  const main = useRef<HTMLElement>(null)
  const [shortcuts, setShortcuts] = useState(false)
  const latest = useUndoLatest()
  // This device's notifications, told to the server again as the app
  // opens: iOS can drop a subscription without a word.
  useQuery(pushDeviceQuery)
  // The user's shortcuts of one key, on until they turn them off.
  const singleKeys = useQuery(sessionQuery).data?.user.single_key_shortcuts !== false
  useSingleKeyShortcuts(singleKeys)
  const form = useRouterState({ select: (state) => formPage(state.location) })
  const [logging, setLogging] = useState(false)
  // What opened the meal to log, which the focus goes back to.
  const opener = useRef<HTMLElement | null>(null)
  // The page's own meal to log, while it has one.
  const logHere = useRef<LogHere | null>(null)
  const setLogHere = (l: LogHere | null) => {
    logHere.current = l
  }
  // log opens the meal to log on today, from what opened it.
  const log = (from: HTMLElement | null) => {
    if (logHere.current !== null) {
      logHere.current(from)
      return
    }
    opener.current = from
    setLogging(true)
  }
  // The command palette, open, with what it starts with, and what had the
  // focus before it, which a sheet it opens gives the focus back to.
  const [palette, setPalette] = useState<{ open: boolean; initial: string }>({
    open: false,
    initial: '',
  })
  const paletteFrom = useRef<HTMLElement | null>(null)
  const openPalette = (initial: string, from?: HTMLElement) => {
    const focused = globalThis.document.activeElement
    paletteFrom.current =
      from ??
      (focused instanceof HTMLElement && focused !== globalThis.document.body ? focused : null)
    setPalette({ open: true, initial })
  }
  // Today's weigh-in, open, and what opened it.
  const [weighing, setWeighing] = useState(false)
  const weighFrom = useRef<HTMLElement | null>(null)
  const weigh = (from: HTMLElement | null) => {
    weighFrom.current = from
    setWeighing(true)
  }
  const repeat = useRepeatYesterday(palette.open)
  const onDay = useRouterState({ select: (state) => state.location.pathname === '/' })
  // find takes the foods' search, there already or once there.
  const find = () => {
    const field = globalThis.document.querySelector<HTMLElement>('[data-find]')
    if (field) {
      field.focus()
    } else {
      void navigate({ to: '/foods', state: { find: true } })
    }
  }
  useShortcuts(
    {
      'Mod+k': () => {
        openPalette('')
      },
      n: () => {
        log(null)
      },
      w: () => {
        weigh(null)
      },
      '/': find,
      'Mod+z': () => {
        latest.undo()
      },
      // The sections, by the key after G: D for the day, F the foods, P the
      // progress, L the plan, S the settings.
      'g d': () => {
        void navigate({ to: '/' })
      },
      'g f': () => {
        void navigate({ to: '/foods' })
      },
      'g p': () => {
        void navigate({ to: '/progress' })
      },
      'g l': () => {
        void navigate({ to: '/plan' })
      },
      'g s': () => {
        void navigate({ to: '/settings' })
      },
      '?': () => {
        setShortcuts(true)
      },
    },
    onDay
      ? (key) => {
          openPalette(key)
        }
      : undefined,
  )
  const k = t.palette.words
  // The palette's actions, as it opens (now, whose meal of the day
  // yesterday's is of).
  const actions = (now: number): Command[] => {
    const repeatLabel = repeat.label(now)
    return [
      {
        id: 'log',
        icon: Plus,
        label: t.palette.logMeal,
        words: k.logMeal,
        ...(singleKeys && { keys: 'N', aria: 'N' }),
        run: () => {
          log(paletteFrom.current)
        },
      },
      {
        id: 'weigh',
        icon: Scale,
        label: t.palette.weighIn,
        words: k.weighIn,
        ...(singleKeys && { keys: 'W', aria: 'W' }),
        run: () => {
          weigh(paletteFrom.current)
        },
      },
      ...(repeatLabel === undefined
        ? []
        : [
            {
              id: 'repeat',
              icon: Repeat2,
              label: repeatLabel,
              words: k.repeat,
              run: () => {
                void repeat.run()
              },
            },
          ]),
      {
        id: 'undo',
        icon: Undo2,
        label: t.palette.undo,
        words: k.undo,
        keys: apple() ? '⌘Z' : 'Ctrl+Z',
        aria: apple() ? 'Meta+Z' : 'Control+Z',
        run: () => {
          latest.undo()
        },
      },
      {
        id: 'shortcuts',
        icon: Keyboard,
        label: t.palette.shortcuts,
        words: k.shortcuts,
        ...(singleKeys && { keys: '?', aria: '?' }),
        run: () => {
          setShortcuts(true)
        },
      },
    ]
  }

  return (
    <>
      <a
        href="#content"
        onClick={(e) => {
          // The page's start takes the focus, and the address stays the
          // page's.
          e.preventDefault()
          main.current?.focus()
        }}
        // Out of sight until it has the focus: its box is the sight's.
        className="sr-only z-50 rounded-md bg-card font-medium text-primary shadow-sheet focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:flex focus:min-h-11 focus:items-center focus:px-4 focus:ring-2 focus:ring-ring focus:outline-none"
      >
        {t.nav.skip}
      </a>
      <AppNav
        onLog={log}
        onSearch={(from) => {
          openPalette('', from)
        }}
        bar={!form}
      />
      <main
        id="content"
        ref={main}
        tabIndex={-1}
        className={cn(
          'outline-none tablet:pb-12 tablet:pl-[calc(4.5rem+env(safe-area-inset-left))] sidebar:pl-60',
          form ? 'pb-[max(1.5rem,env(safe-area-inset-bottom))]' : 'pb-28',
        )}
      >
        <div className="mx-auto w-full max-w-300 pt-6 pr-[max(1rem,env(safe-area-inset-right))] pl-[max(1rem,env(safe-area-inset-left))] lg:pt-8 tablet:pr-[max(1.5rem,env(safe-area-inset-right))] tablet:pl-6 sidebar:px-10">
          <StaleBar />
          <LogHereContext value={setLogHere}>
            <Outlet />
          </LogHereContext>
        </div>
      </main>
      <LogToday open={logging} onOpenChange={setLogging} opener={opener} />
      <WeighInSheet open={weighing} onOpenChange={setWeighing} opener={weighFrom} />
      <CommandPalette
        open={palette.open}
        onOpenChange={(open) => {
          setPalette((p) => ({ ...p, open }))
        }}
        initial={palette.initial}
        actions={actions}
        opener={paletteFrom}
      />
      <ShortcutsDialog open={shortcuts} onOpenChange={setShortcuts} singleKeys={singleKeys} />
    </>
  )
}
