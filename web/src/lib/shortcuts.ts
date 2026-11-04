import { useEffect, useRef } from 'react'

// Where a key is the user's text or a control's own: a field, and the
// widgets that move by arrows or letters. A shortcut is not taken there.
const ownKeys = [
  'input',
  'textarea',
  'select',
  '[contenteditable]:not([contenteditable="false"])',
  '[role="combobox"]',
  '[role="listbox"]',
  '[role="menu"]',
  '[role="radiogroup"]',
  '[role="slider"]',
  '[role="tablist"]',
  '[role="dialog"]',
].join(', ')

// How long the second key of a sequence (g, then d) is waited for.
const sequenceMs = 1500

// apple tells a Mac's or an iPad's keyboard, whose ⌘ is another's Ctrl.
export function apple(): boolean {
  return /Mac|iPhone|iPad/.test(globalThis.navigator.platform)
}

// mod is the name of the key a shortcut's "Mod+" is: ⌘ on a Mac's or an
// iPad's keyboard, Ctrl on another's.
export function mod(): string {
  return apple() ? '⌘' : 'Ctrl'
}

// The actions of every useShortcuts mounted, the latest of each: what
// tells a key typed from a key that is a shortcut.
const listening = new Set<{ current: Readonly<Record<string, () => void>> }>()

// taken tells whether a key, in either case, is a shortcut of any page
// mounted, alone or as the first of a sequence: N, G, R, W, ? or /, and
// a page's own, the plan's A and X.
function taken(key: string): boolean {
  const lower = key.toLowerCase()
  return [...listening].some((l) =>
    Object.keys(l.current).some((name) => name.split(' ')[0]?.toLowerCase() === lower),
  )
}

// typable tells a key that types a letter or a digit, of any alphabet,
// with no key held but Shift, outside an IME's composition (whose
// keystrokes are its own, "Process" while it lasts): what typing on a
// page starts (useShortcuts' typed).
function typable(event: KeyboardEvent): boolean {
  return (
    !event.altKey &&
    !event.ctrlKey &&
    !event.metaKey &&
    !event.isComposing &&
    /^[\p{L}\p{N}]$/u.test(event.key)
  )
}

// keyOf is a key pressed as an action names it: its KeyboardEvent.key
// alone (Shift aside, which makes ?), or with Ctrl or ⌘ as "Mod+z"; with
// Alt, or Ctrl and ⌘ both, none.
function keyOf(event: KeyboardEvent): string | undefined {
  if (event.altKey || (event.ctrlKey && event.metaKey)) {
    return undefined
  }
  return event.ctrlKey || event.metaKey ? `Mod+${event.key}` : event.key
}

// useShortcuts runs the action of a key pressed on the page, outside a
// field or a control that takes keys itself: by its KeyboardEvent.key,
// "Mod+z" for one with Ctrl or ⌘ (Ctrl+Shift+Z is "Mod+Z", another), or
// a sequence of two ("g d", the second within a second and a half of the
// first). The actions of the latest render run, and a sequence begun
// before it goes on. With typed, a letter or a digit typed outside a
// field (typable) that no page mounted takes as a shortcut (taken), nor
// as a sequence's second key, is typed's: the key, which goes no further.
export function useShortcuts(
  actions: Readonly<Record<string, () => void>>,
  typed?: (key: string) => void,
) {
  const latest = useRef(actions)
  const latestTyped = useRef(typed)
  useEffect(() => {
    latest.current = actions
    latestTyped.current = typed
  }, [actions, typed])
  useEffect(() => {
    listening.add(latest)
    return () => {
      listening.delete(latest)
    }
  }, [])
  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      const now = latest.current
      if (event.defaultPrevented || event.isComposing) {
        return
      }
      if (event.target instanceof Element && event.target.closest(ownKeys)) {
        return
      }
      const key = keyOf(event)
      if (key === undefined) {
        return
      }
      const before = sequenceBefore(event)
      // The second key of a sequence begun, a letter, is the sequence's
      // in every page's shortcuts, wherever it was begun: G, then A, is
      // no A. Another key (Tab, Enter, an arrow) is its own.
      const action =
        (before === undefined ? undefined : now[`${before} ${key}`]) ??
        (before !== undefined && key.length === 1 ? undefined : now[key])
      if (action) {
        event.preventDefault()
        action()
        return
      }
      // A letter no listener's sequence knows is left to the browser:
      // preventing it here would hide it from a listener later in turn
      // whose sequence it ends.
      if (before === undefined && Object.keys(now).some((name) => name.startsWith(`${key} `))) {
        event.preventDefault()
        begun = { key, at: Date.now(), event }
        return
      }
      const type = latestTyped.current
      if (type && before === undefined && typable(event) && !taken(event.key)) {
        event.preventDefault()
        type(event.key)
      }
    }
    globalThis.addEventListener('keydown', onKey)
    return () => {
      globalThis.removeEventListener('keydown', onKey)
    }
  }, [])
}

// begun is the first key of a sequence, shared by every page's shortcuts:
// its key, when, and its event, which is no second key of its own.
let begun: { key: string; at: number; event: KeyboardEvent } | undefined

// sequenceBefore is the first key of the sequence event is the second key
// of, if one was begun within sequenceMs before it; the sequence ends
// once every listener has seen it.
function sequenceBefore(event: KeyboardEvent): string | undefined {
  const b = begun
  if (b === undefined || b.event === event) {
    return undefined
  }
  if (Date.now() - b.at > sequenceMs) {
    begun = undefined
    return undefined
  }
  // A task, not a microtask: a browser runs those between two listeners.
  setTimeout(() => {
    if (begun === b) {
      begun = undefined
    }
  })
  return b.key
}
